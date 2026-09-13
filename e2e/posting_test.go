//go:build e2e

package e2e

import (
	"context"
	"errors"
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
	"github.com/VTGare/gumi"
	"github.com/bwmarrin/discordgo"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func newCommandBot(h *harness, stub *stubProvider, captured *error) *bot.Bot {
	b := newTestBot(h, stub, newDetector())
	b.AddRouter(&gumi.Router{
		Commands:       map[string]*gumi.Command{},
		PrefixResolver: func(*discordgo.Session, *discordgo.MessageCreate) []string { return []string{"bt!"} },
		OnErrorCallback: func(_ *gumi.Ctx, err error) {
			*captured = err
		},
	})
	commands.RegisterCommands(b)

	return b
}

func invokeCommand(b *bot.Bot, h *harness, authorID, channelID, messageID, content string) {
	event := &discordgo.MessageCreate{Message: &discordgo.Message{
		ID:        messageID,
		ChannelID: channelID,
		GuildID:   h.cfg.guildID,
		Content:   content,
		Author:    &discordgo.User{ID: authorID, Username: "e2e-cmd", Bot: false},
	}}

	b.Router.Handler()(h.session, event)
}

func embedImagesAfter(h *harness, channelID, seedID string) []string {
	msgs, err := h.session.ChannelMessages(channelID, 25, "", seedID, "")
	Expect(err).NotTo(HaveOccurred())

	images := make([]string, 0)
	for _, m := range msgs {
		if m == nil {
			continue
		}

		for _, e := range m.Embeds {
			if e != nil && e.Image != nil {
				images = append(images, e.Image.URL)
			}
		}
	}

	return images
}

func newestMessageID(h *harness, channelID string) string {
	msgs, err := h.session.ChannelMessages(channelID, 1, "", "", "")
	Expect(err).NotTo(HaveOccurred())

	if len(msgs) == 0 {
		return ""
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

		msg, err := h.session.ChannelMessage(h.cfg.channelID, sent[0].MessageID)
		Expect(err).NotTo(HaveOccurred())
		Expect(msg.Embeds).To(HaveLen(1))
		Expect(msg.Embeds[0].URL).To(Equal(artURL))
		Expect(msg.MessageReference).NotTo(BeNil())
		Expect(msg.MessageReference.MessageID).To(Equal(seed.ID))
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

		msg, err := h.session.ChannelMessage(h.cfg.channelID, sent[0].MessageID)
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

		after, err := h.session.ChannelMessages(h.cfg.channelID, 10, "", seed2.ID, "")
		Expect(err).NotTo(HaveOccurred())

		notice := findEmbedByTitle(after, repostNoticeName)
		Expect(notice).NotTo(BeNil())
		DeferCleanup(func() { h.deleteAll(notice.ChannelID, notice.ID) })
	})

	It("deletes reposts in strict mode", func() {
		perm, err := h.sender.BotHasGuildPerms(
			h.cfg.guildID,
			discordgo.PermissionAdministrator|discordgo.PermissionManageMessages,
		)
		Expect(err).NotTo(HaveOccurred())

		if !perm {
			Skip("bot lacks manage-messages in the test guild")
		}

		ctx := context.Background()
		Expect(h.ensureGuild(ctx, func(g *store.Guild) {
			baselineGuild(g)
			g.Repost = store.GuildRepostStrict
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

		_, err = h.session.ChannelMessage(h.cfg.channelID, seed2.ID)
		Expect(err).To(HaveOccurred())

		after, err := h.session.ChannelMessages(h.cfg.channelID, 10, "", seed1.ID, "")
		Expect(err).NotTo(HaveOccurred())

		notice := findEmbedByTitle(after, repostNoticeName)
		Expect(notice).NotTo(BeNil())
		DeferCleanup(func() { h.deleteAll(notice.ChannelID, notice.ID) })
	})

	It("sends videos as attachments", func() {
		ok, err := h.sender.HasChannelPerms(h.cfg.guildID, h.cfg.channelID,
			sender.SendPermissions|discordgo.PermissionAttachFiles)
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

		stub.add(artURL, "vid", &stubArtwork{files: []*discordgo.File{{Name: "e2e.mp4", Reader: file}}})

		seed := seedChannel(h, h.cfg.channelID, "video")

		sent, err := h.newPoster(stub, detector, nil).Send(context.Background(), h.newRun(h.cfg.channelID, seed.ID, artURL))
		Expect(err).NotTo(HaveOccurred())
		Expect(sent).To(HaveLen(1))
		cleanupSent(h, sent)

		msg, err := h.session.ChannelMessage(h.cfg.channelID, sent[0].MessageID)
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
			g.Reactions = true
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
			msg, err := h.session.ChannelMessage(h.cfg.channelID, sent[0].MessageID)
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
			g.Repost = store.GuildRepostDisabled
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
				after, err := h.session.ChannelMessages(h.cfg.channelID, 10, "", seed.ID, "")
				Expect(err).NotTo(HaveOccurred())
				Expect(findEmbedByTitle(after, repostNoticeName)).To(BeNil())
			}
		}
	})

	It("keeps the message when only some links are reposts", func() {
		ctx := context.Background()
		Expect(h.ensureGuild(ctx, func(g *store.Guild) {
			baselineGuild(g)
			g.Repost = store.GuildRepostStrict
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

		_, err = h.session.ChannelMessage(h.cfg.channelID, seed2.ID)
		Expect(err).NotTo(HaveOccurred())

		after, err := h.session.ChannelMessages(h.cfg.channelID, 10, "", seed2.ID, "")
		Expect(err).NotTo(HaveOccurred())

		notice := findEmbedByTitle(after, repostNoticeName)
		Expect(notice).NotTo(BeNil())
		DeferCleanup(func() { h.deleteAll(notice.ChannelID, notice.ID) })
	})

	It("forgets reposts after expiry", func() {
		ctx := context.Background()
		Expect(h.ensureGuild(ctx, func(g *store.Guild) {
			baselineGuild(g)
			g.RepostExpiration = time.Second
		})).To(Succeed())

		stub := newStubProvider()
		artURL := uniqueURL("repost-expiry")

		stub.add(artURL, "exp", &stubArtwork{previews: []string{"https://example.com/e.png"}})

		detector := newDetector()

		seed1 := seedChannel(h, h.cfg.channelID, "expiry-1")

		first, err := h.newPoster(stub, detector, nil).Send(ctx, h.newRun(h.cfg.channelID, seed1.ID, artURL))
		Expect(err).NotTo(HaveOccurred())
		Expect(first).To(HaveLen(1))
		cleanupSent(h, first)

		time.Sleep(2 * time.Second)

		seed2 := seedChannel(h, h.cfg.channelID, "expiry-2")

		second, err := h.newPoster(stub, detector, nil).Send(ctx, h.newRun(h.cfg.channelID, seed2.ID, artURL))
		Expect(err).NotTo(HaveOccurred())
		Expect(second).To(HaveLen(1))
		cleanupSent(h, second)

		after, err := h.session.ChannelMessages(h.cfg.channelID, 10, "", seed2.ID, "")
		Expect(err).NotTo(HaveOccurred())
		Expect(findEmbedByTitle(after, repostNoticeName)).To(BeNil())
	})

	It("skips ignored users outside commands", func() {
		ctx := context.Background()
		Expect(h.ensureGuild(ctx, baselineGuild)).To(Succeed())

		user, err := h.store.User(ctx, h.userID)
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
			g.Limit = 2
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

		msg, err := h.session.ChannelMessage(h.cfg.channelID, sent[0].MessageID)
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

		_, err := h.store.CreateCrosspostGroup(ctx, h.userID, &store.Group{Name: group, Parent: h.cfg.channelID})
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(func() { _, _ = h.store.DeleteCrosspostGroup(ctx, h.userID, group) })

		_, err = h.store.AddCrosspostChannel(ctx, h.userID, group, h.cfg.xpostChannelID)
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
			byChannel := map[string][]string{}
			for _, info := range sent {
				byChannel[info.ChannelID] = append(byChannel[info.ChannelID], info.MessageID)
			}

			for channelID, ids := range byChannel {
				h.deleteAll(channelID, ids...)
			}
		})

		var mainMsg, xpostMsg *discordgo.Message

		for _, info := range sent {
			msg, err := h.session.ChannelMessage(info.ChannelID, info.MessageID)
			Expect(err).NotTo(HaveOccurred())

			switch info.ChannelID {
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

		event := &discordgo.MessageCreate{Message: &discordgo.Message{
			ID:        seed.ID,
			ChannelID: h.cfg.channelID,
			GuildID:   h.cfg.guildID,
			Content:   artURL,
			Author:    &discordgo.User{ID: h.userID, Username: "e2e", Bot: false},
		}}

		gctx := &gumi.Ctx{Session: h.session, Event: event, Router: gumi.Create(&gumi.Router{})}
		Expect(handlers.OnMessage(b)(gctx)).To(Succeed())

		after, err := h.session.ChannelMessages(h.cfg.channelID, 10, "", seed.ID, "")
		Expect(err).NotTo(HaveOccurred())
		Expect(messageIDsAfter(after)).NotTo(BeEmpty())
		cleanupAfter(h, h.cfg.channelID, seed.ID)

		found := false

		for _, m := range after {
			for _, e := range m.Embeds {
				if e != nil && e.URL == artURL {
					found = true
				}
			}
		}

		Expect(found).To(BeTrue())

		cached, ok := b.EmbedCache.Get(h.cfg.channelID, seed.ID)
		Expect(ok).To(BeTrue())
		Expect(cached.IsParent).To(BeTrue())
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

		invokeCommand(b, h, fmt.Sprintf("e2e-cmd-%d", time.Now().UnixNano()),
			h.cfg.channelID, seed.ID, "bt!share "+artURL+" 1 3")

		Expect(captured).NotTo(HaveOccurred())
		Expect(embedImagesAfter(h, h.cfg.channelID, seed.ID)).To(And(
			HaveLen(2),
			ContainElements("https://example.com/1.png", "https://example.com/3.png"),
		))
		cleanupAfter(h, h.cfg.channelID, seed.ID)

		cached, ok := b.EmbedCache.Get(h.cfg.channelID, seed.ID)
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

		invokeCommand(b, h, fmt.Sprintf("e2e-cmd-%d", time.Now().UnixNano()),
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

		invokeCommand(b, h, fmt.Sprintf("e2e-cmd-%d", time.Now().UnixNano()),
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

		invokeCommand(b, h, fmt.Sprintf("e2e-cmd-%d", time.Now().UnixNano()),
			h.cfg.channelID, seed.ID, "bt!share "+artURL+" abc")

		Expect(captured).To(HaveOccurred())
		Expect(captured.Error()).To(ContainSubstring("abc"))
		Expect(embedImagesAfter(h, h.cfg.channelID, seed.ID)).To(BeEmpty())
	})

	It("needs a link to share", func() {
		var captured error
		b := newCommandBot(h, newStubProvider(), &captured)
		seed := seedChannel(h, h.cfg.channelID, "share-noargs")

		invokeCommand(b, h, fmt.Sprintf("e2e-cmd-%d", time.Now().UnixNano()),
			h.cfg.channelID, seed.ID, "bt!share")

		var cmdErr *messages.IncorrectCmd
		Expect(errors.As(captured, &cmdErr)).To(BeTrue())
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

		_, err := h.store.CreateCrosspostGroup(ctx, h.userID, &store.Group{Name: group, Parent: h.cfg.channelID})
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(func() { _, _ = h.store.DeleteCrosspostGroup(ctx, h.userID, group) })

		_, err = h.store.AddCrosspostChannel(ctx, h.userID, group, h.cfg.xpostChannelID)
		Expect(err).NotTo(HaveOccurred())

		var captured error
		b := newCommandBot(h, stub, &captured)
		seed := seedChannel(h, h.cfg.channelID, "share-cpex")
		before := newestMessageID(h, h.cfg.xpostChannelID)

		invokeCommand(b, h, h.userID, h.cfg.channelID, seed.ID,
			"bt!crosspostexclude "+artURL+" <#"+h.cfg.xpostChannelID+">")

		Expect(captured).NotTo(HaveOccurred())
		Expect(embedImagesAfter(h, h.cfg.channelID, seed.ID)).To(ContainElement("https://example.com/x.png"))
		Expect(newestMessageID(h, h.cfg.xpostChannelID)).To(Equal(before))
		cleanupAfter(h, h.cfg.channelID, seed.ID)
	})
})
