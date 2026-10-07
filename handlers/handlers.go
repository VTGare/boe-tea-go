package handlers

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/VTGare/boe-tea-go/artworks"
	"github.com/VTGare/boe-tea-go/artworks/twitter"
	"github.com/VTGare/boe-tea-go/bot"
	"github.com/VTGare/boe-tea-go/internal/dgoutils"
	"github.com/VTGare/boe-tea-go/internal/embeds"
	"github.com/VTGare/boe-tea-go/messages"
	"github.com/VTGare/boe-tea-go/post"
	"github.com/VTGare/boe-tea-go/repost"
	"github.com/VTGare/boe-tea-go/store"
	"github.com/VTGare/gumi/v2"

	disgobot "github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/snowflake/v2"
	"mvdan.cc/xurls/v2"
)

func RegisterHandlers(b *bot.Bot) {
	b.AddHandler(disgobot.NewListenerFunc(OnReady(b)))
	b.AddHandler(disgobot.NewListenerFunc(func(e *events.GuildReady) { onGuild(b, e.Guild.Guild) }))
	b.AddHandler(disgobot.NewListenerFunc(func(e *events.GuildAvailable) { onGuild(b, e.Guild.Guild) }))
	b.AddHandler(disgobot.NewListenerFunc(func(e *events.GuildJoin) { onGuild(b, e.Guild.Guild) }))
	b.AddHandler(disgobot.NewListenerFunc(OnGuildLeave(b)))
	b.AddHandler(disgobot.NewListenerFunc(OnGuildUnavailable(b)))
	b.AddHandler(disgobot.NewListenerFunc(OnGuildBan(b)))
	b.AddHandler(disgobot.NewListenerFunc(OnChannelDelete(b)))
	b.AddHandler(disgobot.NewListenerFunc(OnReactionAdd(b)))
	b.AddHandler(disgobot.NewListenerFunc(OnReactionRemove(b)))
	b.AddHandler(disgobot.NewListenerFunc(OnMessageRemove(b)))
}

// PrefixResolver returns the guild's command prefixes. Bot mentions are
// handled by the router itself.
func PrefixResolver(b *bot.Bot) gumi.PrefixResolver {
	return func(_ *disgobot.Client, guildID, _ snowflake.ID) []string {
		ctx, cancel := context.WithTimeout(b.Context, 5*time.Second)
		defer cancel()

		g, _ := b.Store.Guild(ctx, dgoutils.IDString(guildID))

		return guildPrefixes(g)
	}
}

func guildPrefixes(g *store.Guild) []string {
	if g == nil || g.Prefix == "bt!" {
		return []string{"bt!", "bt ", "bt.", "bt?"}
	}

	return []string{g.Prefix}
}

// hasPrefix reports whether content starts with one of the guild's prefixes.
// Users put a prefix before a link to stop the bot from posting it.
func hasPrefix(g *store.Guild, content string) bool {
	for _, p := range guildPrefixes(g) {
		if len(content) >= len(p) && strings.EqualFold(content[:len(p)], p) {
			return true
		}
	}

	return false
}

// ObserveStats counts every executed command for bt!stats.
func ObserveStats(b *bot.Bot) gumi.Middleware {
	return func(next gumi.Handler) gumi.Handler {
		return func(ctx *gumi.Context) error {
			err := next(ctx)

			if b.Stats != nil && ctx.Command != nil {
				b.Stats.IncrementCommand(ctx.Command.QualifiedName())
			}

			return err
		}
	}
}

// OnMessage runs artwork auto-posting for every message that isn't a command.
func OnMessage(b *bot.Bot) gumi.FallbackHandler {
	return func(e *events.MessageCreate) {
		m := e.Message

		ctx, cancel := context.WithTimeout(b.Context, 30*time.Second)
		defer cancel()

		var guildID snowflake.ID
		if m.GuildID != nil {
			guildID = *m.GuildID
		}

		guild, created, err := store.GetOrCreateGuild(ctx, b.Store, dgoutils.IDString(guildID))
		if err != nil {
			b.Log.With("error", err).Error("fallback message handling failed")

			return
		}

		if created {
			b.Log.With("guild_id", guildID).Info("guild missing from store, creating it")
		}

		if guild == nil {
			return
		}

		if !guild.PostsIn(m.ChannelID.String()) || hasPrefix(guild, m.Content) {
			return
		}

		urls := xurls.Strict().FindAllString(m.Content, -1)
		if len(urls) == 0 {
			return
		}

		p := post.NewPoster(post.DepsFromBot(b))
		run := post.RunFromMessage(&m, urls, false)

		sent, err := p.Send(ctx, run)
		post.CacheResult(b.EmbedCache, run.AuthorID, run.ChannelID, run.MessageID, sent)

		if err != nil {
			var artworkErr *artworks.Error
			if errors.As(err, &artworkErr) {
				reactionErr := b.Sender.AddReaction(m.ChannelID, m.ID, "😵‍💫")
				if reactionErr != nil && !dgoutils.IsForbidden(reactionErr) {
					b.Log.With("error", reactionErr).Error("failed to add artwork error reaction")
				}
			}

			b.Log.With("error", err).Warn("fallback message handling failed")
		}
	}
}

