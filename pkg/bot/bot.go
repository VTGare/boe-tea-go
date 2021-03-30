package bot

import (
	"github.com/VTGare/boe-tea-go/internal/config"
	"github.com/VTGare/boe-tea-go/pkg/artworks"
	"github.com/VTGare/boe-tea-go/pkg/models"
	"github.com/VTGare/gumi"
	"github.com/bwmarrin/discordgo"
	"go.uber.org/zap"
)

type Bot struct {
	Models           *models.Models
	Logger           *zap.SugaredLogger
	Config           *config.Config
	Router           *gumi.Router
	ArtworkProviders []artworks.Provider
	s                *discordgo.Session
}

func New(config *config.Config, models *models.Models, logger *zap.SugaredLogger) (*Bot, error) {
	dg, err := discordgo.New("Bot " + config.Discord.Token)
	if err != nil {
		return nil, err
	}
	dg.Identify.Intents = discordgo.IntentsAllWithoutPrivileged

	return &Bot{
		Models: models,
		Logger: logger,
		Config: config,
		s:      dg,
	}, nil
}

func (b *Bot) AddRouter(router *gumi.Router) {
	r := gumi.Create(router)

	r.Initialize(b.s)
}

func (b *Bot) AddProvider(provider artworks.Provider) {
	b.ArtworkProviders = append(b.ArtworkProviders, provider)
}

func (b *Bot) AddHandler(handler interface{}) {
	b.s.AddHandler(handler)
}

func (b *Bot) Open() error {
	err := b.s.Open()
	if err != nil {
		return err
	}

	b.Logger.Info("Started a bot.")
	return nil
}

func (b *Bot) Close() error {
	return b.s.Close()
}
