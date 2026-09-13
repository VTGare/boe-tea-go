package sender

import (
	"errors"
	"net/http"

	"github.com/bwmarrin/discordgo"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/zap"
)

var errTestBoom = errors.New("boom")

func restError(status int) error {
	return &discordgo.RESTError{Response: &http.Response{StatusCode: status}}
}

const (
	testGuildID   = "g"
	testChannelID = "c"
	testBotID     = "bot"
)

func newTestSession(overwrites []*discordgo.PermissionOverwrite) *discordgo.Session {
	state := discordgo.NewState()
	state.User = &discordgo.User{ID: testBotID}

	Expect(state.GuildAdd(&discordgo.Guild{
		ID: testGuildID,
		Roles: []*discordgo.Role{
			{ID: testGuildID, Permissions: discordgo.PermissionViewChannel | discordgo.PermissionSendMessages | discordgo.PermissionEmbedLinks},
		},
		Members: []*discordgo.Member{
			{User: &discordgo.User{ID: testBotID}, Roles: []string{testGuildID}},
		},
		Channels: []*discordgo.Channel{
			{ID: testChannelID, GuildID: testGuildID, Type: discordgo.ChannelTypeGuildText, PermissionOverwrites: overwrites},
		},
	})).To(Succeed())

	return &discordgo.Session{State: state}
}

var _ = Describe("CheckChannelPerms", func() {
	It("allows posting with send and embed permissions", func() {
		ok, err := CheckChannelPerms(newTestSession(nil), testChannelID, SendPermissions)

		Expect(err).NotTo(HaveOccurred())
		Expect(ok).To(BeTrue())
	})

	It("denies posting when the channel overwrite revokes send", func() {
		session := newTestSession([]*discordgo.PermissionOverwrite{
			{ID: testGuildID, Type: discordgo.PermissionOverwriteTypeRole, Deny: discordgo.PermissionSendMessages},
		})

		ok, err := CheckChannelPerms(session, testChannelID, SendPermissions)

		Expect(err).NotTo(HaveOccurred())
		Expect(ok).To(BeFalse())
	})

	It("denies file posts without attach permission", func() {
		session := newTestSession([]*discordgo.PermissionOverwrite{
			{ID: testGuildID, Type: discordgo.PermissionOverwriteTypeRole, Deny: discordgo.PermissionAttachFiles},
		})

		ok, err := CheckChannelPerms(session, testChannelID, SendPermissions|discordgo.PermissionAttachFiles)

		Expect(err).NotTo(HaveOccurred())
		Expect(ok).To(BeFalse())
	})

	It("fails open on unknown channels", func() {
		ok, err := CheckChannelPerms(newTestSession(nil), "missing", SendPermissions)

		Expect(err).To(HaveOccurred())
		Expect(ok).To(BeTrue())
	})

	It("fails open without a session", func() {
		ok, err := CheckChannelPerms(nil, testChannelID, SendPermissions)

		Expect(err).To(HaveOccurred())
		Expect(ok).To(BeTrue())
	})
})

var _ = Describe("CheckGuildPerms", func() {
	It("grants permissions held by a member role", func() {
		ok, err := CheckGuildPerms(newTestSession(nil), testGuildID, testBotID, discordgo.PermissionSendMessages)

		Expect(err).NotTo(HaveOccurred())
		Expect(ok).To(BeTrue())
	})

	It("denies permissions the member lacks", func() {
		ok, err := CheckGuildPerms(newTestSession(nil), testGuildID, testBotID, discordgo.PermissionAdministrator)

		Expect(err).NotTo(HaveOccurred())
		Expect(ok).To(BeFalse())
	})

	It("grants the guild owner every permission", func() {
		state := discordgo.NewState()
		owner := &discordgo.User{ID: "owner"}
		state.User = owner

		Expect(state.GuildAdd(&discordgo.Guild{
			ID:      testGuildID,
			OwnerID: "owner",
			Members: []*discordgo.Member{{User: &discordgo.User{ID: "owner"}}},
		})).To(Succeed())

		ok, err := CheckGuildPerms(&discordgo.Session{State: state}, testGuildID, "owner", discordgo.PermissionAdministrator)

		Expect(err).NotTo(HaveOccurred())
		Expect(ok).To(BeTrue())
	})

	It("denies guild permissions without a session", func() {
		ok, err := CheckGuildPerms(nil, testGuildID, testBotID, discordgo.PermissionSendMessages)

		Expect(err).To(HaveOccurred())
		Expect(ok).To(BeFalse())
	})
})

