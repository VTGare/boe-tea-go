//go:build e2e

package e2e

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/VTGare/boe-tea-go/bot"
	"github.com/VTGare/boe-tea-go/commands"
	"github.com/VTGare/boe-tea-go/handlers"
	"github.com/VTGare/boe-tea-go/internal/sender"
	"github.com/VTGare/boe-tea-go/internal/spool"
	"github.com/VTGare/boe-tea-go/messages"
	"github.com/VTGare/boe-tea-go/post"
	"github.com/VTGare/boe-tea-go/repost"
	"github.com/VTGare/boe-tea-go/store"
	"github.com/VTGare/gumi/v2"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/snowflake/v2"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func newCommandBot(h *harness, stub *stubProvider, captured *error) *bot.Bot {
	b := newTestBot(h, stub, newDetector())
	b.AddRouter(gumi.New(gumi.Config{
		Prefixes: []string{"bt!"},
		ErrorHandler: func(_ *gumi.Context, err error) {
			*captured = err
		},
	}))
	commands.RegisterCommands(b)

	return b
}

func invokeCommand(b *bot.Bot, h *harness, authorID, channelID, messageID snowflake.ID, content string) {
	invokeCommandIn(b, h, h.cfg.guildID, authorID, channelID, messageID, content)
}

// guildID 0 makes it a DM.
func invokeCommandIn(b *bot.Bot, h *harness, guildID, authorID, channelID, messageID snowflake.ID, content string) {
	b.Router.HandleMessage(messageEvent(h, guildID, channelID, messageID, discord.User{ID: authorID, Username: "e2e-cmd"}, content))
}

// messageEvent fakes the gateway event for a message, as if author sent it.
func messageEvent(h *harness, guildID, channelID, messageID snowflake.ID, author discord.User, content string) *events.MessageCreate {
	msg := discord.Message{ID: messageID, ChannelID: channelID, Content: content, Author: author}

	var gid *snowflake.ID
	if guildID != 0 {
		gid = &guildID
		msg.GuildID = gid
		msg.Member = &discord.Member{GuildID: guildID, User: author}
	}

	return &events.MessageCreate{GenericMessage: &events.GenericMessage{
		GenericEvent: events.NewGenericEvent(h.client, 0, 0),
		MessageID:    messageID,
		Message:      msg,
		ChannelID:    channelID,
		GuildID:      gid,
	}}
}

// dmChannel opens a DM channel with the configured test user. The bot
// can't DM itself, so the DM specs need E2E_TEST_USER_ID.
func dmChannel(h *harness) snowflake.ID {
	if h.cfg.userID == 0 {
		Skip("set E2E_TEST_USER_ID to enable the DM specs")
	}

	ch, err := h.client.Rest.CreateDMChannel(h.cfg.userID)
	Expect(err).NotTo(HaveOccurred())

	return ch.ID()
}

func embedImagesAfter(h *harness, channelID, seedID snowflake.ID) []string {
	msgs, err := h.messagesAfter(channelID, seedID, 25)
	Expect(err).NotTo(HaveOccurred())

	images := make([]string, 0)
	for _, m := range msgs {
		for _, e := range m.Embeds {
			if e.Image != nil {
				images = append(images, e.Image.URL)
			}
		}
	}

	return images
}

// shareAcksAfter counts "X shared <link>" replies, which only slash
// commands should get.
func shareAcksAfter(h *harness, channelID, seedID snowflake.ID) int {
	msgs, err := h.messagesAfter(channelID, seedID, 25)
	Expect(err).NotTo(HaveOccurred())

	count := 0
	for _, m := range msgs {
		for _, e := range m.Embeds {
			if strings.Contains(e.Description, " shared <") {
				count++
			}
		}
	}

	return count
}

func newestMessageID(h *harness, channelID snowflake.ID) snowflake.ID {
	msgs, err := h.client.Rest.GetMessages(channelID, 0, 0, 0, 1)
	Expect(err).NotTo(HaveOccurred())

	if len(msgs) == 0 {
		return 0
	}

	return msgs[0].ID
}

