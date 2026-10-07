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
	"github.com/VTGare/boe-tea-go/internal/widget"
	"github.com/VTGare/boe-tea-go/post"
	"github.com/VTGare/boe-tea-go/repost"
	"github.com/VTGare/boe-tea-go/store"
	"github.com/VTGare/boe-tea-go/store/postgres"
	"github.com/disgoorg/disgo"
	disgobot "github.com/disgoorg/disgo/bot"
	disgocache "github.com/disgoorg/disgo/cache"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/gateway"
	"github.com/disgoorg/snowflake/v2"
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
	guildID        snowflake.ID
	channelID      snowflake.ID
	xpostChannelID snowflake.ID
	dsn            string
	userID         snowflake.ID
}

func (c e2eConfig) hasXpost() bool {
	return c.xpostChannelID != 0
}

// envID is 0 when the variable is unset or isn't an ID.
func envID(name string) snowflake.ID {
	id, _ := snowflake.Parse(os.Getenv(name))

	return id
}

func loadConfig() (e2eConfig, bool) {
	cfg := e2eConfig{
		token:          os.Getenv("E2E_DISCORD_TOKEN"),
		guildID:        envID("E2E_GUILD_ID"),
		channelID:      envID("E2E_CHANNEL_ID"),
		xpostChannelID: envID("E2E_XPOST_CHANNEL_ID"),
		dsn:            os.Getenv("E2E_POSTGRES_DSN"),
		userID:         envID("E2E_TEST_USER_ID"),
	}

	if cfg.token == "" || cfg.guildID == 0 || cfg.channelID == 0 {
		return e2eConfig{}, false
	}

	if cfg.dsn == "" {
		cfg.dsn = defaultTestDSN
	}

	return cfg, true
}

type harness struct {
	cfg    e2eConfig
	client *disgobot.Client
	store  store.Store
	sender *sender.DiscordSender
	botID  snowflake.ID
	userID snowflake.ID
	log    *zap.SugaredLogger
}

// newClient connects one gateway and waits until the test channel and the
// bot's member are cached, which permission checks need.
func newClient(ctx context.Context, cfg e2eConfig) (*disgobot.Client, error) {
	client, err := disgo.New(cfg.token,
		disgobot.WithGatewayConfigOpts(
			gateway.WithIntents(gateway.IntentsNonPrivileged, gateway.IntentMessageContent),
		),
		disgobot.WithCacheConfigOpts(
			disgocache.WithCaches(disgocache.FlagGuilds, disgocache.FlagChannels, disgocache.FlagRoles, disgocache.FlagMembers),
		),
		disgobot.WithEventManagerConfigOpts(disgobot.WithAsyncEventsEnabled()),
	)
	if err != nil {
		return nil, err
	}

	if err := client.OpenGateway(ctx); err != nil {
		return nil, err
	}

	for {
		_, channel := client.Caches.Channel(cfg.channelID)
		_, member := client.Caches.Member(cfg.guildID, client.ID())
		if channel && member {
			return client, nil
		}

		select {
		case <-ctx.Done():
			client.Close(context.Background())

			return nil, fmt.Errorf("e2e: test guild never arrived: %w", ctx.Err())
		case <-time.After(100 * time.Millisecond):
		}
	}
}

