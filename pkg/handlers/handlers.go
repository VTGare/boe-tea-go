package handlers

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/VTGare/boe-tea-go/internal/arrays"
	"github.com/VTGare/boe-tea-go/internal/cache"
	"github.com/VTGare/boe-tea-go/pkg/artworks"
	"github.com/VTGare/boe-tea-go/pkg/bot"
	"github.com/VTGare/boe-tea-go/pkg/messages"
	"github.com/VTGare/boe-tea-go/pkg/models/artworks/options"
	"github.com/VTGare/boe-tea-go/pkg/models/guilds"
	"github.com/VTGare/boe-tea-go/pkg/models/users"
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

		g, _ := b.Guilds.FindOne(ctx, m.GuildID)
		if g == nil || g.Prefix == "bt!" {
			return []string{mention, mentionExcl, "bt!", "bt ", "bt.", "bt?"}
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

		allSent := make([]*cache.MessageInfo, 0)
		if len(guild.ArtChannels) == 0 || arrays.AnyString(guild.ArtChannels, ctx.Event.ChannelID) {
			rx := xurls.Strict()
			urls := rx.FindAllString(ctx.Event.Content, -1)

			p := post.New(b, ctx, urls...)

			sent, err := p.Send()
			if err != nil {
				return err
			}

			allSent = append(allSent, sent...)

			user, err := b.Users.FindOne(context.Background(), ctx.Event.Author.ID)
			if err != nil {
				if !errors.Is(err, mongo.ErrNoDocuments) {
					return err
				}
			} else {
				if user.Crosspost {
					group, ok := user.FindGroup(ctx.Event.ChannelID)
					if ok {
						sent, _ := p.Crosspost(ctx.Event.Author.ID, group.Name, group.Children)
						allSent = append(allSent, sent...)
					}
				}
			}
		}

		if len(allSent) > 0 {
			b.EmbedCache.Set(
				ctx.Event.Author.ID,
				ctx.Event.ChannelID,
				ctx.Event.ID,
				true,
				allSent...,
			)

			for _, msg := range allSent {
				b.EmbedCache.Set(
					ctx.Event.Author.ID,
					msg.ChannelID,
					msg.MessageID,
					false,
				)
			}
		}

		return nil
	}
}

