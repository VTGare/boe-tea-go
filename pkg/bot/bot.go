package bot

import (
	"context"
	"time"

	"github.com/VTGare/boe-tea-go/internal/arrays"
	"github.com/VTGare/boe-tea-go/internal/config"
	"github.com/VTGare/boe-tea-go/pkg/artworks"
	"github.com/VTGare/boe-tea-go/pkg/models"
	"github.com/VTGare/gumi"
	"github.com/bwmarrin/discordgo"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"mvdan.cc/xurls/v2"
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

func (b *Bot) AddRouter() {
	r := gumi.Create(&gumi.Router{
		PrefixResolver: func(s *discordgo.Session, m *discordgo.MessageCreate) []string {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			g, err := b.Models.Guilds.FindOne(ctx, m.GuildID)
			if err != nil || arrays.AnyString(b.Config.Discord.Prefixes, g.Prefix) {
				return b.Config.Discord.Prefixes
			}

			return []string{g.Prefix}
		},
		NotCommandCallback: func(ctx *gumi.Ctx) error {
			if ctx.Event.GuildID != "733665753793953862" {
				return nil
			}

			rx := xurls.Strict()
			wg, _ := errgroup.WithContext(context.Background())
			urls := rx.FindAllString(ctx.Event.Content, -1)
			artworks := make([]artworks.Artwork, len(urls))

			for i, url := range urls {
				i := i
				url := url

				wg.Go(func() error {
					for _, provider := range b.ArtworkProviders {
						if id, ok := provider.Match(url); ok {
							artwork, err := provider.Find(id)
							if err != nil {
								return err
							}

							artworks[i] = artwork
							break
						}
					}

					return nil
				})
			}

			if err := wg.Wait(); err != nil {
				return err
			}

			for _, artwork := range artworks {
				if artwork != nil {
					embeds := artwork.Embeds("Test")
					for _, embed := range embeds {
						ctx.ReplyEmbed(embed)
					}
				}
			}

			return nil
		},
	})

	r.Initialize(b.s)
}

func (b *Bot) AddProvider(provider artworks.Provider) {
	b.ArtworkProviders = append(b.ArtworkProviders, provider)
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
