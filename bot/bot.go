package bot

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ReneKroon/ttlcache"
	"github.com/VTGare/boe-tea-go/artworks"
	"github.com/VTGare/boe-tea-go/internal/cache"
	"github.com/VTGare/boe-tea-go/internal/config"
	"github.com/VTGare/boe-tea-go/internal/sender"
	"github.com/VTGare/boe-tea-go/internal/widget"
	"github.com/VTGare/boe-tea-go/repost"
	"github.com/VTGare/boe-tea-go/stats"
	"github.com/VTGare/boe-tea-go/store"
	"github.com/VTGare/gumi/v2"
	"github.com/VTGare/sengoku"
	disgobot "github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/snowflake/v2"
	goCache "github.com/patrickmn/go-cache"
	"go.uber.org/zap"
)

type Bot struct {
	Log       *zap.SugaredLogger
	Config    *config.Config
	Stats     *stats.Stats
	StartTime time.Time
	Router    *gumi.Router
	Context   context.Context

	BannedUsers  *ttlcache.Cache
	EmbedCache   *cache.EmbedCache
	ArtworkCache *goCache.Cache

	Sengoku          *sengoku.Sengoku
	ArtworkProviders []artworks.Provider
	RepostDetector   repost.Detector
	Sender           sender.Sender
	WidgetDispatcher *widget.Dispatcher

	Client *disgobot.Client
	Store  store.Store

	listeners []disgobot.EventListener
}

func New(
	config *config.Config,
	client *disgobot.Client,
	store store.Store,
	logger *zap.SugaredLogger,
	rd repost.Detector,
) *Bot {
	banned := ttlcache.NewCache()
	banned.SetTTL(15 * time.Second)

	sg := sengoku.NewSengoku(config.SauceNAO, sengoku.Config{
		DB:      999,
		Results: 10,
	})

	return &Bot{
		Log:              logger,
		Config:           config,
		RepostDetector:   rd,
		BannedUsers:      banned,
		EmbedCache:       cache.NewEmbedCache(),
		ArtworkCache:     goCache.New(60*time.Minute, 90*time.Minute),
		Sengoku:          sg,
		Client:           client,
		Store:            store,
		WidgetDispatcher: widget.NewDispatcher(),
	}
}

func (b *Bot) AddRouter(r *gumi.Router) {
	b.Router = r
}

func (b *Bot) AddProvider(provider artworks.Provider) {
	b.ArtworkProviders = append(b.ArtworkProviders, provider)
}

// Listeners are attached in Start.
func (b *Bot) AddHandler(listener disgobot.EventListener) {
	b.listeners = append(b.listeners, listener)
}

// Start blocks until ctx ends. The client has either a shard manager or a
// single gateway.
func (b *Bot) Start(ctx context.Context) error {
	b.Client.AddEventListeners(b.Router, disgobot.NewListenerFunc(b.WidgetDispatcher.Handle))
	b.Client.AddEventListeners(b.listeners...)

	b.StartTime = time.Now()
	b.Stats = stats.New(b.Router, b.ArtworkProviders)
	b.Context = ctx

	// Syncing only needs REST, so commands are ready by the time the
	// shards connect.
	if guildID := b.Router.Config().DevGuildID; guildID != 0 {
		if err := b.Router.ClearCommands(b.Client, guildID); err != nil {
			b.Log.With("error", err).Error("failed to clear application commands")
		}
	}

	if err := b.Router.Sync(b.Client); err != nil {
		b.Log.With("error", err).Error("failed to sync application commands")
	}

	b.Log.Debug("starting a bot")

	open := b.Client.OpenGateway
	if b.Client.HasShardManager() {
		open = b.Client.OpenShardManager
	}

	if err := open(ctx); err != nil {
		return fmt.Errorf("failed to connect to Discord: %w", err)
	}

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	b.Store.Close(shutdownCtx)
	b.RepostDetector.Close()
	b.Client.Close(shutdownCtx)

	return ctx.Err()
}

// SourceKey is artworks.SourceKey for whichever provider matches url, or
// "" if none does.
func (b *Bot) SourceKey(url string) string {
	if _, p := b.Match(url); p != nil {
		key, _ := artworks.SourceKey(p, url)
		return key
	}

	return ""
}

// FindArtwork looks up a saved artwork by URL. The URL can be in any form
// its provider accepts. Artworks without a source key, e.g. from providers
// Boe Tea no longer supports, are found by their exact saved URL.
func (b *Bot) FindArtwork(ctx context.Context, url string) (*store.Artwork, error) {
	if key := b.SourceKey(url); key != "" {
		artwork, err := b.Store.ArtworkByKey(ctx, key)
		if !errors.Is(err, store.ErrArtworkNotFound) {
			return artwork, err
		}
	}

	return b.Store.Artwork(ctx, 0, url)
}

func (b *Bot) Match(url string) (string, artworks.Provider) {
	for _, provider := range b.ArtworkProviders {
		if id, ok := provider.Match(url); ok {
			return id, provider
		}
	}
	return "", nil
}

func (b *Bot) Latency(guildID snowflake.ID) time.Duration {
	switch {
	case b.Client.HasShardManager():
		if shard := b.Client.ShardManager.ShardByGuildID(guildID); shard != nil {
			return shard.Latency()
		}
	case b.Client.HasGateway():
		return b.Client.Gateway.Latency()
	}

	return 0
}

func (b *Bot) ShardCount() int {
	if !b.Client.HasShardManager() {
		return 1
	}

	count := 0
	for range b.Client.ShardManager.Shards() {
		count++
	}

	return count
}

func (b *Bot) AvatarURL() string {
	if u, ok := b.Client.Caches.SelfUser(); ok {
		return u.EffectiveAvatarURL()
	}

	return ""
}
