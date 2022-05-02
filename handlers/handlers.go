package handlers

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/VTGare/boe-tea-go/artworks"
	"github.com/VTGare/boe-tea-go/bot"
	"github.com/VTGare/boe-tea-go/internal/arrays"
	"github.com/VTGare/boe-tea-go/internal/cache"
	"github.com/VTGare/boe-tea-go/messages"
	"github.com/VTGare/boe-tea-go/post"
	"github.com/VTGare/boe-tea-go/store"
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

		g, _ := b.Store.Guild(ctx, m.GuildID)
		if g == nil || g.Prefix == "bt!" {
			return []string{mention, mentionExcl, "bt!", "bt ", "bt.", "bt?"}
		}

		return []string{mention, mentionExcl, g.Prefix}
	}
}

func OnPanic(b *bot.Bot) func(*gumi.Ctx, interface{}) {
	return func(ctx *gumi.Ctx, r interface{}) {
		b.Log.Errorf("%v", r)
	}
}

//NotCommand is executed on every message that isn't a command.
func NotCommand(b *bot.Bot) func(*gumi.Ctx) error {
	return func(ctx *gumi.Ctx) error {
		guild, err := b.Store.Guild(context.Background(), ctx.Event.GuildID)
		if err != nil {
			return err
		}

		allSent := make([]*cache.MessageInfo, 0)
		if len(guild.ArtChannels) == 0 || arrays.Any(guild.ArtChannels, ctx.Event.ChannelID) {
			rx := xurls.Strict()
			urls := rx.FindAllString(ctx.Event.Content, -1)

			if len(urls) == 0 {
				return nil
			}

			p := post.New(b, ctx, urls...)

			sent, err := p.Send()
			if err != nil {
				return err
			}

			allSent = append(allSent, sent...)
			user, err := b.Store.User(context.Background(), ctx.Event.Author.ID)
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
		_, err := b.Store.Guild(context.Background(), g.ID)
		if errors.Is(err, mongo.ErrNoDocuments) {
			b.Log.Infof("Joined a guild. Name: %v. ID: %v", g.Name, g.ID)
			_, err := b.Store.CreateGuild(context.Background(), g.ID)
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
		//Do nothing for bot's own reactions
		if r.UserID == s.State.User.ID {
			return
		}

		var (
			log = b.Log.With(
				"guild", r.GuildID,
				"channel", r.ChannelID,
				"message", r.MessageID,
				"user", r.UserID,
			)
			name        = r.Emoji.APIName()
			ctx, cancel = context.WithTimeout(context.Background(), 30*time.Second)
		)

		defer cancel()

		deleteEmbed := func() error {
			msg, ok := b.EmbedCache.Get(r.ChannelID, r.MessageID)
			if !ok {
				return nil
			}

			if msg.AuthorID != r.UserID {
				return nil
			}

			log.Infof("deleting a message from reaction event")
			b.EmbedCache.Remove(r.ChannelID, r.MessageID)

			err := s.ChannelMessageDelete(r.ChannelID, r.MessageID)
			if err != nil {
				return err
			}

			if !msg.Parent {
				return nil
			}

			log.Infof("removing children messages")
			childrenIDs := make(map[string][]string)
			for _, child := range msg.Children {
				log.With(
					"parent", r.MessageID,
					"channel", child.ChannelID,
					"message", child.MessageID,
					"user", r.UserID,
				).Infof("removing a child message")

				b.EmbedCache.Remove(child.ChannelID, child.MessageID)

				if _, ok := childrenIDs[child.ChannelID]; !ok {
					childrenIDs[child.ChannelID] = make([]string, 0)
				}

				childrenIDs[child.ChannelID] = append(childrenIDs[child.ChannelID], child.MessageID)
			}

			for channelID, messageIDs := range childrenIDs {
				if err := s.ChannelMessagesBulkDelete(channelID, messageIDs); err != nil {
					log.With("error", err).Warn("failed to delete children messages")
				}
			}

			return nil
		}

		crosspost := func() error {
			msg, err := s.ChannelMessage(r.ChannelID, r.MessageID)
			if err != nil {
				return err
			}

			dgUser, err := s.User(r.UserID)
			if err != nil {
				return err
			}

			if dgUser.Bot {
				return nil
			}

			url := ""
			if len(msg.Embeds) > 0 {
				embed := msg.Embeds[0]
				url = embed.URL
			}

			regex := xurls.Strict()
			if url == "" {
				url = regex.FindString(msg.Content)
			}

			if url == "" {
				return nil
			}

			msg.Author = dgUser
			gumiCtx := &gumi.Ctx{
				Session: s,
				Event: &discordgo.MessageCreate{
					Message: msg,
				},
				Router: b.Router,
			}

			p := post.New(b, gumiCtx, url)
			sent := make([]*cache.MessageInfo, 0)
			user, _ := b.Store.User(ctx, r.UserID)
			if user != nil {
				if group, ok := user.FindGroup(r.ChannelID); ok {
					sent, err = p.Crosspost(user.ID, group.Name, group.Children)
					if err != nil {
						return err
					}
				}
			}

			if len(sent) > 0 {
				b.EmbedCache.Set(r.UserID, r.ChannelID, r.MessageID, true, sent...)
				for _, msg := range sent {
					b.EmbedCache.Set(r.UserID, msg.ChannelID, msg.MessageID, false)
				}
			}

			return nil
		}

		addFavourite := func() error {
			msg, err := s.ChannelMessage(r.ChannelID, r.MessageID)
			if err != nil {
				return fmt.Errorf("failed to get a discord message: %w", err)
			}

			dgUser, err := s.User(r.UserID)
			if err != nil {
				return fmt.Errorf("failed to get a discord user: %w", err)
			}

			if dgUser.Bot {
				return nil
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
							return fmt.Errorf("failed to find an artwork: %w", err)
						}

						break
					}
				}

				if artwork != nil {
					break
				}
			}

			if artwork == nil {
				return nil
			}

			if artwork.Len() == 0 {
				return nil
			}

			artworkDB, err := b.Store.Artwork(ctx, 0, artwork.URL())
			if errors.Is(err, store.ErrArtworkNotFound) {
				artworkDB, err = b.Store.CreateArtwork(ctx, artwork.StoreArtwork())
			}

			if err != nil {
				return fmt.Errorf("failed to find or create an artwork: %w", err)
			}

			var (
				nsfw = name == "🤤"
				fav  = &store.Bookmark{
					UserID:    r.UserID,
					ArtworkID: artworkDB.ID,
					NSFW:      nsfw,
					CreatedAt: time.Now(),
				}
				log = log.With(
					"user", r.UserID,
					"artwork_id", artworkDB.ID,
					"nsfw", nsfw,
				)
			)

			log.Info("inserting a favourite")
			added, err := b.Store.AddBookmark(ctx, fav)
			if err != nil {
				return fmt.Errorf("failed to insert a favourite: %w", err)
			}

			if !added {
				return nil
			}

			user, err := b.Store.User(ctx, r.UserID)
			if err != nil {
				return fmt.Errorf("failed to find or create a user: %w", err)
			}

			if !user.DM {
				return nil
			}

			dmSession := b.ShardManager.SessionForDM()
			ch, err := dmSession.UserChannelCreate(user.ID)
			if err != nil {
				return fmt.Errorf("failed to create private channel: %w", err)
			}

			eb := embeds.NewBuilder()
			if len(artworkDB.Images) > 0 {
				eb.Thumbnail(artworkDB.Images[0])
			}

			eb.Title("💖 Successfully added an artwork to favourites").
				Description("If you dislike direct messages disable them by running `bt!userset dm off` command").
				AddField("ID", strconv.Itoa(artworkDB.ID), true).
				AddField("URL", messages.ClickHere(artworkDB.URL), true).
				AddField("NSFW", strconv.FormatBool(nsfw), true)

			dmSession.ChannelMessageSendEmbed(ch.ID, eb.Finalize())
			return nil
		}

		switch {
		case name == "❌":
			if err := deleteEmbed(); err != nil {
				log.With("error", err).Error("failed to delete an embed on reaction")
			}
		case name == "💖" || name == "🤤":
			if err := addFavourite(); err != nil {
				log.With("error", err).Error("failed to add favourite")
			}

		case name == "📫" || name == "📩":
			if err := crosspost(); err != nil {
				log.With("error", err).Error("failed to crosspost artwork on reaction")
			}
		}
	}
}

