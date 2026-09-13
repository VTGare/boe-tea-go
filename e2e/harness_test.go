//go:build e2e

package e2e

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/VTGare/boe-tea-go/artworks"
	"github.com/VTGare/boe-tea-go/bot"
	"github.com/VTGare/boe-tea-go/internal/cache"
	"github.com/VTGare/boe-tea-go/internal/sender"
	"github.com/VTGare/boe-tea-go/internal/spool"
	"github.com/VTGare/boe-tea-go/post"
	"github.com/VTGare/boe-tea-go/repost"
	"github.com/VTGare/boe-tea-go/store"
	"github.com/VTGare/boe-tea-go/store/postgres"
	"github.com/bwmarrin/discordgo"
	goCache "github.com/patrickmn/go-cache"
	"go.uber.org/zap"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const (
	defaultTestDSN   = "postgres://boetea:test@127.0.0.1:5433/boetea_test?sslmode=disable"
	repostNoticeName = "Repost detected"
)

type e2eConfig struct {
	token          string
	guildID        string
	channelID      string
	xpostChannelID string
	dsn            string
	userID         string
}

func (c e2eConfig) hasXpost() bool {
	return c.xpostChannelID != ""
}

func loadConfig() (e2eConfig, bool) {
	cfg := e2eConfig{
		token:          os.Getenv("E2E_DISCORD_TOKEN"),
		guildID:        os.Getenv("E2E_GUILD_ID"),
		channelID:      os.Getenv("E2E_CHANNEL_ID"),
		xpostChannelID: os.Getenv("E2E_XPOST_CHANNEL_ID"),
		dsn:            os.Getenv("E2E_POSTGRES_DSN"),
		userID:         os.Getenv("E2E_TEST_USER_ID"),
	}

	if cfg.token == "" || cfg.guildID == "" || cfg.channelID == "" {
		return e2eConfig{}, false
	}

	if cfg.dsn == "" {
		cfg.dsn = defaultTestDSN
	}

	return cfg, true
}

type harness struct {
	cfg     e2eConfig
	session *discordgo.Session
	store   store.Store
	sender  *sender.DiscordSender
	botID   string
	userID  string
	log     *zap.SugaredLogger
}

func setupHarness(ctx context.Context, cfg e2eConfig) (*harness, error) {
	session, err := discordgo.New("Bot " + cfg.token)
	if err != nil {
		return nil, err
	}

	session.Identify.Intents = discordgo.IntentsAllWithoutPrivileged | discordgo.IntentMessageContent

	if err := session.Open(); err != nil {
		return nil, err
	}

	me, err := session.User("@me")
	if err != nil {
		session.Close()

		return nil, err
	}

	st, err := postgres.New(ctx, cfg.dsn)
	if err != nil {
		session.Close()

		return nil, err
	}

	if err := st.Init(ctx); err != nil {
		session.Close()

		return nil, err
	}

	log := zap.NewNop().Sugar()

	h := &harness{
		cfg:     cfg,
		session: session,
		store:   st,
		sender:  sender.NewDiscordSender(nil, log, session),
		botID:   me.ID,
		userID:  cfg.userID,
		log:     log,
	}

	if h.userID == "" {
		h.userID = h.botID
	}

	spool.Configure(spool.DefaultConfig())

	if err := h.ensureUser(ctx); err != nil {
		h.close()

		return nil, err
	}

	if err := h.ensureGuild(ctx, baselineGuild); err != nil {
		h.close()

		return nil, err
	}

	return h, nil
}

func (h *harness) close() {
	if h.session != nil {
		h.session.Close()
	}

	if h.store != nil {
		h.store.Close(context.Background())
	}
}

func baselineGuild(g *store.Guild) {
	g.Limit = 10
	g.NSFW = true
	g.Pixiv = true
	g.Twitter = true
	g.Deviant = true
	g.Bluesky = true
	g.Tags = true
	g.FlavorText = false
	g.Crosspost = true
	g.Reactions = false
	g.SkipFirst = false
	g.Repost = store.GuildRepostEnabled
	g.RepostExpiration = time.Hour
	g.ArtChannels = []string{}
}

func (h *harness) ensureGuild(ctx context.Context, mutate func(*store.Guild)) error {
	g, err := h.store.Guild(ctx, h.cfg.guildID)
	if err != nil {
		if !errors.Is(err, store.ErrGuildNotFound) {
			return err
		}

		g, err = h.store.CreateGuild(ctx, h.cfg.guildID)
		if err != nil {
			return err
		}
	}

	if g == nil {
		return errors.New("e2e: guild missing after ensure")
	}

	if mutate != nil {
		mutate(g)
	}

	_, err = h.store.UpdateGuild(ctx, g)

	return err
}

func (h *harness) ensureUser(ctx context.Context) error {
	u, err := h.store.User(ctx, h.userID)
	if err != nil {
		return err
	}

	if u == nil {
		return errors.New("e2e: user missing after ensure")
	}

	u.Crosspost = true
	u.Ignore = false

	_, err = h.store.UpdateUser(ctx, u)

	return err
}

type recordedStats struct {
	mu        sync.Mutex
	providers []string
}

func (r *recordedStats) record(p artworks.Provider) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.providers = append(r.providers, fmt.Sprintf("%T", p))
}

func (r *recordedStats) all() []string {
	r.mu.Lock()
	defer r.mu.Unlock()

	return append([]string(nil), r.providers...)
}

func (h *harness) newPoster(stub *stubProvider, detector repost.Detector, rec *recordedStats) *post.Poster {
	return post.NewPoster(post.Deps{
		Guilds:       h.store,
		Users:        h.store,
		Match:        stub.match,
		Reposts:      detector,
		ArtworkCache: goCache.New(5*time.Minute, 10*time.Minute),
		Sender:       h.sender,
		Log:          h.log,
		RecordArtwork: func(p artworks.Provider) {
			if rec != nil {
				rec.record(p)
			}
		},
	})
}

