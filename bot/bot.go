package bot

import (
	"context"
	"fmt"
	"time"

	"github.com/ReneKroon/ttlcache"
	"github.com/VTGare/boe-tea-go/artworks"
	"github.com/VTGare/boe-tea-go/internal/apis/nhentai"
	"github.com/VTGare/boe-tea-go/internal/cache"
	"github.com/VTGare/boe-tea-go/internal/config"
	"github.com/VTGare/boe-tea-go/repost"
	"github.com/VTGare/boe-tea-go/stats"
	"github.com/VTGare/boe-tea-go/store"
	"github.com/VTGare/gumi"
	"github.com/VTGare/sengoku"
	"github.com/bwmarrin/discordgo"
	goCache "github.com/patrickmn/go-cache"
	"github.com/servusdei2018/shards/v2"
	"go.uber.org/zap"
)

type Bot struct {
	// misc.
	Log       *zap.SugaredLogger
	Config    *config.Config
	Stats     *stats.Stats
	StartTime time.Time
	Router    *gumi.Router
	Context   context.Context

	// caches
	BannedUsers  *ttlcache.Cache
	EmbedCache   *cache.EmbedCache
	ArtworkCache *goCache.Cache

	// services
	Sengoku          *sengoku.Sengoku
	NHentai          *nhentai.API
	ArtworkProviders []artworks.Provider
	RepostDetector   repost.Detector

	ShardManager *shards.Manager
	Store        store.Store
}

func New(
	config *config.Config,
	store store.Store,
	logger *zap.SugaredLogger,
	rd repost.Detector,
) (*Bot, error) {
	mgr, err := shards.New("Bot " + config.Discord.Token)
	if err != nil {
		return nil, fmt.Errorf("failed to init a shard manager: %w", err)
	}

	mgr.RegisterIntent(discordgo.IntentsAllWithoutPrivileged | discordgo.IntentMessageContent)
	banned := ttlcache.NewCache()
	banned.SetTTL(15 * time.Second)

	sg := sengoku.NewSengoku(config.SauceNAO, sengoku.Config{
		DB:      999,
		Results: 10,
	})

	nh, err := nhentai.New()
	if err != nil {
		return nil, fmt.Errorf("failed to create nhentai api client: %w", err)
	}

	return &Bot{
		Log:            logger,
		Config:         config,
		RepostDetector: rd,
		BannedUsers:    banned,
		EmbedCache:     cache.NewEmbedCache(),
		ArtworkCache:   goCache.New(60*time.Minute, 90*time.Minute),
		NHentai:        nh,
		Sengoku:        sg,
		ShardManager:   mgr,
		Store:          store,
	}, nil
}

func (b *Bot) AddRouter(router *gumi.Router) {
	b.Router = gumi.Create(router)
}

func (b *Bot) AddProvider(provider artworks.Provider) {
	b.ArtworkProviders = append(b.ArtworkProviders, provider)
}

func (b *Bot) AddHandler(handler any) {
	b.ShardManager.AddHandler(handler)
}

func (b *Bot) Start(ctx context.Context) error {
	b.ShardManager.AddHandler(b.Router.Handler())

	b.StartTime = time.Now()
	b.Stats = stats.New(b.Router, b.ArtworkProviders)
	b.Context = ctx

	b.Log.Debug("starting a bot")
	if err := b.ShardManager.Start(); err != nil {
		return err
	}

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		b.Store.Close(shutdownCtx)
		b.RepostDetector.Close()
		b.ShardManager.Shutdown()

		return ctx.Err()
	}
}