func setupHarness(ctx context.Context, cfg e2eConfig) (*harness, error) {
	client, err := newClient(ctx, cfg)
	if err != nil {
		return nil, err
	}

	st, err := postgres.New(ctx, cfg.dsn)
	if err != nil {
		client.Close(context.Background())

		return nil, err
	}

	if err := st.Init(ctx); err != nil {
		client.Close(context.Background())

		return nil, err
	}

	log := zap.NewNop().Sugar()

	h := &harness{
		cfg:    cfg,
		client: client,
		store:  st,
		sender: sender.NewDiscordSender(client, log),
		botID:  client.ID(),
		userID: cfg.userID,
		log:    log,
	}

	if h.userID == 0 {
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
	if h.client != nil {
		h.client.Close(context.Background())
	}

	if h.store != nil {
		h.store.Close(context.Background())
	}
}

func baselineGuild(g *store.Guild) {
	g.Posting = store.Posting{Limit: 10, Tags: true, Crosspost: true, NSFWQuotes: true}
	g.DisabledProviders = []string{}
	g.Repost = store.Repost{Mode: store.RepostNotify, TTL: time.Hour}
	g.ArtChannels = []string{}
}

// guildOverride tweaks the stored guild in memory, for values the store
// rejects, such as repost TTLs under a minute.
type guildOverride struct {
	post.GuildStore
	mutate func(*store.Guild)
}

func (o guildOverride) Guild(ctx context.Context, guildID string) (*store.Guild, error) {
	g, err := o.GuildStore.Guild(ctx, guildID)
	if err == nil && g != nil {
		o.mutate(g)
	}

	return g, err
}

func (h *harness) ensureGuild(ctx context.Context, mutate func(*store.Guild)) error {
	g, err := h.store.Guild(ctx, h.cfg.guildID.String())
	if err != nil {
		if !errors.Is(err, store.ErrGuildNotFound) {
			return err
		}

		g, err = h.store.CreateGuild(ctx, h.cfg.guildID.String())
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
	u, err := h.store.User(ctx, h.userID.String())
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
	return h.newPosterWith(h.store, stub, detector, rec)
}

func (h *harness) newPosterWith(guilds post.GuildStore, stub *stubProvider, detector repost.Detector, rec *recordedStats) *post.Poster {
	return post.NewPoster(post.Deps{
		Guilds:       guilds,
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

func (h *harness) newRun(channelID, messageID snowflake.ID, urls ...string) post.Post {
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

func (h *harness) seed(channelID snowflake.ID, tag string) (*discord.Message, error) {
	return h.client.Rest.CreateMessage(channelID, discord.MessageCreate{Content: "e2e seed " + tag})
}

func (h *harness) deleteAll(channelID snowflake.ID, ids ...snowflake.ID) {
	for _, id := range ids {
		if id == 0 {
			continue
		}

		_ = h.client.Rest.DeleteMessage(channelID, id)
	}
}

func (h *harness) sentMessage(info *cache.MessageInfo) (*discord.Message, error) {
	return h.client.Rest.GetMessage(parseID(info.ChannelID), parseID(info.MessageID))
}

func (h *harness) messagesAfter(channelID, afterID snowflake.ID, limit int) ([]discord.Message, error) {
	return h.client.Rest.GetMessages(channelID, 0, 0, afterID, limit)
}

func parseID(s string) snowflake.ID {
	id, _ := snowflake.Parse(s)

	return id
}

// newID makes an ID no real user has, for authors of fake messages.
func newID() snowflake.ID {
	return snowflake.New(time.Now())
}

func newTestBot(h *harness, stub *stubProvider, detector repost.Detector) *bot.Bot {
	b := &bot.Bot{
		Log:              zap.NewNop().Sugar(),
		Store:            h.store,
		RepostDetector:   detector,
		ArtworkCache:     goCache.New(5*time.Minute, 10*time.Minute),
		EmbedCache:       cache.NewEmbedCache(),
		Sender:           h.sender,
		Client:           h.client,
		Context:          context.Background(),
		WidgetDispatcher: widget.NewDispatcher(),
	}
	b.AddProvider(stub)

	return b
}

func newDetector() repost.Detector {
	detector := repost.NewMemory()
	DeferCleanup(func() { detector.Close() })

	return detector
}

func seedChannel(h *harness, channelID snowflake.ID, tag string) *discord.Message {
	seed, err := h.seed(channelID, tag)
	Expect(err).NotTo(HaveOccurred())
	DeferCleanup(func() { h.deleteAll(channelID, seed.ID) })

	return seed
}

func cleanupSent(h *harness, sent []*cache.MessageInfo) {
	DeferCleanup(func() {
		for _, info := range sent {
			h.deleteAll(parseID(info.ChannelID), parseID(info.MessageID))
		}
	})
}

func cleanupAfter(h *harness, channelID, seedID snowflake.ID) {
	DeferCleanup(func() {
		after, _ := h.messagesAfter(channelID, seedID, 25)
		h.deleteAll(channelID, messageIDsAfter(after)...)
	})
}

func uniqueURL(tag string) string {
	return fmt.Sprintf("https://example.com/e2e/%d/%s", time.Now().UnixNano(), tag)
}

func messageIDsAfter(msgs []discord.Message) []snowflake.ID {
	ids := make([]snowflake.ID, 0, len(msgs))
	for _, m := range msgs {
		ids = append(ids, m.ID)
	}

	return ids
}

func findEmbedByTitle(msgs []discord.Message, title string) *discord.Message {
	for i, m := range msgs {
		for _, e := range m.Embeds {
			if e.Title == title {
				return &msgs[i]
			}
		}
	}

	return nil
}

type stubArtwork struct {
	id        string
	url       string
	previews  []string
	files     []*discord.File
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

func (*stubProvider) Info() artworks.Info {
	return artworks.Info{Key: "stub", Label: "Stub"}
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