var _ = Describe("ExpireMessage", func() {
	It("ignores nil messages and sessions without blocking", func() {
		ExpireMessage(zap.NewNop().Sugar(), nil, nil)
		ExpireMessage(zap.NewNop().Sugar(), newTestSession(nil), nil)
	})
})

var _ = Describe("Missing members", func() {
	It("matches Discord 404s only", func() {
		Expect(isNotFound(restError(404))).To(BeTrue())
		Expect(isNotFound(restError(403))).To(BeFalse())
		Expect(isNotFound(errTestBoom)).To(BeFalse())
	})
})

var _ = Describe("Channel and member lookups", func() {
	It("answers channel guilds and membership", func() {
		fake := NewFake()
		fake.ChannelGuilds = map[string]string{"c": "g"}
		fake.Members = map[MemberKey]bool{{GuildID: "g", UserID: "u"}: true}

		guildID, err := fake.ChannelGuildID("hint", "c")

		Expect(err).NotTo(HaveOccurred())
		Expect(guildID).To(Equal("g"))

		member, err := fake.IsMember("g", "u")

		Expect(err).NotTo(HaveOccurred())
		Expect(member).To(BeTrue())

		member, err = fake.IsMember("g", "stranger")

		Expect(err).NotTo(HaveOccurred())
		Expect(member).To(BeFalse())
	})

	It("reports unknown channels and surfaces read errors", func() {
		fake := NewFake()

		_, err := fake.ChannelGuildID("hint", "missing")

		Expect(err).To(HaveOccurred())

		fake.MemberErr = errTestBoom

		_, err = fake.IsMember("g", "u")

		Expect(err).To(MatchError(errTestBoom))
	})
})

var _ = Describe("FakeSender", func() {
	var fake *FakeSender

	BeforeEach(func() {
		fake = NewFake()
	})

	It("defaults to allowing every permission check", func() {
		ok, err := fake.HasChannelPerms("g", "c", SendPermissions)

		Expect(err).NotTo(HaveOccurred())
		Expect(ok).To(BeTrue())

		ok, err = fake.BotHasGuildPerms("g", discordgo.PermissionManageMessages)

		Expect(err).NotTo(HaveOccurred())
		Expect(ok).To(BeTrue())
	})

	It("records sends with incrementing message IDs", func() {
		first, err := fake.SendComplex("g", "c", &discordgo.MessageSend{Content: "one"})

		Expect(err).NotTo(HaveOccurred())
		Expect(first.ID).NotTo(BeEmpty())

		second, err := fake.SendEmbed("g", "c", &discordgo.MessageEmbed{Title: "two"})

		Expect(err).NotTo(HaveOccurred())
		Expect(second.ID).NotTo(Equal(first.ID))

		Expect(fake.Complex).To(HaveLen(1))
		Expect(fake.Complex[0].Message.Content).To(Equal("one"))
		Expect(fake.Embeds).To(HaveLen(1))
		Expect(fake.Embeds[0].Embed.Title).To(Equal("two"))
	})

	It("skips sends when configured to skip", func() {
		fake.Skip = true

		_, err := fake.SendComplex("g", "c", &discordgo.MessageSend{})

		Expect(err).To(MatchError(ErrSkipped))

		_, err = fake.SendEmbed("g", "c", &discordgo.MessageEmbed{})

		Expect(err).To(MatchError(ErrSkipped))
		Expect(fake.Complex).To(BeEmpty())
		Expect(fake.Embeds).To(BeEmpty())
	})

	It("surfaces send errors to the caller", func() {
		fake.SendErr = errTestBoom

		_, err := fake.SendComplex("g", "c", &discordgo.MessageSend{})

		Expect(err).To(MatchError(errTestBoom))
	})

	It("records deletes, reactions, and expired messages", func() {
		Expect(fake.DeleteMessage("g", "c", "m")).To(Succeed())
		Expect(fake.AddReaction("g", "c", "m", "💖")).To(Succeed())

		fake.Expire(&discordgo.Message{ID: "m"})

		Expect(fake.Deleted).To(HaveLen(1))
		Expect(fake.Reactions).To(HaveLen(1))
		Expect(fake.Reactions[0].Emoji).To(Equal("💖"))
		Expect(fake.Expired).To(HaveLen(1))
	})

	It("records embed edits and reaction removals", func() {
		edited, err := fake.EditEmbed("g", "c", "m", &discordgo.MessageEmbed{Title: "two"})

		Expect(err).NotTo(HaveOccurred())
		Expect(edited).NotTo(BeNil())
		Expect(fake.Edited).To(HaveLen(1))
		Expect(fake.Edited[0].Embed.Title).To(Equal("two"))

		Expect(fake.RemoveReaction("g", "c", "m", "▶", "u")).To(Succeed())
		Expect(fake.Unreacted).To(HaveLen(1))
		Expect(fake.Unreacted[0].UserID).To(Equal("u"))

		Expect(fake.RemoveAllReactions("g", "c", "m")).To(Succeed())
		Expect(fake.Cleared).To(HaveLen(1))
	})
})

