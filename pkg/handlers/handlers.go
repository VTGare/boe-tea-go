package handlers

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"time"

	"github.com/VTGare/boe-tea-go/internal/arrays"
	"github.com/VTGare/boe-tea-go/pkg/artworks"
	"github.com/VTGare/boe-tea-go/pkg/artworks/pixiv"
	"github.com/VTGare/boe-tea-go/pkg/artworks/twitter"
	"github.com/VTGare/boe-tea-go/pkg/bot"
	"github.com/VTGare/boe-tea-go/pkg/messages"
	"github.com/VTGare/boe-tea-go/pkg/models/guilds"
	"github.com/VTGare/boe-tea-go/pkg/repost"
	"github.com/VTGare/embeds"
	"github.com/VTGare/gumi"
	"github.com/bwmarrin/discordgo"
	"go.mongodb.org/mongo-driver/mongo"
	"golang.org/x/sync/errgroup"
	"mvdan.cc/xurls/v2"
)

//PrefixResolver returns an array of guild's prefixes and bot mentions.
func PrefixResolver(b *bot.Bot) func(s *discordgo.Session, m *discordgo.MessageCreate) []string {
	return func(s *discordgo.Session, m *discordgo.MessageCreate) []string {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		mention := fmt.Sprintf("<@%v> ", s.State.User.ID)
		mentionExcl := fmt.Sprintf("<@!%v> ", s.State.User.ID)

		g, err := b.Models.Guilds.FindOne(ctx, m.GuildID)
		if err != nil || arrays.AnyString(b.Config.Discord.Prefixes, g.Prefix) {
			return append(b.Config.Discord.Prefixes, mention, mentionExcl)
		}

		return []string{mention, mentionExcl, g.Prefix}
	}
}

//NotCommand is executed on every message that isn't a command.
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
			reposts := make(chan *repost.Repost, len(urls))

			for _, url := range urls {
				url := url //shadowing loop variables to pass them to wg.Go. It's required otherwise variables will stay the same every loop.
				wg.Go(func() error {
					for _, provider := range b.ArtworkProviders {
						if !dm {
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
						}

						if id, ok := provider.Match(url); ok {
							if !dm {
								if rep, _ := b.RepostDetector.Find(ctx.Event.ChannelID, id); rep != nil {
									reposts <- rep
									break
								}
							}

							artwork, err := provider.Find(id)
							if err != nil {
								return err
							}

							artworks <- artwork
							if !dm {
								err = b.RepostDetector.Create(&repost.Repost{
									ID:        id,
									URL:       url,
									GuildID:   ctx.Event.GuildID,
									ChannelID: ctx.Event.ChannelID,
									MessageID: ctx.Event.ID,
								}, 24*time.Hour)
							}

							if err != nil {
								b.Logger.Errorf("Error adding a repost detector: %v", err)
							}

							break
						}
					}

					return nil
				})
			}

			if err := wg.Wait(); err != nil {
				return err
			}

			close(artworks)
			close(reposts)

			for rep := range reposts {
				//TODO: handle reposts
				b.Logger.Info("Repost detected: %v", rep)
			}

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

//OnReady logs that bot's up.
func OnReady(b *bot.Bot) func(*discordgo.Session, *discordgo.Ready) {
	return func(s *discordgo.Session, r *discordgo.Ready) {
		b.Logger.Infof("%v is online. Session ID: %v. Guilds: %v", r.User.String(), r.SessionID, len(r.Guilds))
	}
}

//OnGuildCreate loads server configuration on launch and creates new database entries when joining a new server.
func OnGuildCreate(b *bot.Bot) func(*discordgo.Session, *discordgo.GuildCreate) {
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

//OnGuilldDelete logs guild outages and guilds that kicked the bot out.
func OnGuildDelete(b *bot.Bot) func(*discordgo.Session, *discordgo.GuildDelete) {
	return func(s *discordgo.Session, g *discordgo.GuildDelete) {
		if g.Unavailable {
			b.Logger.Infof("Guild outage. ID: %v", g.ID)
		} else {
			b.Logger.Infof("Kicked/banned from guild: %v", g.ID)
		}
	}
}

//OnGuildBanAdd adds a banned server member to temporary banned users cache to prevent them from losing all their favourites
//on that server due to Discord removing all reactions of banned users.
func OnGuildBanAdd(b *bot.Bot) func(*discordgo.Session, *discordgo.GuildBanAdd) {
	return func(s *discordgo.Session, gb *discordgo.GuildBanAdd) {
		b.BannedUsers.Set(gb.User.ID, struct{}{})
	}
}

//OnError creates an error response, logs them and sends the response on Discord.
func OnError(b *bot.Bot) func(*gumi.Ctx, error) {
	return func(ctx *gumi.Ctx, err error) {
		eb := embeds.NewBuilder()

		var (
			cmd *messages.IncorrectCmd
		)

		switch {
		case errors.As(err, &cmd):
			eb.ErrorTemplate(cmd.Error())
			eb.AddField("Usage", fmt.Sprintf("`%v`", cmd.Usage), true)
			eb.AddField("Example", fmt.Sprintf("`%v`", cmd.Example), true)
		default:
			eb.ErrorTemplate(err.Error())
		}

		ctx.ReplyEmbed(eb.Finalize())
	}
}

//OnRateLimit creates a response for users who use bot's command too frequently
func OnRateLimit(b *bot.Bot) func(*gumi.Ctx) error {
	return func(ctx *gumi.Ctx) error {
		duration, err := ctx.Command.RateLimiter.Expires(ctx.Event.Author.ID)
		if err != nil {
			return err
		}

		eb := embeds.NewBuilder()
		eb.FailureTemplate(fmt.Sprintf("Hold your horses! You're getting rate limited. Try again in **%v**", duration.Round(1*time.Second).String()))

		return ctx.ReplyEmbed(eb.Finalize())
	}
}

//OnNoPerms creates a response for users who used a command without required permissions.
func OnNoPerms(b *bot.Bot) func(ctx *gumi.Ctx) error {
	return func(ctx *gumi.Ctx) error {
		eb := embeds.NewBuilder()
		eb.FailureTemplate("You don't have enough permissions to run this command.")

		return ctx.ReplyEmbed(eb.Finalize())
	}
}

//OnNSFW creates a response for users who used a NSFW command in a SFW channel
func OnNSFW(b *bot.Bot) func(ctx *gumi.Ctx) error {
	return func(ctx *gumi.Ctx) error {
		eb := embeds.NewBuilder()

		eb.FailureTemplate(fmt.Sprintf("Bonk! You're trying to execute a NSFW command `%v` in a SFW channel.", ctx.Command.Name))

		return ctx.ReplyEmbed(eb.Finalize())
	}
}

//OnExecute logs every executed command.
func OnExecute(b *bot.Bot) func(ctx *gumi.Ctx) error {
	return func(ctx *gumi.Ctx) error {
		b.Logger.Infof("Executing command [%v]. Arguments: [%v]. Guild ID: %v, channel ID: %v", ctx.Command.Name, ctx.Args.Raw, ctx.Event.GuildID, ctx.Event.ChannelID)
		return nil
	}
}