func (h *harness) newRun(channelID, messageID string, urls ...string) post.Post {
	return post.Post{
		GuildID:    h.cfg.guildID,
		ChannelID:  channelID,
		MessageID:  messageID,
		AuthorID:   h.userID,
		AuthorName: "e2e",
		URLs:       urls,
		Skip:       post.SkipFilter{Indices: map[int]struct{}{}},
	}
}

func (h *harness) seed(channelID, tag string) (*discordgo.Message, error) {
	return h.session.ChannelMessageSend(channelID, "e2e seed "+tag)
}

func (h *harness) deleteAll(channelID string, ids ...string) {
	for _, id := range ids {
		if id == "" {
			continue
		}

		_ = h.session.ChannelMessageDelete(channelID, id)
	}
}

func newTestBot(h *harness, stub *stubProvider, detector repost.Detector) *bot.Bot {
	b := &bot.Bot{
		Log:            zap.NewNop().Sugar(),
		Store:          h.store,
		RepostDetector: detector,
		ArtworkCache:   goCache.New(5*time.Minute, 10*time.Minute),
		EmbedCache:     cache.NewEmbedCache(),
		Sender:         h.sender,
		Context:        context.Background(),
	}
	b.AddProvider(stub)

	return b
}

func newDetector() repost.Detector {
	detector := repost.NewMemory()
	DeferCleanup(func() { detector.Close() })

	return detector
}

func seedChannel(h *harness, channelID, tag string) *discordgo.Message {
	seed, err := h.seed(channelID, tag)
	Expect(err).NotTo(HaveOccurred())
	DeferCleanup(func() { h.deleteAll(channelID, seed.ID) })

	return seed
}

func cleanupSent(h *harness, sent []*cache.MessageInfo) {
	DeferCleanup(func() {
		for _, info := range sent {
			h.deleteAll(info.ChannelID, info.MessageID)
		}
	})
}

func cleanupAfter(h *harness, channelID, seedID string) {
	DeferCleanup(func() {
		after, _ := h.session.ChannelMessages(channelID, 25, "", seedID, "")
		h.deleteAll(channelID, messageIDsAfter(after)...)
	})
}

func uniqueURL(tag string) string {
	return fmt.Sprintf("https://example.com/e2e/%d/%s", time.Now().UnixNano(), tag)
}

func messageIDsAfter(msgs []*discordgo.Message) []string {
	ids := make([]string, 0, len(msgs))
	for _, m := range msgs {
		if m != nil {
			ids = append(ids, m.ID)
		}
	}

	return ids
}

func findEmbedByTitle(msgs []*discordgo.Message, title string) *discordgo.Message {
	for _, m := range msgs {
		if m == nil {
			continue
		}

		for _, e := range m.Embeds {
			if e != nil && e.Title == title {
				return m
			}
		}
	}

	return nil
}

type stubArtwork struct {
	id        string
	url       string
	previews  []string
	files     []*discordgo.File
	renderErr error
}

func (s *stubArtwork) StoreArtwork() *store.Artwork {
	return &store.Artwork{URL: s.url}
}

func (s *stubArtwork) Render() (artworks.Rendered, error) {
	if s.renderErr != nil {
		return artworks.Rendered{}, s.renderErr
	}

	rendered := artworks.Rendered{
		Title: "E2E " + s.id,
		URL:   s.url,
		Files: s.files,
	}

	for _, preview := range s.previews {
		rendered.Images = append(rendered.Images, artworks.RenderedImage{Preview: preview})
	}

	return rendered, nil
}

func (s *stubArtwork) ID() string {
	return s.id
}

func (s *stubArtwork) URL() string {
	return s.url
}

func (s *stubArtwork) Len() int {
	if len(s.previews) > 0 {
		return len(s.previews)
	}

	if len(s.files) > 0 {
		return 1
	}

	return 0
}

type stubProvider struct {
	mu   sync.Mutex
	ids  map[string]string
	arts map[string]*stubArtwork
}

func newStubProvider() *stubProvider {
	return &stubProvider{
		ids:  make(map[string]string),
		arts: make(map[string]*stubArtwork),
	}
}

func (p *stubProvider) add(url, id string, art *stubArtwork) {
	p.mu.Lock()
	defer p.mu.Unlock()

	art.id = id
	art.url = url
	p.ids[url] = id
	p.arts[id] = art
}

func (p *stubProvider) Match(url string) (string, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()

	id, ok := p.ids[url]

	return id, ok
}

func (p *stubProvider) Find(id string) (artworks.Artwork, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	art, ok := p.arts[id]
	if !ok {
		return nil, fmt.Errorf("e2e: stub artwork %v not found", id)
	}

	return art, nil
}

func (p *stubProvider) Enabled(*store.Guild) bool {
	return true
}

func (p *stubProvider) match(url string) (string, artworks.Provider) {
	if id, ok := p.Match(url); ok {
		return id, p
	}

	return "", nil
}

var e2eHarness *harness

var _ = BeforeSuite(func() {
	cfg, ok := loadConfig()
	if !ok {
		Skip("e2e env not configured: set E2E_DISCORD_TOKEN, E2E_GUILD_ID and E2E_CHANNEL_ID")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	h, err := setupHarness(ctx, cfg)
	Expect(err).NotTo(HaveOccurred())

	e2eHarness = h
})

var _ = AfterSuite(func() {
	if e2eHarness != nil {
		e2eHarness.close()
	}
})