func OnReactionRemove(b *bot.Bot) func(*discordgo.Session, *discordgo.MessageReactionRemove) {
	return func(s *discordgo.Session, r *discordgo.MessageReactionRemove) {
		//Do nothing for bot's own reactions
		if r.UserID == s.State.User.ID {
			return
		}

		log := b.Log.With(
			"guild", r.GuildID,
			"channel", r.ChannelID,
			"message", r.MessageID,
			"user", r.UserID,
		)

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		//Do nothing if user was banned recently. Discord removes all reactions
		//of banned users on the server which in turn removes all favourites.
		if _, ok := b.BannedUsers.Get(r.UserID); ok {
			return
		}

		if r.Emoji.APIName() != "💖" && r.Emoji.APIName() != "🤤" {
			return
		}

		msg, err := s.ChannelMessage(r.ChannelID, r.MessageID)
		if err != nil {
			log.With("error", err).Error("failed to get discord message")
			return
		}

		dgUser, err := s.User(r.UserID)
		if err != nil {
			log.With("error", err).Error("failed to get discord user")
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
						log.With("error", err).Error("failed to find an artwork")
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

		artworkDB, err := b.Store.Artwork(ctx, 0, artwork.URL())
		if err != nil {
			if !errors.Is(err, mongo.ErrNoDocuments) {
				log.With("error", err).Error("failed to find an artwork")
			}

			return
		}

		log.With("user", r.UserID, "artwork", artworkDB.ID).Info("removing a favourite")
		deleted, err := b.Store.DeleteBookmark(ctx, &store.Bookmark{UserID: r.UserID, ArtworkID: artworkDB.ID})
		if err != nil {
			log.With("error", err).Error("failed to remove a favourite")
			return
		}

		if !deleted {
			return
		}

		user, err := b.Store.User(ctx, r.UserID)
		if err != nil {
			log.With("error", err).Error("failed to find or create a user")
			return
		}

		if !user.DM {
			return
		}

		dmSession := b.ShardManager.SessionForDM()
		ch, err := dmSession.UserChannelCreate(user.ID)
		if err != nil {
			log.With("error", err).Error("failed to create private channel")
			return
		}

		eb := embeds.NewBuilder()
		eb.Title("💔 Successfully removed an artwork from favourites").
			Description("If you dislike direct messages disable them by running `bt!userset dm off` command").
			AddField("ID", strconv.Itoa(artworkDB.ID), true).
			AddField("URL", messages.ClickHere(artworkDB.URL), true)

		if len(artworkDB.Images) > 0 {
			eb.Thumbnail(artworkDB.Images[0])
		}

		dmSession.ChannelMessageSendEmbed(ch.ID, eb.Finalize())
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
			eb.FailureTemplate(cmdErr.Error() + "\n" + cmdErr.Description)
			eb.AddField(cmdErr.Embed.Usage, fmt.Sprintf("`%v`", cmdErr.Usage))
			eb.AddField(cmdErr.Embed.Example, fmt.Sprintf("`%v`", cmdErr.Example))
		case errors.As(err, &usrErr):
			if err := usrErr.Unwrap(); err != nil {
				if ctx.Command != nil {
					b.Log.Infof("A user error occured. Command: %v. Arguments: [%v]. Error: %v", ctx.Command.Name, ctx.Args.Raw, err)
				} else {
					b.Log.Infof("A user error occured. Error: %v", err)
				}
			}

			eb.FailureTemplate(usrErr.Error())
		default:
			if ctx.Command != nil {
				b.Log.Errorf("An error occured. Command: %v. Arguments: [%v]. Error: %v", ctx.Command.Name, ctx.Args.Raw, err)
			} else {
				b.Log.Errorf("An error occured. Error: %v", err)
			}

			eb.FailureTemplate("An unexpected error occured. Please try again later.\n" +
				"If error persists, please let the developer know about it with `bt!feedback` command.")
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

		b.Stats.IncrementCommand(ctx.Command.Name)
		return nil
	}
}