var _ = Describe("Posting", func() {
	var (
		h        *harness
		detector repost.Detector
		rec      *recordedStats
	)

	BeforeEach(func() {
		h = e2eHarness
		detector = newDetector()
		rec = &recordedStats{}

		Expect(h.ensureGuild(context.Background(), baselineGuild)).To(Succeed())
	})

	It("replies to the original message", func() {
		ctx := context.Background()
		stub := newStubProvider()
		artURL := uniqueURL("single")

		stub.add(artURL, "single", &stubArtwork{previews: []string{"https://example.com/e2e.png"}})

		seed := seedChannel(h, h.cfg.channelID, "single")

		sent, err := h.newPoster(stub, detector, rec).Send(ctx, h.newRun(h.cfg.channelID, seed.ID, artURL))
		Expect(err).NotTo(HaveOccurred())
		Expect(sent).To(HaveLen(1))
		Expect(sent[0].ArtworkID).To(Equal("single"))
		cleanupSent(h, sent)

		msg, err := h.sentMessage(sent[0])
		Expect(err).NotTo(HaveOccurred())
		Expect(msg.Embeds).To(HaveLen(1))
		Expect(msg.Embeds[0].URL).To(Equal(artURL))
		Expect(msg.MessageReference).NotTo(BeNil())
		Expect(*msg.MessageReference.MessageID).To(Equal(seed.ID))
		Expect(rec.all()).To(HaveLen(1))
	})

	It("keeps page order", func() {
		ctx := context.Background()
		stub := newStubProvider()
		urlA := uniqueURL("order-a")
		urlB := uniqueURL("order-b")

		stub.add(urlA, "a", &stubArtwork{previews: []string{"https://example.com/a.png"}})
		stub.add(urlB, "b", &stubArtwork{previews: []string{"https://example.com/b1.png", "https://example.com/b2.png"}})

		seed := seedChannel(h, h.cfg.channelID, "order")

		sent, err := h.newPoster(stub, detector, nil).Send(ctx, h.newRun(h.cfg.channelID, seed.ID, urlA, urlB))
		Expect(err).NotTo(HaveOccurred())
		Expect(sent).To(HaveLen(3))
		Expect(sent[0].ArtworkID).To(Equal("a"))
		Expect(sent[1].ArtworkID).To(Equal("b"))
		Expect(sent[2].ArtworkID).To(Equal("b"))
		cleanupSent(h, sent)
	})

	It("sends only the requested pages", func() {
		ctx := context.Background()
		stub := newStubProvider()
		artURL := uniqueURL("include")

		stub.add(artURL, "inc", &stubArtwork{previews: []string{
			"https://example.com/1.png",
			"https://example.com/2.png",
			"https://example.com/3.png",
		}})

		seed := seedChannel(h, h.cfg.channelID, "include")

		run := h.newRun(h.cfg.channelID, seed.ID, artURL)
		run.IsCommand = true
		run.Skip = post.SkipFilter{Mode: post.SkipModeInclude, Indices: map[int]struct{}{2: {}}}

		sent, err := h.newPoster(stub, detector, nil).Send(ctx, run)
		Expect(err).NotTo(HaveOccurred())
		Expect(sent).To(HaveLen(1))
		cleanupSent(h, sent)

		msg, err := h.sentMessage(sent[0])
		Expect(err).NotTo(HaveOccurred())
		Expect(msg.Embeds).To(HaveLen(1))
		Expect(msg.Embeds[0].Image).NotTo(BeNil())
		Expect(msg.Embeds[0].Image.URL).To(Equal("https://example.com/2.png"))
	})

	It("warns about reposts and still sends", func() {
		ctx := context.Background()
		stub := newStubProvider()
		artURL := uniqueURL("repost")

		stub.add(artURL, "rep", &stubArtwork{previews: []string{"https://example.com/r.png"}})

		seed1 := seedChannel(h, h.cfg.channelID, "repost-1")

		first, err := h.newPoster(stub, detector, nil).Send(ctx, h.newRun(h.cfg.channelID, seed1.ID, artURL))
		Expect(err).NotTo(HaveOccurred())
		Expect(first).To(HaveLen(1))
		cleanupSent(h, first)

		seed2 := seedChannel(h, h.cfg.channelID, "repost-2")

		second, err := h.newPoster(stub, detector, nil).Send(ctx, h.newRun(h.cfg.channelID, seed2.ID, artURL))
		Expect(err).NotTo(HaveOccurred())
		Expect(second).To(HaveLen(1))
		Expect(second[0].ArtworkID).To(Equal("rep"))
		cleanupSent(h, second)

		after, err := h.messagesAfter(h.cfg.channelID, seed2.ID, 10)
		Expect(err).NotTo(HaveOccurred())

		notice := findEmbedByTitle(after, repostNoticeName)
		Expect(notice).NotTo(BeNil())
		DeferCleanup(func() { h.deleteAll(notice.ChannelID, notice.ID) })
	})

	It("deletes reposts in strict mode", func() {
		perm, err := h.sender.BotHasGuildPerms(h.cfg.guildID, discord.PermissionManageMessages)
		Expect(err).NotTo(HaveOccurred())

		if !perm {
			Skip("bot lacks manage-messages in the test guild")
		}

		ctx := context.Background()
		Expect(h.ensureGuild(ctx, func(g *store.Guild) {
			baselineGuild(g)
			g.Repost.Mode = store.RepostStrict
		})).To(Succeed())

		stub := newStubProvider()
		artURL := uniqueURL("strict")

		stub.add(artURL, "strict", &stubArtwork{previews: []string{"https://example.com/s.png"}})

		seed1 := seedChannel(h, h.cfg.channelID, "strict-1")

		first, err := h.newPoster(stub, detector, nil).Send(ctx, h.newRun(h.cfg.channelID, seed1.ID, artURL))
		Expect(err).NotTo(HaveOccurred())
		Expect(first).To(HaveLen(1))
		cleanupSent(h, first)

		seed2, err := h.seed(h.cfg.channelID, "strict-2")
		Expect(err).NotTo(HaveOccurred())

		second, err := h.newPoster(stub, detector, nil).Send(ctx, h.newRun(h.cfg.channelID, seed2.ID, artURL))
		Expect(err).NotTo(HaveOccurred())
		Expect(second).To(BeEmpty())

		_, err = h.client.Rest.GetMessage(h.cfg.channelID, seed2.ID)
		Expect(err).To(HaveOccurred())

		after, err := h.messagesAfter(h.cfg.channelID, seed1.ID, 10)
		Expect(err).NotTo(HaveOccurred())

		notice := findEmbedByTitle(after, repostNoticeName)
		Expect(notice).NotTo(BeNil())
		DeferCleanup(func() { h.deleteAll(notice.ChannelID, notice.ID) })
	})

	It("sends videos as attachments", func() {
		ok, err := h.sender.HasChannelPerms(h.cfg.guildID, h.cfg.channelID,
			sender.SendPermissions|discord.PermissionAttachFiles)
		Expect(err).NotTo(HaveOccurred())

		if !ok {
			Skip("bot lacks attach-files in the test channel")
		}

		dir := GinkgoT().TempDir()
		spool.Configure(spool.Config{Dir: dir})
		DeferCleanup(func() { spool.Configure(spool.DefaultConfig()) })

		file, err := spool.Download("bt-video-*.mp4", strings.NewReader("e2e-video-payload"))
		Expect(err).NotTo(HaveOccurred())

		stub := newStubProvider()
		artURL := uniqueURL("video")

		stub.add(artURL, "vid", &stubArtwork{files: []*discord.File{{Name: "e2e.mp4", Reader: file}}})

		seed := seedChannel(h, h.cfg.channelID, "video")

		sent, err := h.newPoster(stub, detector, nil).Send(context.Background(), h.newRun(h.cfg.channelID, seed.ID, artURL))
		Expect(err).NotTo(HaveOccurred())
		Expect(sent).To(HaveLen(1))
		cleanupSent(h, sent)

		msg, err := h.sentMessage(sent[0])
		Expect(err).NotTo(HaveOccurred())
		Expect(msg.Attachments).To(HaveLen(1))

		entries, err := os.ReadDir(dir)
		Expect(err).NotTo(HaveOccurred())
		Expect(entries).To(BeEmpty())
	})

	It("adds bookmark reactions", func() {
		ctx := context.Background()
		Expect(h.ensureGuild(ctx, func(g *store.Guild) {
			baselineGuild(g)
			g.Posting.Reactions = true
		})).To(Succeed())

		stub := newStubProvider()
		artURL := uniqueURL("reactions")

		stub.add(artURL, "react", &stubArtwork{previews: []string{"https://example.com/e.png"}})

		seed := seedChannel(h, h.cfg.channelID, "reactions")

		sent, err := h.newPoster(stub, detector, nil).Send(ctx, h.newRun(h.cfg.channelID, seed.ID, artURL))
		Expect(err).NotTo(HaveOccurred())
		Expect(sent).To(HaveLen(1))
		cleanupSent(h, sent)

		Eventually(func(g Gomega) {
			msg, err := h.sentMessage(sent[0])
			g.Expect(err).NotTo(HaveOccurred())

			names := make([]string, 0, len(msg.Reactions))
			for _, r := range msg.Reactions {
				names = append(names, r.Emoji.Name)
			}

			g.Expect(names).To(ContainElements("💖", "🤤"))
		}).WithTimeout(20 * time.Second).WithPolling(500 * time.Millisecond).Should(Succeed())
	})
})

