//go:build e2e

package e2e

import (
	"context"
	"os"
	"time"

	"github.com/VTGare/boe-tea-go/artworks"
	"github.com/VTGare/boe-tea-go/artworks/bluesky"
	"github.com/VTGare/boe-tea-go/artworks/deviant"
	"github.com/VTGare/boe-tea-go/artworks/pixiv"
	"github.com/VTGare/boe-tea-go/artworks/twitter"
	"github.com/VTGare/boe-tea-go/post"
	"github.com/VTGare/boe-tea-go/store"
	goCache "github.com/patrickmn/go-cache"
	"go.uber.org/zap"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func liveProviders() map[string]struct {
	provider artworks.Provider
	urlEnv   string
} {
	return map[string]struct {
		provider artworks.Provider
		urlEnv   string
	}{
		"twitter": {provider: twitter.New(), urlEnv: "E2E_TWITTER_URL"},
		"deviant": {provider: deviant.New(), urlEnv: "E2E_DEVIANT_URL"},
		"bluesky": {provider: bluesky.New(), urlEnv: "E2E_BLUESKY_URL"},
		"pixiv":   {provider: pixiv.New(""), urlEnv: "E2E_PIXIV_URL"},
	}
}

func liveURLs() []string {
	urls := make([]string, 0, 4)
	for _, env := range []string{"E2E_TWITTER_URL", "E2E_DEVIANT_URL", "E2E_BLUESKY_URL", "E2E_PIXIV_URL"} {
		if env == "E2E_PIXIV_URL" && os.Getenv("E2E_PIXIV_REFRESH_TOKEN") == "" {
			continue
		}

		if raw := os.Getenv(env); raw != "" {
			urls = append(urls, raw)
		}
	}

	return urls
}

func livePoster(h *harness, match func(string) (string, artworks.Provider)) *post.Poster {
	return post.NewPoster(post.Deps{
		Guilds:       h.store,
		Users:        h.store,
		Match:        match,
		Reposts:      newDetector(),
		ArtworkCache: goCache.New(5*time.Minute, 10*time.Minute),
		Sender:       h.sender,
		Log:          zap.NewNop().Sugar(),
	})
}

var _ = Describe("Live providers", func() {
	BeforeEach(func() {
		if os.Getenv("E2E_LIVE_ARTWORKS") == "" {
			Skip("set E2E_LIVE_ARTWORKS=1 with pinned artwork URLs to run live provider specs")
		}
	})

	It("recognizes pinned artwork links", func() {
		for name, p := range liveProviders() {
			raw := os.Getenv(p.urlEnv)
			if raw == "" {
				continue
			}

			id, ok := p.provider.Match(raw)
			Expect(ok).To(BeTrue(), name+" should match "+raw)
			Expect(id).NotTo(BeEmpty())
		}
	})

	It("posts pinned live artworks", func() {
		h := e2eHarness

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		Expect(h.ensureGuild(ctx, baselineGuild)).To(Succeed())

		providers := []artworks.Provider{twitter.New(), deviant.New(), bluesky.New()}

		if os.Getenv("E2E_PIXIV_REFRESH_TOKEN") != "" {
			if err := pixiv.LoadAuth("", os.Getenv("E2E_PIXIV_REFRESH_TOKEN")); err == nil {
				providers = append(providers, pixiv.New(""))
			}
		}

		urls := liveURLs()
		if len(urls) == 0 {
			Skip("no pinned provider URLs set")
		}

		poster := livePoster(h, func(url string) (string, artworks.Provider) {
			for _, p := range providers {
				if id, ok := p.Match(url); ok {
					return id, p
				}
			}

			return "", nil
		})

		seed := seedChannel(h, h.cfg.channelID, "live")

		sent, err := poster.Send(ctx, h.newRun(h.cfg.channelID, seed.ID, urls...))
		Expect(err).NotTo(HaveOccurred())
		Expect(sent).NotTo(BeEmpty())
		cleanupSent(h, sent)
	})

	It("skips the first page for skip-first guilds", func() {
		raw := os.Getenv("E2E_TWITTER_URL")
		if raw == "" {
			Skip("set E2E_TWITTER_URL to a photo tweet")
		}

		h := e2eHarness

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		Expect(h.ensureGuild(ctx, func(g *store.Guild) {
			baselineGuild(g)
			g.SkipFirst = true
		})).To(Succeed())
		DeferCleanup(func() { _ = h.ensureGuild(context.Background(), baselineGuild) })

		provider := twitter.New()

		id, ok := provider.Match(raw)
		Expect(ok).To(BeTrue())

		found, err := provider.Find(id)
		Expect(err).NotTo(HaveOccurred())

		tweet, ok := found.(*twitter.Artwork)
		Expect(ok).To(BeTrue())

		poster := livePoster(h, func(string) (string, artworks.Provider) { return id, provider })
		seed := seedChannel(h, h.cfg.channelID, "skipfirst")

		sent, err := poster.Send(ctx, h.newRun(h.cfg.channelID, seed.ID, raw))
		Expect(err).NotTo(HaveOccurred())
		cleanupSent(h, sent)

		if len(tweet.Videos) > 0 {
			Expect(sent).To(HaveLen(1))
		} else {
			Expect(sent).To(HaveLen(max(len(tweet.Photos)-1, 0)))
		}
	})

	It("posts text-only tweets from commands, skips them otherwise", func() {
		raw := os.Getenv("E2E_TWITTER_TEXT_URL")
		if raw == "" {
			Skip("set E2E_TWITTER_TEXT_URL to a text-only tweet")
		}

		h := e2eHarness

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		Expect(h.ensureGuild(ctx, baselineGuild)).To(Succeed())

		provider := twitter.New()

		id, ok := provider.Match(raw)
		Expect(ok).To(BeTrue())

		found, err := provider.Find(id)
		Expect(err).NotTo(HaveOccurred())
		Expect(found.Len()).To(BeZero())

		rendered, err := found.Render()
		Expect(err).NotTo(HaveOccurred())

		poster := livePoster(h, func(string) (string, artworks.Provider) { return id, provider })
		seed := seedChannel(h, h.cfg.channelID, "textonly")

		auto, err := poster.Send(ctx, h.newRun(h.cfg.channelID, seed.ID, raw))
		Expect(err).NotTo(HaveOccurred())
		Expect(auto).To(BeEmpty())

		cmdRun := h.newRun(h.cfg.channelID, seed.ID, raw)
		cmdRun.IsCommand = true

		sent, err := poster.Send(ctx, cmdRun)
		Expect(err).NotTo(HaveOccurred())
		Expect(sent).To(HaveLen(1))
		cleanupSent(h, sent)

		msg, err := h.session.ChannelMessage(h.cfg.channelID, sent[0].MessageID)
		Expect(err).NotTo(HaveOccurred())
		Expect(msg.Embeds).To(HaveLen(1))
		Expect(msg.Embeds[0].Title).To(Equal(rendered.Title))
	})
})
