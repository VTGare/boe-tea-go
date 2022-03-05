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
	"github.com/VTGare/boe-tea-go/metrics"
	"github.com/VTGare/boe-tea-go/models"
	artworksModel "github.com/VTGare/boe-tea-go/models/artworks"
	"github.com/VTGare/boe-tea-go/models/guilds"
	"github.com/VTGare/boe-tea-go/models/users"
	"github.com/VTGare/boe-tea-go/repost"
	"github.com/VTGare/sengoku"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/session/shard"
	"github.com/diamondburned/arikawa/v3/state"
)

type Bot struct {
	// models
	Guilds   guilds.Service
	Users    users.Service
	Artworks artworksModel.Service

	// misc.
	Log     *zap.SugaredLogger
	Config  *config.Config
	Metrics *metrics.Metrics

	// caches
	BannedUsers *ttlcache.Cache
	EmbedCache  *cache.EmbedCache

	// services
	Sengoku          *sengoku.Sengoku
	NHentai          *nhentai.API
	ArtworkProviders []artworks.Provider
	RepostDetector   repost.Detector

	Me           *discord.User
	Application  *discord.Application
	ShardManager *shard.Manager
}

func New(
	config *config.Config, models *models.Models, logger *zap.SugaredLogger,
	rd repost.Detector, handlers ...interface{},
) (*Bot, error) {
	banned := ttlcache.NewCache()
	banned.SetTTL(15 * time.Second)

	sg := sengoku.NewSengoku(config.SauceNAO, sengoku.Config{
		DB:      999,
		Results: 10,
	})

	return &Bot{
		Guilds:         models.Guilds,
		Users:          models.Users,
		Artworks:       models.Artworks,
		Log:            logger,
		Config:         config,
		RepostDetector: rd,
		BannedUsers:    banned,
		EmbedCache:     cache.NewEmbedCache(),
		Metrics:        metrics.New(),
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

			registeredCommands, err := state.Commands(b.Application.ID)
			if err != nil {
				return fmt.Errorf("failed to get current commands: %w", err)
			}

			takenNames := make(map[string]discord.CommandID)
			for _, cmd := range registeredCommands {
				takenNames[cmd.Name] = cmd.ID
			}

			switch b.Config.Env {
			case config.DevEnvironment:
				if _, err := state.BulkOverwriteGuildCommands(b.Application.ID, discord.GuildID(b.Config.TestGuildID), commands); err != nil {
					return fmt.Errorf("failed to overwrite guild commands: %w", err)
				}
			case config.ProdEnvironment:
				for _, cmd := range commands {
					if id, ok := takenNames[cmd.Name]; ok {
						_, err := state.EditCommand(b.Application.ID, id, cmd)
						if err != nil {
							return fmt.Errorf("failed to edit a command: %w", err)
						}
					} else {
						_, err := state.CreateCommand(b.Application.ID, cmd)
						if err != nil {
							return fmt.Errorf("failed to create a command: %w", err)
						}
					}
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
	return nil
}

func (b *Bot) Close() error {
	return b.ShardManager.Close()
}
