package handlers

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/VTGare/boe-tea-go/internal/arrays"
	"github.com/VTGare/boe-tea-go/pkg/bot"
	"github.com/VTGare/boe-tea-go/pkg/messages"
	"github.com/VTGare/boe-tea-go/pkg/models/guilds"
	"github.com/VTGare/boe-tea-go/pkg/post"
	"github.com/VTGare/embeds"
	"github.com/VTGare/gumi"
	"github.com/bwmarrin/discordgo"
	"go.mongodb.org/mongo-driver/mongo"
	"mvdan.cc/xurls/v2"
)

//PrefixResolver returns an array of guild's prefixes and bot mentions.
func PrefixResolver(b *bot.Bot) func(s *discordgo.Session, m *discordgo.MessageCreate) []string {
	return func(s *discordgo.Session, m *discordgo.MessageCreate) []string {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		mention := fmt.Sprintf("<@%v> ", s.State.User.ID)
		mentionExcl := fmt.Sprintf("<@!%v> ", s.State.User.ID)

		g, err := b.Guilds.FindOne(ctx, m.GuildID)
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
			guild, err = b.Guilds.FindOne(context.Background(), ctx.Event.GuildID)
			if err != nil {
				return err
			}
		}

		if len(guild.ArtChannels) == 0 || arrays.AnyString(guild.ArtChannels, ctx.Event.ChannelID) {
			rx := xurls.Strict()
			urls := rx.FindAllString(ctx.Event.Content, -1)

			p := post.New(b, ctx, urls...)
			err := p.Send()
			if err != nil {
				return err
			}

			user, err := b.Users.FindOne(context.Background(), ctx.Event.Author.ID)
			if err != nil {
				if !errors.Is(err, mongo.ErrNoDocuments) {
					return err
				}
			} else {
				if user.Crosspost {
					group, ok := user.FindGroup(ctx.Event.ChannelID)
					if ok {
						p.Crosspost(ctx.Event.Author.ID, group.Name, group.Children)
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
		b.Log.Infof("%v is online. Session ID: %v. Guilds: %v", r.User.String(), r.SessionID, len(r.Guilds))
	}
}

//OnGuildCreate loads server configuration on launch and creates new database entries when joining a new server.
func OnGuildCreate(b *bot.Bot) func(*discordgo.Session, *discordgo.GuildCreate) {
	return func(s *discordgo.Session, g *discordgo.GuildCreate) {
		_, err := b.Guilds.FindOne(context.Background(), g.ID)
		if errors.Is(err, mongo.ErrNoDocuments) {
			b.Log.Infof("Joined a guild. Name: %v. ID: %v", g.Name, g.ID)
			_, err := b.Guilds.InsertOne(context.Background(), g.ID)
			if err != nil {
				b.Log.Errorf("Error while inserting guild %v: %v", g.ID, err)
			}
		}
	}
}

//OnGuilldDelete logs guild outages and guilds that kicked the bot out.
func OnGuildDelete(b *bot.Bot) func(*discordgo.Session, *discordgo.GuildDelete) {
	return func(s *discordgo.Session, g *discordgo.GuildDelete) {
		if g.Unavailable {
			b.Log.Infof("Guild outage. ID: %v", g.ID)
		} else {
			b.Log.Infof("Kicked/banned from guild: %v", g.ID)
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
			cmdErr *messages.IncorrectCmd
			usrErr *messages.UserErr
		)

		switch {
		case errors.As(err, &cmdErr):
			eb.FailureTemplate(cmdErr.Error())
			eb.AddField(cmdErr.Embed.Usage, fmt.Sprintf("`%v`", cmdErr.Usage), true)
			eb.AddField(cmdErr.Embed.Example, fmt.Sprintf("`%v`", cmdErr.Example), true)
		case errors.As(err, &usrErr):
			if err := usrErr.Unwrap(); err != nil {
				if ctx.Command != nil {
					b.Log.Errorf("An error occured. Command: %v. Arguments: [%v]. Error: %v", ctx.Command.Name, ctx.Args.Raw, err)
				} else {
					b.Log.Errorf("An error occured. Error: %v", err)
				}
			}

			eb.FailureTemplate(usrErr.Error())
		default:
			if ctx.Command != nil {
				b.Log.Errorf("An error occured. Command: %v. Arguments: [%v]. Error: %v", ctx.Command.Name, ctx.Args.Raw, err)
			} else {
				b.Log.Errorf("An error occured. Error: %v", err)
			}

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
		eb.FailureTemplate(messages.RateLimit(duration))

		return ctx.ReplyEmbed(eb.Finalize())
	}
}

//OnNoPerms creates a response for users who used a command without required permissions.
func OnNoPerms(b *bot.Bot) func(ctx *gumi.Ctx) error {
	return func(ctx *gumi.Ctx) error {
		eb := embeds.NewBuilder()
		eb.FailureTemplate(messages.NoPerms())

		return ctx.ReplyEmbed(eb.Finalize())
	}
}

//OnNSFW creates a response for users who used a NSFW command in a SFW channel
func OnNSFW(b *bot.Bot) func(ctx *gumi.Ctx) error {
	return func(ctx *gumi.Ctx) error {
		eb := embeds.NewBuilder()

		eb.FailureTemplate(messages.NSFWCommand(ctx.Command.Name))

		return ctx.ReplyEmbed(eb.Finalize())
	}
}

//OnExecute logs every executed command.
func OnExecute(b *bot.Bot) func(ctx *gumi.Ctx) error {
	return func(ctx *gumi.Ctx) error {
		b.Log.Infof("Executing command [%v]. Arguments: [%v]. Guild ID: %v, channel ID: %v", ctx.Command.Name, ctx.Args.Raw, ctx.Event.GuildID, ctx.Event.ChannelID)
		return nil
	}
}