var _ = Describe("Reposts", func() {
	var h *harness

	BeforeEach(func() {
		h = e2eHarness
	})

	It("sends everything when detection is off", func() {
		ctx := context.Background()
		Expect(h.ensureGuild(ctx, func(g *store.Guild) {
			baselineGuild(g)
			g.Repost.Mode = store.RepostOff
		})).To(Succeed())

		stub := newStubProvider()
		artURL := uniqueURL("repost-off")

		stub.add(artURL, "off", &stubArtwork{previews: []string{"https://example.com/o.png"}})

		detector := newDetector()

		for _, tag := range []string{"off-1", "off-2"} {
			seed := seedChannel(h, h.cfg.channelID, tag)

			sent, err := h.newPoster(stub, detector, nil).Send(ctx, h.newRun(h.cfg.channelID, seed.ID, artURL))
			Expect(err).NotTo(HaveOccurred())
			Expect(sent).To(HaveLen(1))
			cleanupSent(h, sent)

			if tag == "off-2" {
				after, err := h.messagesAfter(h.cfg.channelID, seed.ID, 10)
				Expect(err).NotTo(HaveOccurred())
				Expect(findEmbedByTitle(after, repostNoticeName)).To(BeNil())
			}
		}
	})

	It("warns about reposts without sending when the provider is disabled", func() {
		ctx := context.Background()
		Expect(h.ensureGuild(ctx, func(g *store.Guild) {
			baselineGuild(g)
			g.DisabledProviders = []string{"stub"}
		})).To(Succeed())

		stub := newStubProvider()
		artURL := uniqueURL("repost-disabled")

		stub.add(artURL, "dis", &stubArtwork{previews: []string{"https://example.com/d.png"}})

		detector := newDetector()

		seed1 := seedChannel(h, h.cfg.channelID, "disabled-1")

		first, err := h.newPoster(stub, detector, nil).Send(ctx, h.newRun(h.cfg.channelID, seed1.ID, artURL))
		Expect(err).NotTo(HaveOccurred())
		Expect(first).To(BeEmpty())
		Expect(embedImagesAfter(h, h.cfg.channelID, seed1.ID)).To(BeEmpty())

		seed2 := seedChannel(h, h.cfg.channelID, "disabled-2")

		second, err := h.newPoster(stub, detector, nil).Send(ctx, h.newRun(h.cfg.channelID, seed2.ID, artURL))
		Expect(err).NotTo(HaveOccurred())
		Expect(second).To(BeEmpty())
		Expect(embedImagesAfter(h, h.cfg.channelID, seed2.ID)).To(BeEmpty())

		after, err := h.messagesAfter(h.cfg.channelID, seed2.ID, 10)
		Expect(err).NotTo(HaveOccurred())

		notice := findEmbedByTitle(after, repostNoticeName)
		Expect(notice).NotTo(BeNil())
		DeferCleanup(func() { h.deleteAll(notice.ChannelID, notice.ID) })
	})

	It("keeps the message when only some links are reposts", func() {
		ctx := context.Background()
		Expect(h.ensureGuild(ctx, func(g *store.Guild) {
			baselineGuild(g)
			g.Repost.Mode = store.RepostStrict
		})).To(Succeed())

		stub := newStubProvider()
		urlA := uniqueURL("strict-partial-a")
		urlB := uniqueURL("strict-partial-b")

		stub.add(urlA, "pa", &stubArtwork{previews: []string{"https://example.com/pa.png"}})
		stub.add(urlB, "pb", &stubArtwork{previews: []string{"https://example.com/pb.png"}})

		detector := newDetector()

		seed1 := seedChannel(h, h.cfg.channelID, "strict-partial-1")

		first, err := h.newPoster(stub, detector, nil).Send(ctx, h.newRun(h.cfg.channelID, seed1.ID, urlA))
		Expect(err).NotTo(HaveOccurred())
		Expect(first).To(HaveLen(1))
		cleanupSent(h, first)

		seed2 := seedChannel(h, h.cfg.channelID, "strict-partial-2")

		second, err := h.newPoster(stub, detector, nil).Send(ctx, h.newRun(h.cfg.channelID, seed2.ID, urlA, urlB))
		Expect(err).NotTo(HaveOccurred())
		Expect(second).To(HaveLen(1))
		Expect(second[0].ArtworkID).To(Equal("pb"))
		cleanupSent(h, second)

		_, err = h.client.Rest.GetMessage(h.cfg.channelID, seed2.ID)
		Expect(err).NotTo(HaveOccurred())

		after, err := h.messagesAfter(h.cfg.channelID, seed2.ID, 10)
		Expect(err).NotTo(HaveOccurred())

		notice := findEmbedByTitle(after, repostNoticeName)
		Expect(notice).NotTo(BeNil())
		DeferCleanup(func() { h.deleteAll(notice.ChannelID, notice.ID) })
	})

	It("forgets reposts after expiry", func() {
		ctx := context.Background()
		Expect(h.ensureGuild(ctx, baselineGuild)).To(Succeed())

		// The store keeps TTLs at a minute or more; shorten it in memory.
		guilds := guildOverride{GuildStore: h.store, mutate: func(g *store.Guild) { g.Repost.TTL = time.Second }}

		stub := newStubProvider()
		artURL := uniqueURL("repost-expiry")

		stub.add(artURL, "exp", &stubArtwork{previews: []string{"https://example.com/e.png"}})

		detector := newDetector()

		seed1 := seedChannel(h, h.cfg.channelID, "expiry-1")

		first, err := h.newPosterWith(guilds, stub, detector, nil).Send(ctx, h.newRun(h.cfg.channelID, seed1.ID, artURL))
		Expect(err).NotTo(HaveOccurred())
		Expect(first).To(HaveLen(1))
		cleanupSent(h, first)

		time.Sleep(2 * time.Second)

		seed2 := seedChannel(h, h.cfg.channelID, "expiry-2")

		second, err := h.newPosterWith(guilds, stub, detector, nil).Send(ctx, h.newRun(h.cfg.channelID, seed2.ID, artURL))
		Expect(err).NotTo(HaveOccurred())
		Expect(second).To(HaveLen(1))
		cleanupSent(h, second)

		after, err := h.messagesAfter(h.cfg.channelID, seed2.ID, 10)
		Expect(err).NotTo(HaveOccurred())
		Expect(findEmbedByTitle(after, repostNoticeName)).To(BeNil())
	})

	It("skips ignored users outside commands", func() {
		ctx := context.Background()
		Expect(h.ensureGuild(ctx, baselineGuild)).To(Succeed())

		user, err := h.store.User(ctx, h.userID.String())
		Expect(err).NotTo(HaveOccurred())

		user.Ignore = true
		_, err = h.store.UpdateUser(ctx, user)
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(func() {
			user.Ignore = false
			_, _ = h.store.UpdateUser(context.Background(), user)
		})

		stub := newStubProvider()
		artURL := uniqueURL("ignored")

		stub.add(artURL, "ign", &stubArtwork{previews: []string{"https://example.com/i.png"}})

		detector := newDetector()
		seed := seedChannel(h, h.cfg.channelID, "ignored")

		silent, err := h.newPoster(stub, detector, nil).Send(ctx, h.newRun(h.cfg.channelID, seed.ID, artURL))
		Expect(err).NotTo(HaveOccurred())
		Expect(silent).To(BeEmpty())
		Expect(embedImagesAfter(h, h.cfg.channelID, seed.ID)).To(BeEmpty())

		cmdRun := h.newRun(h.cfg.channelID, seed.ID, artURL)
		cmdRun.IsCommand = true

		served, err := h.newPoster(stub, detector, nil).Send(ctx, cmdRun)
		Expect(err).NotTo(HaveOccurred())
		Expect(served).To(HaveLen(1))
		cleanupSent(h, served)
	})

	It("trims albums over the guild limit", func() {
		ctx := context.Background()
		Expect(h.ensureGuild(ctx, func(g *store.Guild) {
			baselineGuild(g)
			g.Posting.Limit = 2
		})).To(Succeed())

		stub := newStubProvider()
		artURL := uniqueURL("limit")

		stub.add(artURL, "lim", &stubArtwork{previews: []string{
			"https://example.com/1.png",
			"https://example.com/2.png",
			"https://example.com/3.png",
			"https://example.com/4.png",
		}})

		detector := newDetector()
		seed := seedChannel(h, h.cfg.channelID, "limit")

		sent, err := h.newPoster(stub, detector, nil).Send(ctx, h.newRun(h.cfg.channelID, seed.ID, artURL))
		Expect(err).NotTo(HaveOccurred())
		Expect(sent).To(HaveLen(2))
		cleanupSent(h, sent)

		msg, err := h.sentMessage(sent[0])
		Expect(err).NotTo(HaveOccurred())
		Expect(msg.Content).To(Equal(messages.LimitExceeded(2, 1, 4)))
	})
})

