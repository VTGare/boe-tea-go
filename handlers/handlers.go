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
	"github.com/VTGare/boe-tea-go/internal/sender"
	"github.com/VTGare/boe-tea-go/messages"
	"github.com/VTGare/boe-tea-go/post"
	"github.com/VTGare/boe-tea-go/repost"
	"github.com/VTGare/boe-tea-go/router"
	"github.com/VTGare/boe-tea-go/store"
	"github.com/VTGare/embeds"

	"github.com/bwmarrin/discordgo"
	"github.com/julien040/go-ternary"
	"mvdan.cc/xurls/v2"
)

func RegisterHandlers(b *bot.Bot) {
	b.AddHandler(OnReady(b))
	b.AddHandler(OnGuildCreate(b))
	b.AddHandler(OnGuildDelete(b))
	b.AddHandler(OnGuildBanAdd(b))
	b.AddHandler(OnChannelDelete(b))
	b.AddHandler(OnReactionAdd(b))
	b.AddHandler(OnReactionRemove(b))
	b.AddHandler(OnMessageRemove(b))
}

// PrefixResolver returns the guild's command prefixes. Bot mentions are
// handled by the router itself.
func PrefixResolver(b *bot.Bot) router.PrefixResolver {
	return func(s *discordgo.Session, guildID, _ string) []string {
		defaults := []string{"bt!", "bt ", "bt.", "bt?"}
		if s == nil || s.State == nil || s.State.User == nil {
			return defaults
		}

		ctx, cancel := context.WithTimeout(b.Context, 5*time.Second)
		defer cancel()

		g, _ := b.Store.Guild(ctx, guildID)
		if g == nil || g.Prefix == "bt!" {
			return defaults
		}

		return []string{g.Prefix}
	}
}

// ObserveStats counts every executed command for bt!stats.
func ObserveStats(b *bot.Bot) router.Middleware {
	return func(next router.Handler) router.Handler {
		return func(ctx *router.Context) error {
			err := next(ctx)

			if b.Stats != nil && ctx.Command != nil {
				b.Stats.IncrementCommand(ctx.Command.QualifiedName())
			}

			return err
		}
	}
}

// OnMessage runs artwork auto-posting for every message that isn't a command.
func OnMessage(b *bot.Bot) router.FallbackHandler {
	return func(s *discordgo.Session, m *discordgo.MessageCreate) {
		if m == nil || m.Message == nil {
			return
		}

		ctx, cancel := context.WithTimeout(b.Context, 30*time.Second)
		defer cancel()

		guild, created, err := store.GetOrCreateGuild(ctx, b.Store, m.GuildID)
		if err != nil {
			b.Log.With("error", err).Error("fallback message handling failed")

			return
		}

		if created {
			b.Log.With("guild_id", m.GuildID).Info("guild missing from store, creating it")
		}

		if guild == nil {
			return
		}

		if !(len(guild.ArtChannels) == 0 || slices.Contains(guild.ArtChannels, m.ChannelID)) {
			return
		}

		urls := xurls.Strict().FindAllString(m.Content, -1)
		if len(urls) == 0 {
			return
		}

		p := post.NewPoster(post.DepsFromBot(b))
		run := post.RunFromMessage(m.Message, urls, false)

		sent, err := p.Send(ctx, run)
		post.CacheResult(b.EmbedCache, run.AuthorID, run.ChannelID, run.MessageID, sent)

		if err != nil {
			var artworkErr *artworks.Error
			if errors.As(err, &artworkErr) {
				reactionErr := s.MessageReactionAdd(m.ChannelID, m.ID, "😵‍💫")
				if reactionErr != nil && !strings.Contains(reactionErr.Error(), "403") {
					b.Log.With("error", reactionErr).Error("failed to add artwork error reaction")
				}
			}

			b.Log.With("error", err).Warn("fallback message handling failed")
		}
	}
}

