package bot

import (
	"time"

	"github.com/ReneKroon/ttlcache"
	"github.com/VTGare/boe-tea-go/internal/cache"
	"github.com/VTGare/boe-tea-go/internal/config"
	"github.com/VTGare/boe-tea-go/pkg/artworks"
	"github.com/VTGare/boe-tea-go/pkg/metrics"
	"github.com/VTGare/boe-tea-go/pkg/models"
	artworksModel "github.com/VTGare/boe-tea-go/pkg/models/artworks"
	"github.com/VTGare/boe-tea-go/pkg/models/guilds"
	"github.com/VTGare/boe-tea-go/pkg/models/users"
	"github.com/VTGare/boe-tea-go/pkg/repost"
	"github.com/VTGare/gumi"
	"github.com/VTGare/sengoku"
	"github.com/bwmarrin/discordgo"
	"go.uber.org/zap"
)

type Bot struct {
	Guilds           guilds.Service
	Users            users.Service
	Artworks         artworksModel.Service
	Log              *zap.SugaredLogger
	Config           *config.Config
	Router           *gumi.Router
	ArtworkProviders []artworks.Provider
	RepostDetector   repost.Detector
	BannedUsers      *ttlcache.Cache
	EmbedCache       *cache.EmbedCache
	Sengoku          *sengoku.Sengoku
	Metrics          *metrics.Metrics
	s                *discordgo.Session
}

func New(config *config.Config, models *models.Models, logger *zap.SugaredLogger, rd repost.Detector) (*Bot, error) {
	dg, err := discordgo.New("Bot " + config.Discord.Token)
	if err != nil {
		return nil, err
	}
	dg.Identify.Intents = discordgo.IntentsAllWithoutPrivileged

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
		Sengoku:        sg,
		s:              dg,
	}, nil
}

func (b *Bot) AddRouter(router *gumi.Router) {
	b.Router = gumi.Create(router)
}

func (b *Bot) AddProvider(provider artworks.Provider) {
	b.ArtworkProviders = append(b.ArtworkProviders, provider)
}

func (b *Bot) AddHandler(handler interface{}) {
	b.s.AddHandler(handler)
}

func (b *Bot) Open() error {
	b.Router.Initialize(b.s)

	err := b.s.Open()
	if err != nil {
		return err
	}

	b.Log.Info("Started a bot.")
	return nil
}

func (b *Bot) Close() error {
	return b.s.Close()
}
