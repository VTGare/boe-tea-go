package sender

import (
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	gt "github.com/VTGare/gumi/v2/gumitest"
	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/snowflake/v2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var errTestBoom = errors.New("boom")

const (
	testOwnerID snowflake.ID = 4000
	testRoleID  snowflake.ID = 5000
	testUserID  snowflake.ID = 6000
)

// discordAPI answers every request with the next queued status, then with
// 200 once the queue is empty.
type discordAPI struct {
	mu       sync.Mutex
	statuses []int
	requests []string
}

func (a *discordAPI) RoundTrip(req *http.Request) (*http.Response, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.requests = append(a.requests, req.Method+" "+req.URL.Path)

	if req.Body != nil {
		_, _ = io.Copy(io.Discard, req.Body)
	}

	status := http.StatusOK
	if len(a.statuses) > 0 {
		status, a.statuses = a.statuses[0], a.statuses[1:]
	}

	body := "{}"
	if status >= http.StatusBadRequest {
		body = `{"message": "error", "code": 0}`
	}

	return &http.Response{
		StatusCode: status,
		Status:     http.StatusText(status),
		Header:     http.Header{"Content-Type": {"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
		Request:    req,
	}, nil
}

func (a *discordAPI) Requests() []string {
	a.mu.Lock()
	defer a.mu.Unlock()

	return append([]string(nil), a.requests...)
}

// newTestClient caches a guild where @everyone can view, send, embed and attach,
// and the bot holds only @everyone.
func newTestClient(overwrites ...map[string]any) *bot.Client {
	c, _ := gt.NewClient()

	c.Caches.AddGuild(discord.Guild{ID: gt.GuildID, OwnerID: testOwnerID})
	c.Caches.AddRole(discord.Role{
		ID:      gt.GuildID,
		GuildID: gt.GuildID,
		Permissions: discord.PermissionViewChannel | discord.PermissionSendMessages |
			discord.PermissionEmbedLinks | discord.PermissionAttachFiles,
	})
	c.Caches.AddRole(discord.Role{ID: testRoleID, GuildID: gt.GuildID, Permissions: discord.PermissionManageMessages})
	c.Caches.AddMember(discord.Member{GuildID: gt.GuildID, User: discord.User{ID: gt.BotID}})
	c.Caches.AddChannel(gt.GuildChannel(map[string]any{
		"id":                    gt.ChannelID.String(),
		"guild_id":              gt.GuildID.String(),
		"type":                  discord.ChannelTypeGuildText,
		"permission_overwrites": overwrites,
	}))

	return c
}

func denyEveryone(perms discord.Permissions) map[string]any {
	return map[string]any{"id": gt.GuildID.String(), "type": 0, "allow": "0", "deny": strconv.FormatInt(int64(perms), 10)}
}

func newTestSender(c *bot.Client) (*DiscordSender, *discordAPI) {
	api := &discordAPI{}
	c.Rest.HTTPClient().Transport = api

	d := NewDiscordSender(c, nil)
	d.backoff = time.Millisecond

	return d, api
}

var _ = Describe("DiscordSender permissions", func() {
	It("allows posting with send and embed permissions", func() {
		d, _ := newTestSender(newTestClient())

		ok, err := d.HasChannelPerms(gt.GuildID, gt.ChannelID, SendPermissions)

		Expect(err).NotTo(HaveOccurred())
		Expect(ok).To(BeTrue())
	})

	It("denies posting when the channel overwrite revokes send", func() {
		d, _ := newTestSender(newTestClient(denyEveryone(discord.PermissionSendMessages)))

		ok, err := d.HasChannelPerms(gt.GuildID, gt.ChannelID, SendPermissions)

		Expect(err).NotTo(HaveOccurred())
		Expect(ok).To(BeFalse())
	})

	It("fails open on unknown channels", func() {
		d, _ := newTestSender(newTestClient())

		ok, err := d.HasChannelPerms(gt.GuildID, 1, SendPermissions)

		Expect(err).To(HaveOccurred())
		Expect(ok).To(BeTrue())
	})

	It("allows everything in DMs", func() {
		d, _ := newTestSender(newTestClient())

		ok, err := d.HasChannelPerms(0, 1, SendPermissions)

		Expect(err).NotTo(HaveOccurred())
		Expect(ok).To(BeTrue())
	})

	It("grants guild permissions held by a member role", func() {
		c := newTestClient()
		c.Caches.AddMember(discord.Member{GuildID: gt.GuildID, User: discord.User{ID: gt.BotID}, RoleIDs: []snowflake.ID{testRoleID}})
		d, _ := newTestSender(c)

		Expect(d.BotHasGuildPerms(gt.GuildID, discord.PermissionManageMessages)).To(BeTrue())
	})

	It("denies guild permissions the bot lacks", func() {
		d, _ := newTestSender(newTestClient())

		Expect(d.BotHasGuildPerms(gt.GuildID, discord.PermissionManageMessages)).To(BeFalse())
	})

	It("grants the guild owner every permission", func() {
		c := newTestClient()
		c.Caches.AddGuild(discord.Guild{ID: gt.GuildID, OwnerID: gt.BotID})
		d, _ := newTestSender(c)

		Expect(d.BotHasGuildPerms(gt.GuildID, discord.PermissionManageMessages)).To(BeTrue())
	})

	It("reports no guild permissions in DMs", func() {
		d, _ := newTestSender(newTestClient())

		Expect(d.BotHasGuildPerms(0, discord.PermissionManageMessages)).To(BeFalse())
	})
})

var _ = Describe("DiscordSender sends", func() {
	It("posts messages the bot is allowed to send", func() {
		d, api := newTestSender(newTestClient())

		_, err := d.SendEmbed(gt.ChannelID, discord.Embed{Title: "art"})

		Expect(err).NotTo(HaveOccurred())
		Expect(api.Requests()).To(Equal([]string{"POST /api/v10/channels/3000/messages"}))
	})

	It("skips sends the bot has no permission for", func() {
		d, api := newTestSender(newTestClient(denyEveryone(discord.PermissionEmbedLinks)))

		_, err := d.SendEmbed(gt.ChannelID, discord.Embed{Title: "art"})

		Expect(err).To(MatchError(ErrSkipped))
		Expect(api.Requests()).To(BeEmpty())
	})

	It("needs attach permission for files", func() {
		d, _ := newTestSender(newTestClient(denyEveryone(discord.PermissionAttachFiles)))

		_, err := d.SendComplex(gt.ChannelID, discord.MessageCreate{
			Files: []*discord.File{discord.NewFile("v.mp4", "", strings.NewReader("video"))},
		})

		Expect(err).To(MatchError(ErrSkipped))
	})

	It("retries Discord's server errors with the whole file", func() {
		c := newTestClient()
		rec := gt.Attach(c)
		d := NewDiscordSender(c, nil)
		d.backoff = time.Millisecond

		failing := &discordAPI{statuses: []int{http.StatusBadGateway}}
		c.Rest.HTTPClient().Transport = roundTrips(failing, rec)

		_, err := d.SendComplex(gt.ChannelID, discord.MessageCreate{
			Files: []*discord.File{discord.NewFile("v.mp4", "", strings.NewReader("video"))},
		})

		Expect(err).NotTo(HaveOccurred())
		Expect(failing.Requests()).To(HaveLen(1))
		Expect(rec.Requests()).To(HaveLen(1))
		Expect(rec.Requests()[0].Files).To(HaveKeyWithValue("v.mp4", "video"))
	})

	It("gives up after a few server errors", func() {
		d, api := newTestSender(newTestClient())
		api.statuses = []int{500, 500, 500, 500, 500}

		_, err := d.SendEmbed(gt.ChannelID, discord.Embed{})

		Expect(err).To(HaveOccurred())
		Expect(api.Requests()).To(HaveLen(4))
	})

	It("doesn't retry client errors", func() {
		d, api := newTestSender(newTestClient())
		api.statuses = []int{http.StatusForbidden}

		Expect(d.DeleteMessage(gt.ChannelID, 1)).NotTo(Succeed())
		Expect(api.Requests()).To(HaveLen(1))
	})
})

var _ = Describe("DiscordSender lookups", func() {
	It("finds a cached channel's guild without asking Discord", func() {
		d, api := newTestSender(newTestClient())

		Expect(d.ChannelGuildID(gt.ChannelID)).To(Equal(gt.GuildID))
		Expect(api.Requests()).To(BeEmpty())
	})

	It("reports members that left as non-members", func() {
		d, api := newTestSender(newTestClient())
		api.statuses = []int{http.StatusNotFound}

		Expect(d.IsMember(gt.GuildID, testUserID)).To(BeFalse())
	})

	It("surfaces other member lookup errors", func() {
		d, api := newTestSender(newTestClient())
		api.statuses = []int{http.StatusForbidden}

		_, err := d.IsMember(gt.GuildID, testUserID)

		Expect(err).To(HaveOccurred())
	})
})

// roundTrips sends each request to first until its queue is used up, then
// to second.
func roundTrips(first *discordAPI, second http.RoundTripper) http.RoundTripper {
	return roundTripFunc(func(req *http.Request) (*http.Response, error) {
		first.mu.Lock()
		pending := len(first.statuses) > 0
		first.mu.Unlock()

		if pending {
			return first.RoundTrip(req)
		}

		return second.RoundTrip(req)
	})
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

var _ = Describe("FakeSender", func() {
	var fake *FakeSender

	BeforeEach(func() {
		fake = NewFake()
	})

	It("defaults to allowing every permission check", func() {
		ok, err := fake.HasChannelPerms(1, 2, SendPermissions)

		Expect(err).NotTo(HaveOccurred())
		Expect(ok).To(BeTrue())

		ok, err = fake.BotHasGuildPerms(1, discord.PermissionManageMessages)

		Expect(err).NotTo(HaveOccurred())
		Expect(ok).To(BeTrue())
	})

	It("records sends with incrementing message IDs", func() {
		first, err := fake.SendComplex(2, discord.MessageCreate{Content: "one"})

		Expect(err).NotTo(HaveOccurred())
		Expect(first.ID).NotTo(BeZero())

		second, err := fake.SendEmbed(2, discord.Embed{Title: "two"})

		Expect(err).NotTo(HaveOccurred())
		Expect(second.ID).NotTo(Equal(first.ID))

		Expect(fake.Complex).To(HaveLen(1))
		Expect(fake.Complex[0].Message.Content).To(Equal("one"))
		Expect(fake.Embeds).To(HaveLen(1))
		Expect(fake.Embeds[0].Embed.Title).To(Equal("two"))
	})

	It("skips sends when configured to skip", func() {
		fake.Skip = true

		_, err := fake.SendComplex(2, discord.MessageCreate{})

		Expect(err).To(MatchError(ErrSkipped))

		_, err = fake.SendEmbed(2, discord.Embed{})

		Expect(err).To(MatchError(ErrSkipped))
		Expect(fake.Complex).To(BeEmpty())
		Expect(fake.Embeds).To(BeEmpty())
	})

	It("surfaces send errors to the caller", func() {
		fake.SendErr = errTestBoom

		_, err := fake.SendComplex(2, discord.MessageCreate{})

		Expect(err).To(MatchError(errTestBoom))
	})

	It("records deletes, reactions, and expired messages", func() {
		Expect(fake.DeleteMessage(2, 3)).To(Succeed())
		Expect(fake.AddReaction(2, 3, "💖")).To(Succeed())

		fake.Expire(&discord.Message{ID: 3})

		Expect(fake.Deleted).To(HaveLen(1))
		Expect(fake.Reactions).To(HaveLen(1))
		Expect(fake.Reactions[0].Emoji).To(Equal("💖"))
		Expect(fake.Expired).To(HaveLen(1))
	})

	It("answers channel guilds and membership", func() {
		fake.ChannelGuilds = map[snowflake.ID]snowflake.ID{2: 1}
		fake.Members = map[MemberKey]bool{{GuildID: 1, UserID: testUserID}: true}

		Expect(fake.ChannelGuildID(2)).To(Equal(snowflake.ID(1)))
		Expect(fake.IsMember(1, testUserID)).To(BeTrue())
		Expect(fake.IsMember(1, testOwnerID)).To(BeFalse())
	})

	It("reports unknown channels and surfaces read errors", func() {
		_, err := fake.ChannelGuildID(9)

		Expect(err).To(HaveOccurred())

		fake.MemberErr = errTestBoom

		_, err = fake.IsMember(1, testUserID)

		Expect(err).To(MatchError(errTestBoom))
	})
})