var _ = Describe("Crossposts", func() {
	It("credits the original author", func() {
		h := e2eHarness

		if !h.cfg.hasXpost() {
			Skip("set E2E_XPOST_CHANNEL_ID to enable the crosspost spec")
		}

		ctx := context.Background()
		Expect(h.ensureGuild(ctx, baselineGuild)).To(Succeed())

		group := fmt.Sprintf("e2e-%d", time.Now().UnixNano())

		_, err := h.store.CreateCrosspostGroup(ctx, h.userID.String(), &store.Group{Name: group, Parent: h.cfg.channelID.String()})
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(func() { _, _ = h.store.DeleteCrosspostGroup(ctx, h.userID.String(), group) })

		_, err = h.store.AddCrosspostChannel(ctx, h.userID.String(), group, h.cfg.xpostChannelID.String())
		Expect(err).NotTo(HaveOccurred())

		detector := newDetector()
		stub := newStubProvider()
		artURL := uniqueURL("xpost")

		stub.add(artURL, "x", &stubArtwork{previews: []string{"https://example.com/x.png"}})

		seed := seedChannel(h, h.cfg.channelID, "xpost")

		sent, err := h.newPoster(stub, detector, nil).Send(ctx, h.newRun(h.cfg.channelID, seed.ID, artURL))
		Expect(err).NotTo(HaveOccurred())
		Expect(sent).NotTo(BeEmpty())
		DeferCleanup(func() {
			byChannel := map[snowflake.ID][]snowflake.ID{}
			for _, info := range sent {
				channelID := parseID(info.ChannelID)
				byChannel[channelID] = append(byChannel[channelID], parseID(info.MessageID))
			}

			for channelID, ids := range byChannel {
				h.deleteAll(channelID, ids...)
			}
		})

		var mainMsg, xpostMsg *discord.Message

		for _, info := range sent {
			msg, err := h.sentMessage(info)
			Expect(err).NotTo(HaveOccurred())

			switch msg.ChannelID {
			case h.cfg.channelID:
				mainMsg = msg
			case h.cfg.xpostChannelID:
				xpostMsg = msg
			}
		}

		Expect(mainMsg).NotTo(BeNil())
		Expect(xpostMsg).NotTo(BeNil())
		Expect(mainMsg.MessageReference).NotTo(BeNil())
		Expect(xpostMsg.Embeds).NotTo(BeEmpty())
		Expect(xpostMsg.Embeds[0].Author).NotTo(BeNil())
		Expect(xpostMsg.Embeds[0].Author.Name).To(Equal(messages.CrosspostBy("e2e")))
	})
})