var _ = Describe("DiscordSender", func() {
	nopLog := zap.NewNop().Sugar()

	It("fails every operation without a session", func() {
		d := NewDiscordSender(nil, nopLog, nil)

		_, err := d.SendComplex("1", "c", &discordgo.MessageSend{Content: "hi"})

		Expect(err).To(HaveOccurred())

		_, err = d.SendEmbed("1", "c", &discordgo.MessageEmbed{})

		Expect(err).To(HaveOccurred())
		Expect(d.DeleteMessage("1", "c", "m")).NotTo(Succeed())
		Expect(d.AddReaction("1", "c", "m", "💖")).NotTo(Succeed())

		_, err = d.EditEmbed("1", "c", "m", &discordgo.MessageEmbed{})

		Expect(err).To(HaveOccurred())
		Expect(d.RemoveReaction("1", "c", "m", "▶", "u")).NotTo(Succeed())
		Expect(d.RemoveAllReactions("1", "c", "m")).NotTo(Succeed())
	})

	It("fails open on channel permission checks without a session", func() {
		d := NewDiscordSender(nil, nopLog, nil)

		ok, err := d.HasChannelPerms("1", "c", SendPermissions)

		Expect(err).To(HaveOccurred())
		Expect(ok).To(BeTrue())
	})

	It("fails closed on guild permission checks without a session", func() {
		d := NewDiscordSender(nil, nopLog, nil)

		ok, err := d.BotHasGuildPerms("1", discordgo.PermissionManageMessages)

		Expect(err).To(HaveOccurred())
		Expect(ok).To(BeFalse())
	})

	It("skips sends the bot has no permission for", func() {
		denied := newTestSession([]*discordgo.PermissionOverwrite{
			{ID: testGuildID, Type: discordgo.PermissionOverwriteTypeRole, Deny: discordgo.PermissionSendMessages},
		})
		d := NewDiscordSender(nil, nopLog, denied)

		_, err := d.SendComplex("1", testChannelID, &discordgo.MessageSend{Content: "hi"})

		Expect(err).To(MatchError(ErrSkipped))
	})

	It("ignores nil expiry messages", func() {
		d := NewDiscordSender(nil, nopLog, nil)
		d.Expire(nil)
	})

	It("fails channel and member lookups without a session", func() {
		d := NewDiscordSender(nil, nopLog, nil)

		_, err := d.ChannelGuildID("1", "c")

		Expect(err).To(HaveOccurred())

		_, err = d.IsMember("1", "u")

		Expect(err).To(HaveOccurred())
	})
})