// OnReady logs that bot's up.
func OnReady(b *bot.Bot) func(*discordgo.Session, *discordgo.Ready) {
	return func(s *discordgo.Session, r *discordgo.Ready) {
		if r == nil || r.User == nil {
			b.Log.Info("shard is connected")

			return
		}

		b.Log.With("user", r.User.String(), "session_id", r.SessionID, "guilds", len(r.Guilds)).Info("shard is connected")
	}
}

// OnGuildCreate loads server configuration on launch and creates new database entries when joining a new server.
func OnGuildCreate(b *bot.Bot) func(*discordgo.Session, *discordgo.GuildCreate) {
	return func(s *discordgo.Session, g *discordgo.GuildCreate) {
		if g == nil || g.Guild == nil {
			return
		}

		ctx, cancel := context.WithTimeout(b.Context, 5*time.Second)
		defer cancel()

		_, created, err := store.GetOrCreateGuild(ctx, b.Store, g.ID)
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
}

// OnGuildDelete logs guild outages and guilds that kicked the bot out.
func OnGuildDelete(b *bot.Bot) func(*discordgo.Session, *discordgo.GuildDelete) {
	return func(s *discordgo.Session, g *discordgo.GuildDelete) {
		if g == nil || g.Guild == nil {
			return
		}

		log := b.Log.With(
			"guild_id", g.ID,
		)

		log.Info(ternary.If(
			g.Unavailable,
			"guild outage",
			"bot kicked/banned from guild",
		))
	}
}

// OnGuildBanAdd adds a banned server member to temporary banned users cache to prevent them from losing all their bookmarks
// on that server due to Discord removing all reactions of banned users.
func OnGuildBanAdd(b *bot.Bot) func(*discordgo.Session, *discordgo.GuildBanAdd) {
	return func(s *discordgo.Session, gb *discordgo.GuildBanAdd) {
		if gb == nil || gb.User == nil {
			return
		}

		b.BannedUsers.Set(gb.User.ID, struct{}{})
	}
}

func OnChannelDelete(b *bot.Bot) func(*discordgo.Session, *discordgo.ChannelDelete) {
	return func(s *discordgo.Session, ch *discordgo.ChannelDelete) {
		if ch == nil || ch.Channel == nil {
			return
		}

		log := b.Log.With("channel_id", ch.ID, "guild_id", ch.GuildID)

		guild, err := b.Store.Guild(b.Context, ch.GuildID)
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

		if slices.Contains(guild.ArtChannels, ch.ID) {
			_, err = b.Store.DeleteArtChannels(
				b.Context,
				guild.ID,
				[]string{ch.ID},
			)
			if err != nil {
				log.With("error", err).Warn("failed to delete art channel")
			}
		}
	}
}

func OnMessageRemove(b *bot.Bot) func(*discordgo.Session, *discordgo.MessageDelete) {
	return func(s *discordgo.Session, m *discordgo.MessageDelete) {
		if m == nil || m.Message == nil {
			return
		}

		log := b.Log.With("channel_id", m.ChannelID, "parent_id", m.ID)
		msg, ok := b.EmbedCache.Get(
			m.ChannelID, m.ID,
		)

		if !ok {
			return
		}

		b.EmbedCache.Remove(
			m.ChannelID, m.ID,
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

				if err := s.ChannelMessageDelete(child.ChannelID, child.MessageID); err != nil {
					log.With("error", err, "message_id", child.MessageID).Warn("failed to delete child message")
				}
			}
		}
	}
}