var _ = Describe("Message handler", func() {
	It("posts links from plain messages", func() {
		h := e2eHarness
		ctx := context.Background()
		Expect(h.ensureGuild(ctx, baselineGuild)).To(Succeed())

		detector := newDetector()
		stub := newStubProvider()
		artURL := uniqueURL("handler")

		stub.add(artURL, "hdl", &stubArtwork{previews: []string{"https://example.com/h.png"}})

		seed := seedChannel(h, h.cfg.channelID, "handler")
		b := newTestBot(h, stub, detector)

		handlers.OnMessage(b)(messageEvent(h, h.cfg.guildID, h.cfg.channelID, seed.ID,
			discord.User{ID: h.userID, Username: "e2e"}, artURL))

		after, err := h.messagesAfter(h.cfg.channelID, seed.ID, 10)
		Expect(err).NotTo(HaveOccurred())
		Expect(messageIDsAfter(after)).NotTo(BeEmpty())
		cleanupAfter(h, h.cfg.channelID, seed.ID)

		found := false

		for _, m := range after {
			for _, e := range m.Embeds {
				if e.URL == artURL {
					found = true
				}
			}
		}

		Expect(found).To(BeTrue())

		cached, ok := b.EmbedCache.Get(h.cfg.channelID.String(), seed.ID.String())
		Expect(ok).To(BeTrue())
		Expect(cached.IsParent).To(BeTrue())
	})
})