// OnReady logs that a shard is up.
func OnReady(b *bot.Bot) func(*events.Ready) {
	return func(e *events.Ready) {
		b.Log.With(
			"shard_id", e.ShardID(),
			"user", e.User.Username,
			"session_id", e.SessionID,
			"guilds", len(e.Guilds),
		).Info("shard is connected")
	}
}

// onGuild makes sure every guild the bot sees has settings. Guilds arrive
// on startup, after outages and when the bot joins.
func onGuild(b *bot.Bot, g discord.Guild) {
	ctx, cancel := context.WithTimeout(b.Context, 5*time.Second)
	defer cancel()

	_, created, err := store.GetOrCreateGuild(ctx, b.Store, g.ID.String())
	if err != nil {
		b.Log.With(
			"error", err,
			"guild_id", g.ID,
		).Error("failed to ensure a new guild")

		return
	}

	if created {
		b.Log.With("guild", g.Name, "guild_id", g.ID).Info("invited to a new server")
	}
}

func OnGuildLeave(b *bot.Bot) func(*events.GuildLeave) {
	return func(e *events.GuildLeave) {
		b.Log.With("guild_id", e.GuildID).Info("bot kicked/banned from guild")
	}
}

func OnGuildUnavailable(b *bot.Bot) func(*events.GuildUnavailable) {
	return func(e *events.GuildUnavailable) {
		b.Log.With("guild_id", e.GuildID).Info("guild outage")
	}
}

// OnGuildBan adds a banned server member to temporary banned users cache to prevent them from losing all their bookmarks
// on that server due to Discord removing all reactions of banned users.
func OnGuildBan(b *bot.Bot) func(*events.GuildBan) {
	return func(e *events.GuildBan) {
		b.BannedUsers.Set(e.User.ID.String(), struct{}{})
	}
}

func OnChannelDelete(b *bot.Bot) func(*events.GuildChannelDelete) {
	return func(e *events.GuildChannelDelete) {
		channelID := e.ChannelID.String()
		log := b.Log.With("channel_id", channelID, "guild_id", e.GuildID)

		guild, err := b.Store.Guild(b.Context, e.GuildID.String())
		if err != nil {
			log.With("error", err).Warn("failed to find guild")
			return
		}

		if guild == nil {
			return
		}

		if len(guild.ArtChannels) == 0 {
			return
		}

		if slices.Contains(guild.ArtChannels, channelID) {
			_, err = b.Store.DeleteArtChannels(
				b.Context,
				guild.ID,
				[]string{channelID},
			)
			if err != nil {
				log.With("error", err).Warn("failed to delete art channel")
			}
		}
	}
}

func OnMessageRemove(b *bot.Bot) func(*events.MessageDelete) {
	return func(e *events.MessageDelete) {
		log := b.Log.With("channel_id", e.ChannelID, "parent_id", e.MessageID)
		msg, ok := b.EmbedCache.Get(
			e.ChannelID.String(), e.MessageID.String(),
		)

		if !ok {
			return
		}

		b.EmbedCache.Remove(
			e.ChannelID.String(), e.MessageID.String(),
		)

		if msg.IsParent {
			log.With("user_id", msg.AuthorID).Info("removing children messages")

			for _, child := range msg.Children {
				log.With("user_id", msg.AuthorID, "message_id", child.MessageID).Info("removing a repost")
				if err := b.RepostDetector.Delete(b.Context, child.ChannelID, child.ArtworkID); err != nil {
					if !errors.Is(err, repost.ErrNotFound) {
						log.With("error", err).Warn("failed to remove repost")
					}
				}

				log.With("user_id", msg.AuthorID, "message_id", child.MessageID).Info("removing a child message")

				b.EmbedCache.Remove(
					child.ChannelID, child.MessageID,
				)

				err := b.Sender.DeleteMessage(dgoutils.ParseID(child.ChannelID), dgoutils.ParseID(child.MessageID))
				if err != nil {
					log.With("error", err, "message_id", child.MessageID).Warn("failed to delete child message")
				}
			}
		}
	}
}