func RegisterHandlers(b *bot.Bot) {
	b.AddHandler(OnReady(b))
	b.AddHandler(OnGuildCreate(b))
	b.AddHandler(OnGuildDelete(b))
	b.AddHandler(OnGuildBanAdd(b))
	b.AddHandler(OnReactionAdd(b))
	b.AddHandler(OnReactionRemove(b))
	b.AddHandler(OnMessageRemove(b))
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

//OnGuildDelete logs guild outages and guilds that kicked the bot out.
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

func OnMessageRemove(b *bot.Bot) func(*discordgo.Session, *discordgo.MessageDelete) {
	return func(s *discordgo.Session, m *discordgo.MessageDelete) {
		msg, ok := b.EmbedCache.Get(
			m.ChannelID,
			m.ID,
		)

		if !ok {
			return
		}

		b.EmbedCache.Remove(
			m.ChannelID, m.ID,
		)

		if msg.Parent {
			b.Log.Infof(
				"Removing children messages. Channel ID: %v. Parent ID: %v. User ID: %v.",
				m.ChannelID,
				m.ID,
				msg.AuthorID,
			)

			for _, child := range msg.Children {
				b.Log.Infof(
					"Removing a child message. Parent ID: %v. Channel ID: %v. Message ID: %v. User ID: %v.",
					m.ID,
					child.ChannelID,
					child.MessageID,
					msg.AuthorID,
				)

				b.EmbedCache.Remove(
					child.ChannelID, child.MessageID,
				)

				err := s.ChannelMessageDelete(child.ChannelID, child.MessageID)
				if err != nil {
					b.Log.Warn("OnMessageRemove -> s.ChannelMessageDelete: ", err)
				}
			}
		}
	}
}

func OnReactionAdd(b *bot.Bot) func(*discordgo.Session, *discordgo.MessageReactionAdd) {
	return func(s *discordgo.Session, r *discordgo.MessageReactionAdd) {
		//Do nothing if Boe Tea adds reactions
		if r.UserID == s.State.User.ID {
			return
		}

		name := r.Emoji.APIName()
		switch {
		case name == "❌":
			msg, ok := b.EmbedCache.Get(
				r.ChannelID, r.MessageID,
			)

			if !ok {
				return
			}

			if msg.AuthorID != r.UserID {
				return
			}

			b.Log.Infof(
				"Removing a message by reacting. Channel ID: %v. Message ID: %v. User ID: %v.",
				r.ChannelID,
				r.MessageID,
				r.UserID,
			)

			b.EmbedCache.Remove(
				r.ChannelID, r.MessageID,
			)

			err := s.ChannelMessageDelete(r.ChannelID, r.MessageID)
			if err != nil {
				b.Log.Warn("OnReactionAdd -> s.ChannelMessageDelete: ", err)
			}

			if msg.Parent {
				b.Log.Infof(
					"Removing children messages. Channel ID: %v. Parent ID: %v. User ID: %v.",
					r.ChannelID,
					r.MessageID,
					r.UserID,
				)

				for _, child := range msg.Children {
					b.Log.Infof(
						"Removing a child message. Parent ID: %v. Channel ID: %v. Message ID: %v. User ID: %v.",
						r.MessageID,
						child.ChannelID,
						child.MessageID,
						r.UserID,
					)

					b.EmbedCache.Remove(
						child.ChannelID, child.MessageID,
					)

					err := s.ChannelMessageDelete(child.ChannelID, child.MessageID)
					if err != nil {
						b.Log.Warn("OnReactionAdd -> s.ChannelMessageDelete: ", err)
					}
				}
			}
		case name == "💖" || name == "🤤":
			nsfw := name == "🤤"

			msg, err := s.ChannelMessage(r.ChannelID, r.MessageID)
			if err != nil {
				b.Log.Warn("OnReactionAdd -> ChannelMessage: ", err)
				return
			}

			dgUser, err := s.User(r.UserID)
			if err != nil {
				b.Log.Warn("OnReactionRemove -> User: ", err)
				return
			}

			if dgUser.Bot {
				return
			}

			urls := make([]string, 0, 2)
			if len(msg.Embeds) > 0 {
				embed := msg.Embeds[0]
				urls = append(urls, embed.URL)
			}

			regex := xurls.Strict()
			if url := regex.FindString(msg.Content); url != "" {
				urls = append(urls, url)
			}

			var artwork artworks.Artwork
			for _, url := range urls {
				for _, provider := range b.ArtworkProviders {
					if id, ok := provider.Match(url); ok {
						artwork, err = provider.Find(id)
						if err != nil {
							b.Log.Warn("OnReactionAdd -> provider.Find: ", err)
							return
						}

						break
					}
				}

				if artwork != nil {
					break
				}
			}

			if artwork == nil {
				return
			}

			insert := artwork.ToModel()
			if len(insert.Images) == 0 {
				return
			}

			artworkDB, created, err := b.Artworks.FindOneOrCreate(context.Background(), &options.FilterOne{
				URL: artwork.URL(),
			}, insert)

			if created {
				b.Log.Infof(
					"Created a new artwork. ID: %v. URL: %v. Images: %v",
					artworkDB.ID,
					artworkDB.URL,
					len(artworkDB.Images),
				)
			}

			if err != nil {
				b.Log.Warn("OnReactionAdd -> Artworks.FindOneOrCreate: ", err)
				return
			}

			user, err := b.Users.FindOneOrCreate(context.Background(), r.UserID)
			if err != nil {
				b.Log.Warn("OnReactionAdd -> Users.FindOneOrCreate: ", err)
				return
			}

			b.Log.Infof("Inserting a favourite. User ID: %v. Artwork ID: %v", r.UserID, artworkDB.ID)
			_, err = b.Users.InsertFavourite(context.Background(), r.UserID, &users.Favourite{
				ArtworkID: artworkDB.ID,
				NSFW:      nsfw,
				CreatedAt: time.Now(),
			})

			if err != nil {
				switch {
				case errors.Is(err, mongo.ErrNoDocuments):
				default:
					b.Log.Warn("OnReactionAdd -> InsertFavourite: ", err)
					return
				}
			}

			if user.DM {
				ch, err := s.UserChannelCreate(user.ID)
				if err == nil {
					var (
						eb     = embeds.NewBuilder()
						locale = messages.UserFavouriteAdded()
					)

					eb.Title(locale.Title).Description(locale.Description)
					eb.AddField(
						"ID",
						strconv.Itoa(artworkDB.ID),
						true,
					).AddField(
						"URL",
						messages.ClickHere(artworkDB.URL),
						true,
					).AddField(
						"NSFW",
						strconv.FormatBool(nsfw),
						true,
					)
					if len(artworkDB.Images) > 0 {
						eb.Thumbnail(artworkDB.Images[0])
					}

					s.ChannelMessageSendEmbed(ch.ID, eb.Finalize())
				}
			}
		}
	}
}

func OnReactionRemove(b *bot.Bot) func(*discordgo.Session, *discordgo.MessageReactionRemove) {
	return func(s *discordgo.Session, r *discordgo.MessageReactionRemove) {
		//Do nothing if Boe Tea adds reactions
		if r.UserID == s.State.User.ID {
			return
		}

		//Do nothing if user was banned recently. Discord removes all reactions
		//of banned users on the server which in turn removes all favourites.
		if _, ok := b.BannedUsers.Get(r.UserID); ok {
			return
		}

		if r.Emoji.APIName() == "💖" || r.Emoji.APIName() == "🤤" {
			msg, err := s.ChannelMessage(r.ChannelID, r.MessageID)
			if err != nil {
				b.Log.Warn("OnReactionRemove -> ChannelMessage: ", err)
				return
			}

			dgUser, err := s.User(r.UserID)
			if err != nil {
				b.Log.Warn("OnReactionRemove -> User: ", err)
				return
			}

			if dgUser.Bot {
				return
			}

			urls := make([]string, 0, 2)
			if len(msg.Embeds) > 0 {
				embed := msg.Embeds[0]
				urls = append(urls, embed.URL)
			}

			regex := xurls.Strict()
			if url := regex.FindString(msg.Content); url != "" {
				urls = append(urls, url)
			}

			var artwork artworks.Artwork
			for _, url := range urls {
				for _, provider := range b.ArtworkProviders {
					if id, ok := provider.Match(url); ok {
						artwork, err = provider.Find(id)
						if err != nil {
							b.Log.Warn("OnReactionRemove -> provider.Find: ", err)
							return
						}

						break
					}
				}

				if artwork != nil {
					break
				}
			}

			if artwork == nil {
				return
			}

			artworkDB, err := b.Artworks.FindOne(context.Background(), &options.FilterOne{
				URL: artwork.URL(),
			})

			if err != nil {
				if !errors.Is(err, mongo.ErrNoDocuments) {
					b.Log.Warn("OnReactionRemove -> Artworks.FindOneOrCreate: ", err)
				}
				return
			}

			user, err := b.Users.FindOne(context.Background(), r.UserID)
			if err != nil {
				if !errors.Is(err, mongo.ErrNoDocuments) {
					b.Log.Warn("OnReactionRemove -> Users.FindOneOrCreate: ", err)
				}

				return
			}

			if fav, ok := user.FindFavourite(artworkDB.ID); ok {
				b.Log.Infof("Removing a favourite. User ID: %v. Artwork ID: %v", r.UserID, artworkDB.ID)
				_, err := b.Users.DeleteFavourite(
					context.Background(),
					user.ID,
					fav,
				)

				if err != nil {
					b.Log.Warn("OnReactionRemove -> Users.DeleteFavourite: ", err)
					return
				}

				if user.DM {
					ch, err := s.UserChannelCreate(user.ID)
					if err == nil {
						var (
							eb     = embeds.NewBuilder()
							locale = messages.UserFavouriteRemoved()
						)

						eb.Title(locale.Title).Description(locale.Description)
						eb.AddField(
							"ID",
							strconv.Itoa(artworkDB.ID),
							true,
						).AddField(
							"URL",
							messages.ClickHere(artworkDB.URL),
							true,
						).AddField(
							"NSFW",
							strconv.FormatBool(fav.NSFW),
							true,
						)
						if len(artworkDB.Images) > 0 {
							eb.Thumbnail(artworkDB.Images[0])
						}

						s.ChannelMessageSendEmbed(ch.ID, eb.Finalize())
					}
				}
			}
		}
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

		b.Metrics.IncrementCommand()
		return nil
	}
}