var _ = Describe("DMs", func() {
	var h *harness

	BeforeEach(func() {
		h = e2eHarness
	})

	It("posts links from plain DM messages", func() {
		dm := dmChannel(h)
		stub := newStubProvider()
		artURL := uniqueURL("dm-handler")

		stub.add(artURL, "dmhdl", &stubArtwork{previews: []string{"https://example.com/dm.png"}})

		seed := seedChannel(h, dm, "dm-handler")
		b := newTestBot(h, stub, newDetector())

		handlers.OnMessage(b)(messageEvent(h, 0, dm, seed.ID,
			discord.User{ID: newID(), Username: "e2e-dm"}, artURL))

		after, err := h.messagesAfter(dm, seed.ID, 10)
		Expect(err).NotTo(HaveOccurred())
		cleanupAfter(h, dm, seed.ID)

		found := false

		for _, m := range after {
			for _, e := range m.Embeds {
				if e.URL == artURL {
					found = true
				}
			}
		}

		Expect(found).To(BeTrue())
	})

	It("answers share commands in DMs", func() {
		dm := dmChannel(h)
		stub := newStubProvider()
		artURL := uniqueURL("dm-share")

		stub.add(artURL, "dmsh", &stubArtwork{previews: []string{
			"https://example.com/dm1.png",
			"https://example.com/dm2.png",
		}})

		var captured error
		b := newCommandBot(h, stub, &captured)
		seed := seedChannel(h, dm, "dm-share")

		invokeCommandIn(b, h, 0, newID(), dm, seed.ID, "bt!share "+artURL)

		Expect(captured).NotTo(HaveOccurred())
		Expect(embedImagesAfter(h, dm, seed.ID)).To(And(
			HaveLen(2),
			ContainElements("https://example.com/dm1.png", "https://example.com/dm2.png"),
		))
		cleanupAfter(h, dm, seed.ID)
	})

	It("answers exclude commands in DMs", func() {
		dm := dmChannel(h)
		stub := newStubProvider()
		artURL := uniqueURL("dm-exclude")

		stub.add(artURL, "dmex", &stubArtwork{previews: []string{
			"https://example.com/dm1.png",
			"https://example.com/dm2.png",
			"https://example.com/dm3.png",
		}})

		var captured error
		b := newCommandBot(h, stub, &captured)
		seed := seedChannel(h, dm, "dm-exclude")

		invokeCommandIn(b, h, 0, newID(), dm, seed.ID, "bt!ex "+artURL+" 2")

		Expect(captured).NotTo(HaveOccurred())
		Expect(embedImagesAfter(h, dm, seed.ID)).To(And(
			HaveLen(2),
			ContainElements("https://example.com/dm1.png", "https://example.com/dm3.png"),
		))
		cleanupAfter(h, dm, seed.ID)
	})
})