func OnReactionAdd(b *bot.Bot) func(*events.MessageReactionAdd) {
	return func(r *events.MessageReactionAdd) {
		// Do nothing for bot's own reactions
		if r.UserID == r.Client().ID() {
			return
		}

		rest := r.Client().Rest

		log := b.Log.With(
			"guild_id", r.GuildID,
			"channel_id", r.ChannelID,
			"message_id", r.MessageID,
			"user_id", r.UserID,
		)

		ctx, cancel := context.WithTimeout(b.Context, 30*time.Second)
		defer cancel()

		deleteEmbed := func() error {
			msg, ok := b.EmbedCache.Get(r.ChannelID.String(), r.MessageID.String())
			if !ok {
				return nil
			}

			if msg.AuthorID != r.UserID.String() {
				return nil
			}

			log.Infof("deleting a message from reaction event")
			b.EmbedCache.Remove(r.ChannelID.String(), r.MessageID.String())

			err := b.Sender.DeleteMessage(r.ChannelID, r.MessageID)
			if err != nil {
				return err
			}

			if !msg.IsParent {
				return nil
			}

			log.Infof("removing children messages")
			childrenIDs := make(map[snowflake.ID][]snowflake.ID)
			for _, child := range msg.Children {
				log.With(
					"parent_id", r.MessageID,
					"channel_id", child.ChannelID,
					"message_id", child.MessageID,
					"user_id", r.UserID,
				).Infof("removing a child message")

				b.EmbedCache.Remove(child.ChannelID, child.MessageID)

				channelID := dgoutils.ParseID(child.ChannelID)
				childrenIDs[channelID] = append(childrenIDs[channelID], dgoutils.ParseID(child.MessageID))
			}

			for channelID, messageIDs := range childrenIDs {
				// Bulk deletes take 2 to 100 messages.
				var err error
				if len(messageIDs) == 1 {
					err = b.Sender.DeleteMessage(channelID, messageIDs[0])
				} else {
					err = rest.BulkDeleteMessages(channelID, messageIDs)
				}

				if err != nil {
					log.With("error", err).Warn("failed to delete children messages")
				}
			}

			return nil
		}

		crosspost := func() error {
			msg, err := rest.GetMessage(r.ChannelID, r.MessageID)
			if err != nil {
				return err
			}

			dgUser, err := rest.GetUser(r.UserID)
			if err != nil {
				return err
			}

			if dgUser.Bot {
				return nil
			}

			var url string
			if len(msg.Embeds) > 0 {
				url = msg.Embeds[0].URL
			}

			if url == "" {
				url = xurls.Strict().FindString(msg.Content)
			}

			if url == "" {
				return nil
			}

			// REST messages don't say which guild they're in.
			msg.GuildID = r.GuildID
			msg.Author = *dgUser
			run := post.RunFromMessage(msg, []string{url}, false)

			p := post.NewPoster(post.DepsFromBot(b))

			if user, _ := b.Store.User(ctx, r.UserID.String()); user != nil {
				if group, ok := user.FindGroup(r.ChannelID.String()); ok {
					sent, err := p.Crosspost(ctx, run, r.UserID, group)
					post.CacheResult(b.EmbedCache, r.UserID, r.ChannelID, r.MessageID, sent)

					if err != nil {
						return err
					}
				}
			}

			return nil
		}

		addBookmark := func() error {
			msg, err := rest.GetMessage(r.ChannelID, r.MessageID)
			if err != nil {
				return fmt.Errorf("failed to get a discord message: %w", err)
			}

			dgUser, err := rest.GetUser(r.UserID)
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

			artworkDB, err := b.FindArtwork(ctx, artwork.URL())
			if errors.Is(err, store.ErrArtworkNotFound) {
				toStore := artwork.StoreArtwork()
				toStore.SourceKey = b.SourceKey(artwork.URL())
				artworkDB, err = b.Store.CreateArtwork(ctx, toStore)
			}

			if err != nil {
				return fmt.Errorf("failed to find or create an artwork: %w", err)
			}

			if artworkDB == nil {
				return fmt.Errorf("failed to find or create an artwork: not found")
			}

			var (
				nsfw = emojiName(r.Emoji) == "🤤"
				fav  = &store.Bookmark{
					UserID:    r.UserID.String(),
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

			user, err := b.Store.User(ctx, r.UserID.String())
			if err != nil {
				return fmt.Errorf("failed to find or create a user: %w", err)
			}

			if user == nil {
				return fmt.Errorf("failed to find or create a user: not found")
			}

			if !user.DM {
				return nil
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

			if err := dgoutils.SendDM(r.Client(), r.UserID, eb.Finalize()); err != nil {
				log.With("error", err).Debug("failed to send a bookmark DM")
			}

			return nil
		}

		name := emojiName(r.Emoji)
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

func OnReactionRemove(b *bot.Bot) func(*events.MessageReactionRemove) {
	return func(r *events.MessageReactionRemove) {
		// Do nothing for bot's own reactions
		if r.UserID == r.Client().ID() {
			return
		}

		rest := r.Client().Rest

		log := b.Log.With(
			"guild_id", r.GuildID,
			"channel_id", r.ChannelID,
			"message_id", r.MessageID,
			"user_id", r.UserID,
		)

		ctx, cancel := context.WithTimeout(b.Context, 10*time.Second)
		defer cancel()

		// Do nothing if user was banned recently. Discord removes all reactions
		// of banned users on the server which in turn removes all bookmarks.
		if _, ok := b.BannedUsers.Get(r.UserID.String()); ok {
			return
		}

		if name := emojiName(r.Emoji); name != "💖" && name != "🤤" {
			return
		}

		msg, err := rest.GetMessage(r.ChannelID, r.MessageID)
		if err != nil {
			log.With("error", err).Error("failed to get discord message")
			return
		}

		dgUser, err := rest.GetUser(r.UserID)
		if err != nil {
			log.With("error", err).Error("failed to get discord user")
			return
		}

		if dgUser.Bot {
			return
		}

		urls := make([]string, 0, 2)
		if len(msg.Embeds) > 0 {
			urls = append(urls, msg.Embeds[0].URL)
		}

		if url := xurls.Strict().FindString(msg.Content); url != "" {
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

		artworkDB, err := b.FindArtwork(ctx, artwork.URL())
		if err != nil {
			if !errors.Is(err, store.ErrArtworkNotFound) {
				log.With("error", err).Error("failed to find an artwork")
			}

			return
		}

		if artworkDB == nil {
			return
		}

		log.With("user_id", r.UserID, "artwork_id", artworkDB.ID).Info("removing a bookmark")

		deleted, err := b.Store.DeleteBookmark(ctx, &store.Bookmark{UserID: r.UserID.String(), ArtworkID: artworkDB.ID})
		if err != nil {
			log.With("error", err).Error("failed to remove a bookmark")
			return
		}

		if !deleted {
			return
		}

		user, err := b.Store.User(ctx, r.UserID.String())
		if err != nil {
			log.With("error", err, "user_id", r.UserID).Error("failed to find or create a user")
			return
		}

		if user == nil {
			return
		}

		if !user.DM {
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

		if err := dgoutils.SendDM(r.Client(), r.UserID, eb.Finalize()); err != nil {
			log.With("error", err).Debug("failed to send a bookmark DM")
		}
	}
}

// emojiName is the emoji itself for Unicode emojis.
func emojiName(e discord.PartialEmoji) string {
	if e.Name == nil {
		return ""
	}

	return *e.Name
}

// OnError replies to command failures and logs the rest.
func OnError(b *bot.Bot) gumi.ErrorHandler {
	return func(ctx *gumi.Context, err error) {
		if ctx == nil {
			b.Log.With("error", err).Error("error with nil context")

			return
		}

		var (
			panicErr    *gumi.PanicError
			checkErr    *gumi.CheckError
			cooldownErr *gumi.CooldownError
			cmdErr      *messages.IncorrectCmd
			usrErr      *messages.UserErr
			artworkErr  *artworks.Error
		)

		switch {
		case errors.As(err, &panicErr):
			fields := []any{"panic", panicErr.Value, "stacktrace", string(panicErr.Stack)}
			if ctx.Command != nil {
				fields = append(fields, "command", ctx.Command.QualifiedName())
			}

			b.Log.With(fields...).Error("recovered from a panic in handler")
		case errors.As(err, &checkErr):
			onCheckError(b, ctx, checkErr)
		case errors.As(err, &cooldownErr):
			replyFailure(b, ctx, messages.RateLimit(cooldownErr.Remaining))
		case errors.As(err, &cmdErr):
			onCommandError(b, ctx, cmdErr)
		case errors.As(err, &usrErr):
			onUserError(b, ctx, usrErr)
		case errors.As(err, &artworkErr):
			onArtworkError(b, ctx, artworkErr)
		default:
			if msg, ok := gumi.UserMessageOf(err); ok {
				replyFailure(b, ctx, msg)

				return
			}

			name := ""
			if ctx.Command != nil {
				name = ctx.Command.QualifiedName()
			}

			b.Log.With("error", err, "command", name).Warn("failed to execute command due to an unexpected error")
		}
	}
}

func onCheckError(b *bot.Bot, ctx *gumi.Context, err *gumi.CheckError) {
	if err.Silent {
		return
	}

	switch err.Check {
	case "permissions", "bot_permissions":
		replyFailure(b, ctx, messages.NoPerms())
	case "nsfw":
		name := ""
		if ctx.Command != nil {
			name = ctx.Command.QualifiedName()
		}

		replyFailure(b, ctx, messages.NSFWCommand(name))
	case "guild_only", "dm_only":
		return
	default:
		replyFailure(b, ctx, err.UserMessage())
	}
}

func replyFailure(b *bot.Bot, ctx *gumi.Context, msg string) {
	eb := embeds.NewBuilder()
	eb.FailureTemplate(msg)

	if err := ctx.Reply(gumi.Embed(eb.Finalize())); err != nil {
		b.Log.With("error", err).Error("failed to reply in error handler")
	}
}

func onCommandError(b *bot.Bot, ctx *gumi.Context, err *messages.IncorrectCmd) {
	name := err.Name
	raw := ""

	if ctx.Command != nil {
		name = ctx.Command.QualifiedName()
	}

	if ctx.Message != nil {
		raw = ctx.Message.Content
	}

	b.Log.With("error", err, "command", name, "arguments", raw).Debug("failed to execute command due to a command error")

	lead := ctx.DisplayPrefix()
	if lead == "" {
		lead = "/"
	}

	eb := embeds.NewBuilder()
	eb.FailureTemplate(err.Error() + "\n" + err.Description)

	if ctx.Command != nil {
		eb.AddField(err.Embed.Usage, fmt.Sprintf("`%v`", ctx.Command.Usage(lead)))
	}

	if len(err.Examples) > 0 {
		examples := make([]string, 0, len(err.Examples))
		for _, ex := range err.Examples {
			examples = append(examples, "`"+lead+ex+"`")
		}

		eb.AddField(err.Embed.Example, strings.Join(examples, "\n"))
	}

	if rerr := ctx.Reply(gumi.Embed(eb.Finalize())); rerr != nil {
		b.Log.With("error", rerr).Error("failed to reply in error handler")
	}
}

func onUserError(b *bot.Bot, ctx *gumi.Context, err *messages.UserErr) {
	if uerr := err.Unwrap(); uerr != nil {
		name := ""
		raw := ""

		if ctx.Command != nil {
			name = ctx.Command.QualifiedName()
		}

		if ctx.Message != nil {
			raw = ctx.Message.Content
		}

		b.Log.With("error", uerr, "command", name, "arguments", raw).Info("failed to execute command due to an user error")
	}

	eb := embeds.NewBuilder()
	eb.FailureTemplate(err.Error())

	if rerr := ctx.Reply(gumi.Embed(eb.Finalize())); rerr != nil {
		b.Log.With("error", rerr).Error("failed to reply in error handler")
	}
}

func onArtworkError(b *bot.Bot, ctx *gumi.Context, err *artworks.Error) {
	content := ""
	if ctx.Message != nil {
		content = ctx.Message.Content
	}

	b.Log.With(
		"guild", ctx.GuildID(),
		"channel", ctx.ChannelID(),
		"content", content,
		"err", err,
	).Info("artwork error occurred")

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
		eb.Description("Tweet not found or is NSFW. NSFW tweets can't be embedded due to API changes.")
	case errors.Is(err, twitter.ErrPrivateAccount):
		eb.Description("Unable to view this tweet because this account owner limits who can view their tweets.")

	default:
		name := ""
		if ctx.Command != nil {
			name = ctx.Command.QualifiedName()
		}

		b.Log.With("error", err, "command", name).Warn("failed to execute command due to an unexpected error")

		return
	}

	msg, ferr := ctx.Followup(gumi.Embed(eb.Finalize()))
	if ferr != nil {
		b.Log.With("error", ferr).Error("failed to reply in error handler")

		return
	}

	b.Sender.Expire(msg)
}
