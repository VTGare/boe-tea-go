package dgoutils

import (
	"testing"

	"github.com/VTGare/boe-tea-go/internal/sender"
	"github.com/bwmarrin/discordgo"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestDgoutils(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Dgoutils Suite")
}

var _ = Describe("Send permissions", func() {
	const (
		guildID   = "g"
		channelID = "c"
		botID     = "bot"
	)

	newSession := func(overwrites []*discordgo.PermissionOverwrite) *discordgo.Session {
		state := discordgo.NewState()
		state.User = &discordgo.User{ID: botID}

		Expect(state.GuildAdd(&discordgo.Guild{
			ID: guildID,
			Roles: []*discordgo.Role{
				{ID: guildID, Permissions: discordgo.PermissionViewChannel | discordgo.PermissionSendMessages | discordgo.PermissionEmbedLinks},
			},
			Members: []*discordgo.Member{
				{User: &discordgo.User{ID: botID}, Roles: []string{guildID}},
			},
			Channels: []*discordgo.Channel{
				{ID: channelID, GuildID: guildID, Type: discordgo.ChannelTypeGuildText, PermissionOverwrites: overwrites},
			},
		})).To(Succeed())

		return &discordgo.Session{State: state}
	}

	It("allows posting with send and embed permissions", func() {
		ok, err := sender.CheckChannelPerms(newSession(nil), channelID, sender.SendPermissions)

		Expect(err).NotTo(HaveOccurred())
		Expect(ok).To(BeTrue())
	})

	It("denies posting when the channel overwrite revokes send", func() {
		session := newSession([]*discordgo.PermissionOverwrite{
			{ID: guildID, Type: discordgo.PermissionOverwriteTypeRole, Deny: discordgo.PermissionSendMessages},
		})

		ok, err := sender.CheckChannelPerms(session, channelID, sender.SendPermissions)

		Expect(err).NotTo(HaveOccurred())
		Expect(ok).To(BeFalse())
	})

	It("denies file posts without attach permission", func() {
		session := newSession([]*discordgo.PermissionOverwrite{
			{ID: guildID, Type: discordgo.PermissionOverwriteTypeRole, Deny: discordgo.PermissionAttachFiles},
		})

		ok, err := sender.CheckChannelPerms(session, channelID, sender.SendPermissions|discordgo.PermissionAttachFiles)

		Expect(err).NotTo(HaveOccurred())
		Expect(ok).To(BeFalse())
	})

	It("fails open on unknown channels", func() {
		ok, err := sender.CheckChannelPerms(newSession(nil), "missing", sender.SendPermissions)

		Expect(err).To(HaveOccurred())
		Expect(ok).To(BeTrue())
	})

	It("fails open without a session", func() {
		ok, err := sender.CheckChannelPerms(nil, channelID, sender.SendPermissions)

		Expect(err).To(HaveOccurred())
		Expect(ok).To(BeTrue())
	})
})