var _ = Describe("Share command", func() {
	var h *harness

	BeforeEach(func() {
		h = e2eHarness
		Expect(h.ensureGuild(context.Background(), baselineGuild)).To(Succeed())
	})

	It("shares chosen pages", func() {
		stub := newStubProvider()
		artURL := uniqueURL("share-idx")

		stub.add(artURL, "idx", &stubArtwork{previews: []string{
			"https://example.com/1.png",
			"https://example.com/2.png",
			"https://example.com/3.png",
		}})

		var captured error
		b := newCommandBot(h, stub, &captured)
		seed := seedChannel(h, h.cfg.channelID, "share-idx")

		invokeCommand(b, h, newID(),
			h.cfg.channelID, seed.ID, "bt!share "+artURL+" 1 3")

		Expect(captured).NotTo(HaveOccurred())
		Expect(embedImagesAfter(h, h.cfg.channelID, seed.ID)).To(And(
			HaveLen(2),
			ContainElements("https://example.com/1.png", "https://example.com/3.png"),
		))
		Expect(shareAcksAfter(h, h.cfg.channelID, seed.ID)).To(BeZero())
		cleanupAfter(h, h.cfg.channelID, seed.ID)

		cached, ok := b.EmbedCache.Get(h.cfg.channelID.String(), seed.ID.String())
		Expect(ok).To(BeTrue())
		Expect(cached.IsParent).To(BeTrue())
	})

	It("shares a page range", func() {
		stub := newStubProvider()
		artURL := uniqueURL("share-range")

		stub.add(artURL, "rng", &stubArtwork{previews: []string{
			"https://example.com/1.png",
			"https://example.com/2.png",
			"https://example.com/3.png",
		}})

		var captured error
		b := newCommandBot(h, stub, &captured)
		seed := seedChannel(h, h.cfg.channelID, "share-range")

		invokeCommand(b, h, newID(),
			h.cfg.channelID, seed.ID, "bt!share "+artURL+" 1-2")

		Expect(captured).NotTo(HaveOccurred())
		Expect(embedImagesAfter(h, h.cfg.channelID, seed.ID)).To(And(
			HaveLen(2),
			ContainElements("https://example.com/1.png", "https://example.com/2.png"),
		))
		cleanupAfter(h, h.cfg.channelID, seed.ID)
	})

	It("leaves out excluded pages", func() {
		stub := newStubProvider()
		artURL := uniqueURL("share-ex")

		stub.add(artURL, "ex", &stubArtwork{previews: []string{
			"https://example.com/1.png",
			"https://example.com/2.png",
			"https://example.com/3.png",
		}})

		var captured error
		b := newCommandBot(h, stub, &captured)
		seed := seedChannel(h, h.cfg.channelID, "share-ex")

		invokeCommand(b, h, newID(),
			h.cfg.channelID, seed.ID, "bt!ex "+artURL+" 2")

		Expect(captured).NotTo(HaveOccurred())
		Expect(embedImagesAfter(h, h.cfg.channelID, seed.ID)).To(And(
			HaveLen(2),
			ContainElements("https://example.com/1.png", "https://example.com/3.png"),
		))
		cleanupAfter(h, h.cfg.channelID, seed.ID)
	})

	It("rejects bad page numbers", func() {
		stub := newStubProvider()
		artURL := uniqueURL("share-bad")

		stub.add(artURL, "bad", &stubArtwork{previews: []string{"https://example.com/1.png"}})

		var captured error
		b := newCommandBot(h, stub, &captured)
		seed := seedChannel(h, h.cfg.channelID, "share-bad")

		invokeCommand(b, h, newID(),
			h.cfg.channelID, seed.ID, "bt!share "+artURL+" abc")

		Expect(captured).To(HaveOccurred())
		Expect(captured.Error()).To(ContainSubstring("abc"))
		Expect(embedImagesAfter(h, h.cfg.channelID, seed.ID)).To(BeEmpty())
	})

	It("needs a link to share", func() {
		var captured error
		b := newCommandBot(h, newStubProvider(), &captured)
		seed := seedChannel(h, h.cfg.channelID, "share-noargs")

		invokeCommand(b, h, newID(),
			h.cfg.channelID, seed.ID, "bt!share")

		Expect(captured).To(MatchError(gumi.ErrMissingOption))
		Expect(embedImagesAfter(h, h.cfg.channelID, seed.ID)).To(BeEmpty())
	})

	It("skips excluded crosspost channels", func() {
		if !h.cfg.hasXpost() {
			Skip("set E2E_XPOST_CHANNEL_ID to enable the crosspostexclude spec")
		}

		ctx := context.Background()
		stub := newStubProvider()
		artURL := uniqueURL("share-cpex")

		stub.add(artURL, "cpex", &stubArtwork{previews: []string{"https://example.com/x.png"}})

		group := fmt.Sprintf("e2e-cmd-%d", time.Now().UnixNano())

		_, err := h.store.CreateCrosspostGroup(ctx, h.userID.String(), &store.Group{Name: group, Parent: h.cfg.channelID.String()})
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(func() { _, _ = h.store.DeleteCrosspostGroup(ctx, h.userID.String(), group) })

		_, err = h.store.AddCrosspostChannel(ctx, h.userID.String(), group, h.cfg.xpostChannelID.String())
		Expect(err).NotTo(HaveOccurred())

		var captured error
		b := newCommandBot(h, stub, &captured)
		seed := seedChannel(h, h.cfg.channelID, "share-cpex")
		before := newestMessageID(h, h.cfg.xpostChannelID)

		invokeCommand(b, h, h.userID, h.cfg.channelID, seed.ID,
			"bt!crosspostexclude "+artURL+" "+discord.ChannelMention(h.cfg.xpostChannelID))

		Expect(captured).NotTo(HaveOccurred())
		Expect(embedImagesAfter(h, h.cfg.channelID, seed.ID)).To(ContainElement("https://example.com/x.png"))
		Expect(newestMessageID(h, h.cfg.xpostChannelID)).To(Equal(before))
		cleanupAfter(h, h.cfg.channelID, seed.ID)
	})
})
