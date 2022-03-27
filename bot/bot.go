package bot

import (
	"context"
	"fmt"
	"time"

	"github.com/VTGare/boe-tea-go/artworks"
	"github.com/VTGare/boe-tea-go/internal/apis/nhentai"
	"github.com/VTGare/boe-tea-go/internal/cache"
	"github.com/VTGare/boe-tea-go/internal/config"
	"github.com/VTGare/boe-tea-go/repost"
	"github.com/VTGare/boe-tea-go/store"
	"github.com/VTGare/sengoku"
	gocache "github.com/patrickmn/go-cache"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/session/shard"
	"github.com/diamondburned/arikawa/v3/state"
)

type Bot struct {
	Store store.Store

	// misc.
	Log    *zap.SugaredLogger
	Config *config.Config

	// caches
	ArtworkCache *gocache.Cache
	BannedUsers  *gocache.Cache
	EmbedCache   *cache.EmbedCache

	// services
	Sengoku          *sengoku.Sengoku
	NHentai          *nhentai.API
	ArtworkProviders []artworks.Provider
	RepostDetector   repost.Detector

	Me           *discord.User
	Application  *discord.Application
	ShardManager *shard.Manager
	StartupTime  time.Time
}

func New(config *config.Config, store store.Store, logger *zap.SugaredLogger, rd repost.Detector) (*Bot, error) {
	var (
		banned       = gocache.New(5*time.Second, 5*time.Second)
		artworkCache = gocache.New(30*time.Minute, 45*time.Hour)
		sg           = sengoku.NewSengoku(config.SauceNAO, sengoku.Config{DB: 999, Results: 10})
	)

	return &Bot{
		Log:            logger,
		Config:         config,
		Store:          store,
		RepostDetector: rd,
		ArtworkCache:   artworkCache,
		BannedUsers:    banned,
		EmbedCache:     cache.NewEmbedCache(),
		NHentai:        nhentai.New(),
		Sengoku:        sg,
	}, nil
}

func (b *Bot) WithShardManager(mgr *shard.Manager) {
	b.ShardManager = mgr
}

func (b *Bot) WithProvider(provider artworks.Provider) {
	b.ArtworkProviders = append(b.ArtworkProviders, provider)
}

func (b *Bot) WithCommands(commands []api.CreateCommandData) error {
	eg, _ := errgroup.WithContext(context.Background())
	b.ShardManager.ForEach(func(shard shard.Shard) {
		eg.Go(func() error {
			state := shard.(*state.State)

			switch b.Config.Env {
			case config.DevEnvironment:
				if _, err := state.BulkOverwriteGuildCommands(b.Application.ID, b.Config.Discord.TestGuildID, commands); err != nil {
					return fmt.Errorf("failed to overwrite guild commands: %w", err)
				}
			case config.ProdEnvironment:
				if _, err := state.BulkOverwriteCommands(b.Application.ID, commands); err != nil {
					return fmt.Errorf("failed to overwrite commands: %w", err)
				}
			}

			return nil
		})
	})

	err := eg.Wait()
	if err != nil {
		return fmt.Errorf("failed to register commands: %w", err)
	}

	return nil
}

func (b *Bot) Open() error {
	err := b.ShardManager.Open(context.Background())
	if err != nil {
		return err
	}

	b.Log.Info("opened a connection to gateway")
	state := b.ShardManager.Shard(0).(*state.State)
	me, err := state.Me()
	if err != nil {
		return fmt.Errorf("failed to get me: %w", err)
	}
	b.Me = me

	app, err := state.CurrentApplication()
	if err != nil {
		return fmt.Errorf("failed to get current application: %w", err)
	}

	b.Application = app
	b.StartupTime = time.Now()
	return nil
}

func (b *Bot) Close() error {
	return b.ShardManager.Close()
}
