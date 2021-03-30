package handlers

import (
	"context"
	"errors"
	"math/rand"
	"time"

	"github.com/VTGare/boe-tea-go/internal/arrays"
	"github.com/VTGare/boe-tea-go/pkg/artworks"
	"github.com/VTGare/boe-tea-go/pkg/artworks/pixiv"
	"github.com/VTGare/boe-tea-go/pkg/artworks/twitter"
	"github.com/VTGare/boe-tea-go/pkg/bot"
	"github.com/VTGare/boe-tea-go/pkg/models/guilds"
	"github.com/VTGare/gumi"
	"github.com/bwmarrin/discordgo"
	"go.mongodb.org/mongo-driver/mongo"
	"golang.org/x/sync/errgroup"
	"mvdan.cc/xurls/v2"
)

func PrefixResolver(b *bot.Bot) func(s *discordgo.Session, m *discordgo.MessageCreate) []string {
	return func(s *discordgo.Session, m *discordgo.MessageCreate) []string {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		g, err := b.Models.Guilds.FindOne(ctx, m.GuildID)
		if err != nil || arrays.AnyString(b.Config.Discord.Prefixes, g.Prefix) {
			return b.Config.Discord.Prefixes
		}

		return []string{g.Prefix}
	}
}

func NotCommand(b *bot.Bot) func(*gumi.Ctx) error {
	return func(ctx *gumi.Ctx) error {
		var (
			dm    = ctx.Event.GuildID == ""
			guild *guilds.Guild
		)

		if dm {
			guild = guilds.DefaultGuild("")
		} else {
			var err error
			guild, err = b.Models.Guilds.FindOne(context.Background(), ctx.Event.GuildID)
			if err != nil {
				return err
			}
		}

		if len(guild.ArtChannels) == 0 || arrays.AnyString(guild.ArtChannels, ctx.Event.ChannelID) {
			rx := xurls.Strict()
			urls := rx.FindAllString(ctx.Event.Content, -1)

			wg, _ := errgroup.WithContext(context.Background())
			artworks := make(chan artworks.Artwork, len(urls))

			for _, url := range urls {
				url := url //shadowing loop variables to pass them to wg.Go. It's required otherwise variables will stay the same every loop.

				wg.Go(func() error {
					for _, provider := range b.ArtworkProviders {
						switch provider.(type) {
						case twitter.Twitter:
							if !guild.Twitter {
								continue
							}
						case pixiv.Pixiv:
							if !guild.Pixiv {
								continue
							}
						}

						if id, ok := provider.Match(url); ok {
							artwork, err := provider.Find(id)
							if err != nil {
								return err
							}

							artworks <- artwork
							break
						}
					}

					return nil
				})
			}

			if err := wg.Wait(); err != nil {
				return err
			}

			//close(artworks)
			for artwork := range artworks {
				if artwork != nil {
					s := rand.NewSource(time.Now().Unix())
					r := rand.New(s)

					var quote string
					if l := len(b.Config.Quotes); l > 0 {
						quote = b.Config.Quotes[r.Intn(l)].Content
					}

					embeds := artwork.Embeds(quote)
					for _, embed := range embeds {
						ctx.ReplyEmbed(embed)
					}
				}
			}
		}

		return nil
	}
}

func OnReady(b *bot.Bot) func(*discordgo.Session, *discordgo.Ready) {
	return func(s *discordgo.Session, r *discordgo.Ready) {
		b.Logger.Infof("%v is online. Session ID: %v. Guilds: %v", r.User.String(), r.SessionID, len(r.Guilds))
	}
}

func GuildCreated(b *bot.Bot) func(*discordgo.Session, *discordgo.GuildCreate) {
	return func(s *discordgo.Session, g *discordgo.GuildCreate) {
		_, err := b.Models.Guilds.FindOne(context.Background(), g.ID)
		if errors.Is(err, mongo.ErrNoDocuments) {
			b.Logger.Infof("Joined a guild. Name: %v. ID: %v", g.Name, g.ID)
			_, err := b.Models.Guilds.InsertOne(context.Background(), g.ID)
			if err != nil {
				b.Logger.Errorf("Error while inserting guild %v: %v", g.ID, err)
			}
		}
	}
}
