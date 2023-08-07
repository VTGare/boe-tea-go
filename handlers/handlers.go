package handlers

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/VTGare/boe-tea-go/artworks"
	"github.com/VTGare/boe-tea-go/artworks/twitter"
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

// PrefixResolver returns an array of guild's prefixes and bot mentions.
func PrefixResolver(b *bot.Bot) func(s *discordgo.Session, m *discordgo.MessageCreate) []string {
	return func(s *discordgo.Session, m *discordgo.MessageCreate) []string {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
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

// NotCommand is executed on every message that isn't a command.
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
						sent, _ := p.Crosspost(ctx.Event.Author.ID, group)
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

// OnReady logs that bot's up.
func OnReady(b *bot.Bot) func(*discordgo.Session, *discordgo.Ready) {
	return func(s *discordgo.Session, r *discordgo.Ready) {
		b.Log.With("user", r.User.String(), "session_id", r.SessionID, "guilds", len(r.Guilds)).Info("shard is connected")
	}
}

// OnGuildCreate loads server configuration on launch and creates new database entries when joining a new server.
func OnGuildCreate(b *bot.Bot) func(*discordgo.Session, *discordgo.GuildCreate) {
	return func(s *discordgo.Session, g *discordgo.GuildCreate) {
		_, err := b.Store.Guild(context.Background(), g.ID)
		if errors.Is(err, mongo.ErrNoDocuments) {
			b.Log.With("guild", g.Name, "guild_id", g.ID).Info("joined a guild")
			_, err := b.Store.CreateGuild(context.Background(), g.ID)
			if err != nil {
				b.Log.With("error", err, "guild_id", g.ID).Error("failed to insert guild")
			}
		}
	}
}

// OnGuildDelete logs guild outages and guilds that kicked the bot out.
func OnGuildDelete(b *bot.Bot) func(*discordgo.Session, *discordgo.GuildDelete) {
	return func(s *discordgo.Session, g *discordgo.GuildDelete) {
		var log = b.Log.With(
			"guild_id", g.ID,
		)

		if g.Unavailable {
			log.Info("guild outage")
		} else {
			log.Info("bot kicked/banned from guild")
		}
	}
}

// OnGuildBanAdd adds a banned server member to temporary banned users cache to prevent them from losing all their bookmarks
// on that server due to Discord removing all reactions of banned users.
func OnGuildBanAdd(b *bot.Bot) func(*discordgo.Session, *discordgo.GuildBanAdd) {
	return func(s *discordgo.Session, gb *discordgo.GuildBanAdd) {
		b.BannedUsers.Set(gb.User.ID, struct{}{})
	}
}

func OnMessageRemove(b *bot.Bot) func(*discordgo.Session, *discordgo.MessageDelete) {
	return func(s *discordgo.Session, m *discordgo.MessageDelete) {
		var log = b.Log.With("channel_id", m.ChannelID, "parent_id", m.ID)
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
			log.With("user_id", msg.AuthorID).Info("removing children messages")

			for _, child := range msg.Children {
				log.With("user_id", msg.AuthorID, "message_id", child.MessageID).Info("removing a child message")

				b.EmbedCache.Remove(
					child.ChannelID, child.MessageID,
				)

				err := s.ChannelMessageDelete(child.ChannelID, child.MessageID)
				if err != nil {
					log.With("error", err, "message_id", child.MessageID).Warn("failed to delete child message")
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
				"guild_id", r.GuildID,
				"channel_id", r.ChannelID,
				"message_id", r.MessageID,
				"user_id", r.UserID,
			)
			name        = r.Emoji.APIName()
			ctx, cancel = context.WithTimeout(context.Background(), 2*time.Minute)
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
					"parent_id", r.MessageID,
					"channel_id", child.ChannelID,
					"message_id", child.MessageID,
					"user_id", r.UserID,
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
					sent, err = p.Crosspost(user.ID, group)
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

		addBookmark := func() error {
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
					"user_id", r.UserID,
					"artwork_id", artworkDB.ID,
					"nsfw", nsfw,
				)
			)

			log.Info("inserting a bookmark")
			added, err := b.Store.AddBookmark(ctx, fav)
			if err != nil {
				return fmt.Errorf("failed to insert a bookmark: %w", err)
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

			eb.Title("💖 Successfully bookmarked an artwork").
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
			if err := addBookmark(); err != nil {
				log.With("error", err).Error("failed to add a bookmark")
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
			"guild_id", r.GuildID,
			"channel_id", r.ChannelID,
			"message_id", r.MessageID,
			"user_id", r.UserID,
		)

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		//Do nothing if user was banned recently. Discord removes all reactions
		//of banned users on the server which in turn removes all bookmarks.
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
						log.With("error", err, "artwork_id", id).Error("failed to find an artwork")
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

		log.With("user_id", r.UserID, "artwork_id", artworkDB.ID).Info("removing a bookmark")
		deleted, err := b.Store.DeleteBookmark(ctx, &store.Bookmark{UserID: r.UserID, ArtworkID: artworkDB.ID})
		if err != nil {
			log.With("error", err).Error("failed to remove a bookmark")
			return
		}

		if !deleted {
			return
		}

		user, err := b.Store.User(ctx, r.UserID)
		if err != nil {
			log.With("error", err, "user_id", r.UserID).Error("failed to find or create a user")
			return
		}

		if !user.DM {
			return
		}

		dmSession := b.ShardManager.SessionForDM()
		ch, err := dmSession.UserChannelCreate(user.ID)
		if err != nil {
			log.With("error", err, "user_id", user.ID).Error("failed to create private channel")
			return
		}

		eb := embeds.NewBuilder()
		eb.Title("💔 Successfully removed a bookmark.").
			Description("If you dislike direct messages disable them by running `bt!userset dm off` command").
			AddField("ID", strconv.Itoa(artworkDB.ID), true).
			AddField("URL", messages.ClickHere(artworkDB.URL), true)

		if len(artworkDB.Images) > 0 {
			eb.Thumbnail(artworkDB.Images[0])
		}

		dmSession.ChannelMessageSendEmbed(ch.ID, eb.Finalize())
	}
}

// OnError creates an error response, logs them and sends the response on Discord.
func OnError(b *bot.Bot) func(*gumi.Ctx, error) {
	return func(ctx *gumi.Ctx, err error) {
		var (
			eb         = embeds.NewBuilder()
			cmdErr     *messages.IncorrectCmd
			usrErr     *messages.UserErr
			artworkErr *artworks.Error
		)

		switch {
		case errors.As(err, &cmdErr):
			eb = onCommandError(b, ctx, cmdErr)
		case errors.As(err, &usrErr):
			eb = onUserError(b, ctx, usrErr)
		case errors.As(err, &artworkErr):
			eb = onArtworkError(b, ctx, artworkErr)
		default:
			eb = onDefaultError(b, ctx, err)
		}

		if eb == nil {
			return
		}

		if err := ctx.ReplyEmbed(eb.Finalize()); err != nil {
			b.Log.With("error", err).Error("failed to reply in error handler")
		}
	}
}

func onArtworkError(b *bot.Bot, gctx *gumi.Ctx, err *artworks.Error) *embeds.Builder {
	eb := embeds.NewBuilder().FailureTemplate("")
	eb.Title("❎ Failed to embed artwork")

	switch {
	// Common errors
	case errors.Is(err, artworks.ErrArtworkNotFound):
		eb.Description("Artwork has been removed or is invalid.")
	case errors.Is(err, artworks.ErrRateLimited):
		eb.Description("Boe Tea was rate limited. Please try again later.")

	// Twitter errors
	case errors.Is(err, twitter.ErrTweetNotFound):
		if gctx.Command == nil {
			return nil
		}

		eb.Description("Tweet not found or is NSFW. NSFW tweets can't be embedded due to API changes.")
	case errors.Is(err, twitter.ErrPrivateAccount):
		eb.Description("Unable to view this tweet because this account owner limits who can view their tweets.")

	default:
		return onDefaultError(b, gctx, err)
	}

	return eb
}

func onCommandError(b *bot.Bot, gctx *gumi.Ctx, err *messages.IncorrectCmd) *embeds.Builder {
	if gctx.Command != nil {
		b.Log.With("error", err, "command", gctx.Command.Name, "arguments", gctx.Args.Raw).Error("failed to execute command due to a command error")
	} else {
		b.Log.With("error", err).Error("a command error occured")
	}

	eb := embeds.NewBuilder()
	eb.FailureTemplate(err.Error() + "\n" + err.Description)
	eb.AddField(err.Embed.Usage, fmt.Sprintf("`%v`", err.Usage))
	eb.AddField(err.Embed.Example, fmt.Sprintf("`%v`", err.Example))
	return eb
}

func onUserError(b *bot.Bot, gctx *gumi.Ctx, err *messages.UserErr) *embeds.Builder {
	if err := err.Unwrap(); err != nil {
		if gctx.Command != nil {
			b.Log.With("error", err, "command", gctx.Command.Name, "arguments", gctx.Args.Raw).Info("failed to execute command due to an user error")
		} else {
			b.Log.With("error", err).Info("an user error occured")
		}
	}

	eb := embeds.NewBuilder()
	return eb.FailureTemplate(err.Error())
}

func onDefaultError(b *bot.Bot, gctx *gumi.Ctx, err error) *embeds.Builder {
	if gctx.Command != nil {
		b.Log.With(
			"error", err,
			"command", gctx.Command.Name,
			"arguments", gctx.Args.Raw,
		).Error("failed to execute command due to an unexpected error")
	} else {
		b.Log.With("error", err).Error("an unexpected error occured")
	}

	eb := embeds.NewBuilder().FailureTemplate("An unexpected error occured. Please try again later.\n" +
		"If error persists, please let the developer know about it with `bt!feedback` command.",
	)

	return eb
}

// OnRateLimit creates a response for users who use bot's command too frequently
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

// OnNoPerms creates a response for users who used a command without required permissions.
func OnNoPerms(b *bot.Bot) func(ctx *gumi.Ctx) error {
	return func(ctx *gumi.Ctx) error {
		eb := embeds.NewBuilder()
		eb.FailureTemplate(messages.NoPerms())

		return ctx.ReplyEmbed(eb.Finalize())
	}
}

// OnNSFW creates a response for users who used a NSFW command in a SFW channel
func OnNSFW(b *bot.Bot) func(ctx *gumi.Ctx) error {
	return func(ctx *gumi.Ctx) error {
		eb := embeds.NewBuilder()

		eb.FailureTemplate(messages.NSFWCommand(ctx.Command.Name))

		return ctx.ReplyEmbed(eb.Finalize())
	}
}

// OnExecute logs every executed command.
func OnExecute(b *bot.Bot) func(ctx *gumi.Ctx) error {
	return func(ctx *gumi.Ctx) error {
		b.Log.With("command", ctx.Command.Name, "arguments", ctx.Args.Raw, "guild_id", ctx.Event.GuildID, "channel_id", ctx.Event.ChannelID).Info("executing command")

		b.Stats.IncrementCommand(ctx.Command.Name)
		return nil
	}
}