func OnReactionAdd(b *bot.Bot) func(*discordgo.Session, *discordgo.MessageReactionAdd) {
	return func(s *discordgo.Session, r *discordgo.MessageReactionAdd) {
		if r == nil || s == nil || s.State == nil || s.State.User == nil {
			return
		}

		// Do nothing for bot's own reactions
		if r.UserID == s.State.User.ID {
			return
		}

		log := b.Log.With(
			"guild_id", r.GuildID,
			"channel_id", r.ChannelID,
			"message_id", r.MessageID,
			"user_id", r.UserID,
		)

		ctx, cancel := context.WithTimeout(b.Context, 30*time.Second)
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

			if !msg.IsParent {
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

			var url string
			if len(msg.Embeds) > 0 && msg.Embeds[0] != nil {
				url = msg.Embeds[0].URL
			}

			if url == "" {
				url = xurls.Strict().FindString(msg.Content)
			}

			if url == "" {
				return nil
			}

			msg.Author = dgUser
			run := post.RunFromMessage(msg, []string{url}, false)

			p := post.NewPoster(post.DepsFromBot(b))

			if user, _ := b.Store.User(ctx, r.UserID); user != nil {
				if group, ok := user.FindGroup(r.ChannelID); ok {
					sent, err := p.Crosspost(ctx, run, user.ID, group)
					post.CacheResult(b.EmbedCache, r.UserID, r.ChannelID, r.MessageID, sent)

					if err != nil {
						return err
					}
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
			if len(msg.Embeds) > 0 && msg.Embeds[0] != nil {
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

			if artworkDB == nil {
				return fmt.Errorf("failed to find or create an artwork: not found")
			}

			var (
				nsfw = r.Emoji.APIName() == "🤤"
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

			if user == nil {
				return fmt.Errorf("failed to find or create a user: not found")
			}

			if !user.DM {
				return nil
			}

			if b.ShardManager == nil {
				return fmt.Errorf("shard manager not ready")
			}

			dmSession := b.ShardManager.SessionForDM()
			if dmSession == nil {
				return fmt.Errorf("no DM session available")
			}

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

		name := r.Emoji.APIName()
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
		if r == nil || s == nil || s.State == nil || s.State.User == nil {
			return
		}

		// Do nothing for bot's own reactions
		if r.UserID == s.State.User.ID {
			return
		}

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
		if len(msg.Embeds) > 0 && msg.Embeds[0] != nil {
			embed := msg.Embeds[0]
			urls = append(urls, embed.URL)
		}

		regex := xurls.Strict()

		if msg != nil {
			if url := regex.FindString(msg.Content); url != "" {
				urls = append(urls, url)
			}
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
			if !errors.Is(err, store.ErrArtworkNotFound) {
				log.With("error", err).Error("failed to find an artwork")
			}

			return
		}

		if artworkDB == nil {
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

		if user == nil {
			return
		}

		if !user.DM {
			return
		}

		if b.ShardManager == nil {
			return
		}

		dmSession := b.ShardManager.SessionForDM()
		if dmSession == nil {
			return
		}

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

// OnError replies to command failures and logs the rest.
func OnError(b *bot.Bot) router.ErrorHandler {
	return func(ctx *router.Context, err error) {
		if ctx == nil {
			b.Log.With("error", err).Error("error with nil context")

			return
		}

		var (
			panicErr    *router.PanicError
			checkErr    *router.CheckError
			cooldownErr *router.CooldownError
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
			if msg, ok := router.UserMessageOf(err); ok {
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

func onCheckError(b *bot.Bot, ctx *router.Context, err *router.CheckError) {
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

func replyFailure(b *bot.Bot, ctx *router.Context, msg string) {
	eb := embeds.NewBuilder()
	eb.FailureTemplate(msg)

	if err := ctx.Reply(router.Embed(eb.Finalize())); err != nil {
		b.Log.With("error", err).Error("failed to reply in error handler")
	}
}

func onCommandError(b *bot.Bot, ctx *router.Context, err *messages.IncorrectCmd) {
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

	if rerr := ctx.Reply(router.Embed(eb.Finalize())); rerr != nil {
		b.Log.With("error", rerr).Error("failed to reply in error handler")
	}
}

func onUserError(b *bot.Bot, ctx *router.Context, err *messages.UserErr) {
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

	if rerr := ctx.Reply(router.Embed(eb.Finalize())); rerr != nil {
		b.Log.With("error", rerr).Error("failed to reply in error handler")
	}
}

func onArtworkError(b *bot.Bot, ctx *router.Context, err *artworks.Error) {
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

	msg, ferr := ctx.Followup(router.Embed(eb.Finalize()))
	if ferr != nil {
		b.Log.With("error", ferr).Error("failed to reply in error handler")

		return
	}

	sender.ExpireMessage(b.Log, ctx.Session, msg)
}
