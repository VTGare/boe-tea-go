package handlers

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/VTGare/boe-tea-go/artworks"
	"github.com/VTGare/boe-tea-go/bot"
	"github.com/VTGare/boe-tea-go/commands"
	"github.com/VTGare/boe-tea-go/internal/arikawautils"
	"github.com/VTGare/boe-tea-go/internal/arikawautils/embeds"
	"github.com/VTGare/boe-tea-go/internal/cache"
	"github.com/VTGare/boe-tea-go/internal/config"
	"github.com/VTGare/boe-tea-go/messages"
	"github.com/VTGare/boe-tea-go/post"
	"github.com/VTGare/boe-tea-go/store"
	"github.com/bwmarrin/discordgo"
	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/gateway"
	"github.com/diamondburned/arikawa/v3/state"
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

func All(b *bot.Bot, s *state.State) []interface{} {
	return []interface{}{
		OnReady(b, s), OnMessageCreate(b, s), OnMessageRemove(b, s), OnGuildCreate(b, s), OnGuildDelete(b, s),
		OnReactionRemove(b, s), OnReactionAdd(b, s), OnGuildBanAdd(b, s), OnInteractionCreate(b, s),
	}
}

func OnMessageCreate(b *bot.Bot, s *state.State) func(m *gateway.MessageCreateEvent) {
	return func(m *gateway.MessageCreateEvent) {
		if b.Config.Env == config.DevEnvironment && m.GuildID != discord.GuildID(b.Config.TestGuildID) {
			return
		}

		log := b.Log.With(
			"author_id", m.Author.ID,
			"guild_id", m.GuildID,
			"message_id", m.ID,
		)

		if m.Author.Bot {
			return
		}

		regex := xurls.Strict()
		url := regex.FindString(m.Content)
		if url == "" {
			return
		}

		p := post.New(b, s, m, url)
		if _, err := p.Send(); err != nil {
			log.With("error", err).Warn("send error")
			return
		}

		user, _ := b.Store.User(context.Background(), m.Author.ID.String())
		if user != nil {
			if group, ok := user.FindGroup(m.ChannelID.String()); ok {
				_, err := p.Crosspost(m.Author.ID, group.Name, group.Children)
				if err != nil {
					log.With("error", err).Warn("crosspost error")
				}
			}
		}
	}
}

func OnInteractionCreate(b *bot.Bot, s *state.State) func(e *gateway.InteractionCreateEvent) {
	return func(e *gateway.InteractionCreateEvent) {
		if b.Config.Env == config.DevEnvironment && e.GuildID != discord.GuildID(b.Config.TestGuildID) {
			return
		}

		var err error
		switch interaction := e.Data.(type) {
		case *discord.PingInteraction:
			err = s.RespondInteraction(e.ID, e.Token, api.InteractionResponse{
				Type: api.PongInteraction,
			})

		case *discord.CommandInteraction:
			cmd, ok := commands.Find(interaction.Name)
			if !ok {
				return
			}

			var resp api.InteractionResponse
			resp, err = cmd.Exec(b, s)
			if err != nil {
				b.Log.With("error", err).Error("failed to execute a command")
			}

			err = s.RespondInteraction(e.ID, e.Token, resp)
		case *discord.AutocompleteInteraction:
		case *discord.ButtonInteraction:
		case *discord.SelectInteraction:
		case *discord.ModalInteraction:
		}

		if err != nil {
			b.Log.With("error", err).Error("failed to respond to interaction")
		}
	}
}

//OnReady logs that bot's up.
func OnReady(b *bot.Bot, s *state.State) func(*gateway.ReadyEvent) {
	return func(r *gateway.ReadyEvent) {
		b.Log.Infof("%v is online. Session ID: %v. Guilds: %v", r.User.Username, r.SessionID, len(r.Guilds))
	}
}

//OnGuildCreate loads server configuration on launch and creates new database entries when joining a new server.
func OnGuildCreate(b *bot.Bot, _ *state.State) func(*gateway.GuildCreateEvent) {
	return func(g *gateway.GuildCreateEvent) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		_, err := b.Store.Guild(ctx, g.ID.String())
		if errors.Is(err, mongo.ErrNoDocuments) {
			b.Log.Infof("Joined a guild. Name: %v. ID: %v", g.Name, g.ID)

			_, err := b.Store.CreateGuild(ctx, g.ID.String())
			if err != nil {
				b.Log.Errorf("Error while inserting guild %v: %v", g.ID, err)
			}
		}
	}
}

//OnGuildDelete logs guild outages and guilds that kicked the bot out.
func OnGuildDelete(b *bot.Bot, s *state.State) func(*gateway.GuildDeleteEvent) {
	return func(g *gateway.GuildDeleteEvent) {
		if g.Unavailable {
			b.Log.Infof("Guild outage. ID: %v", g.ID)
		} else {
			b.Log.Infof("Kicked/banned from guild: %v", g.ID)
		}
	}
}

//OnGuildBanAdd adds a banned server member to temporary banned users cache to prevent them from losing all their favourites
//on that server due to Discord removing all reactions of banned users.
func OnGuildBanAdd(b *bot.Bot, _ *state.State) func(*gateway.GuildBanAddEvent) {
	return func(g *gateway.GuildBanAddEvent) {
		if b.Config.Env == config.DevEnvironment && g.GuildID != discord.GuildID(b.Config.TestGuildID) {
			return
		}

		b.BannedUsers.Set(g.User.ID.String(), struct{}{})
	}
}

func OnMessageRemove(b *bot.Bot, s *state.State) func(*gateway.MessageDeleteEvent) {
	return func(m *gateway.MessageDeleteEvent) {
		if b.Config.Env == config.DevEnvironment && m.GuildID != discord.GuildID(b.Config.TestGuildID) {
			return
		}

		msg, ok := b.EmbedCache.Get(
			m.ChannelID, m.ID,
		)
		if !ok {
			return
		}

		log := b.Log.With("channel", m.ChannelID, "message", m.ID, "user", msg.AuthorID)

		log.Info("removing message cache")
		b.EmbedCache.Remove(m.ChannelID, m.ID)
		if !msg.Parent {
			return
		}

		for _, child := range msg.Children {
			log := log.With("parent", m.ID, "message", child.MessageID, "channel", child.ChannelID)

			log.Info("removing child message")
			b.EmbedCache.Remove(child.ChannelID, child.MessageID)
			if err := s.DeleteMessage(child.ChannelID, child.MessageID, "Removing artwork embeds on user request."); err != nil {
				log.With("error", err).Error("failed to delete message")
			}
		}
	}
}

func OnReactionAdd(b *bot.Bot, s *state.State) func(*gateway.MessageReactionAddEvent) {
	return func(r *gateway.MessageReactionAddEvent) {
		if b.Config.Env == config.DevEnvironment && r.GuildID != discord.GuildID(b.Config.TestGuildID) {
			return
		}

		//Do nothing for bot's own reactions
		if me, err := s.Me(); err != nil {
			if r.UserID == me.ID {
				return
			}
		}

		var (
			log = b.Log.With(
				"guild", r.GuildID,
				"channel", r.ChannelID,
				"message", r.MessageID,
				"user", r.UserID,
			)
			name        = r.Emoji.APIString()
			ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
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
			b.EmbedCache.Remove(
				r.ChannelID, r.MessageID,
			)

			reason := api.AuditLogReason("Deleted artwork on user's request.")
			err := s.DeleteMessage(r.ChannelID, r.MessageID, reason)
			if err != nil {
				return err
			}

			if !msg.Parent {
				return nil
			}

			log.Infof("removing children messages")
			childrenIDs := make(map[discord.ChannelID][]discord.MessageID)
			for _, child := range msg.Children {
				log.With(
					"parent", r.MessageID,
					"channel", child.ChannelID,
					"message", child.MessageID,
					"user", r.UserID,
				).Infof("removing a child message")

				b.EmbedCache.Remove(
					child.ChannelID, child.MessageID,
				)

				if _, ok := childrenIDs[child.ChannelID]; !ok {
					childrenIDs[child.ChannelID] = make([]discord.MessageID, 0)
				}

				childrenIDs[child.ChannelID] = append(childrenIDs[child.ChannelID], child.MessageID)
			}

			for channelID, messageIDs := range childrenIDs {
				if err := s.DeleteMessages(channelID, messageIDs, reason); err != nil {
					log.With("error", err).Warn("failed to delete children messages")
				}
			}

			return nil
		}

		crosspost := func() error {
			msg, err := s.Message(r.ChannelID, r.MessageID)
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

			msg.Author = *dgUser
			p := post.New(b, s, &gateway.MessageCreateEvent{*msg, &discord.Member{User: *dgUser}})

			sent := make([]*cache.MessageInfo, 0)
			user, _ := b.Store.User(ctx, r.UserID.String())
			if user != nil {
				if group, ok := user.FindGroup(r.ChannelID.String()); ok {
					userID, err := arikawautils.UserID(user.ID)
					if err != nil {
						return err
					}

					sent, err = p.Crosspost(userID, group.Name, group.Children)
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
			msg, err := s.Message(r.ChannelID, r.MessageID)
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

			user, err := b.Store.User(ctx, r.UserID.String())
			if err != nil {
				return fmt.Errorf("failed to find or create a user: %w", err)
			}

			var (
				nsfw = name == "🤤"
				fav  = &store.Favourite{
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
			if err := b.Store.AddFavourite(ctx, r.UserID.String(), fav); err != nil {
				return fmt.Errorf("failed to insert a favourite: %w", err)
			}

			if !user.DM {
				return nil
			}

			userID, err := arikawautils.UserID(user.ID)
			if err != nil {
				return fmt.Errorf("failed to parse snowflake: %w", err)
			}

			ch, err := s.CreatePrivateChannel(userID)
			if err != nil {
				return fmt.Errorf("failed to create private channel: %w", err)
			}

			var (
				locale = messages.FavouriteAddedEmbed()
				eb     = embeds.NewBuilder()
			)

			if len(artworkDB.Images) > 0 {
				eb.Thumbnail(artworkDB.Images[0])
			}

			eb.Title(locale.Title).
				Description(locale.Description).
				AddField("ID", strconv.Itoa(artworkDB.ID), true).
				AddField("URL", messages.ClickHere(artworkDB.URL), true).
				AddField("NSFW", strconv.FormatBool(nsfw), true)

			s.SendMessage(ch.ID, "", eb.Build())
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

func OnReactionRemove(b *bot.Bot, s *state.State) func(*gateway.MessageReactionRemoveEvent) {
	return func(r *gateway.MessageReactionRemoveEvent) {
		if b.Config.Env == config.DevEnvironment && r.GuildID != discord.GuildID(b.Config.TestGuildID) {
			return
		}

		//Do nothing for bot's own reactions
		if me, err := s.Me(); err != nil {
			if r.UserID == me.ID {
				return
			}
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
		if _, ok := b.BannedUsers.Get(r.UserID.String()); ok {
			return
		}

		if r.Emoji.APIString() != "💖" && r.Emoji.APIString() != "🤤" {
			return
		}

		msg, err := s.Message(r.ChannelID, r.MessageID)
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

		user, err := b.Store.User(ctx, r.UserID.String())
		if err != nil {
			if !errors.Is(err, mongo.ErrNoDocuments) {
				log.With("error", err).Error("failed to find a user")
			}

			return
		}

		fav, ok := user.FindFavourite(artworkDB.ID)
		if !ok {
			return
		}

		log.With("user", r.UserID, "artwork", artworkDB.ID).Info("removing a favourite")
		if err := b.Store.DeleteFavourite(ctx, user.ID, fav); err != nil {
			log.With("error", err).Error("failed to remove a favourite")
			return
		}

		if !user.DM {
			return
		}

		userID, err := arikawautils.UserID(user.ID)
		if err != nil {
			log.With("error", err).Warn("failed to parse snowflake")
			return
		}

		ch, err := s.CreatePrivateChannel(userID)
		if err != nil {
			log.With("error", err).Error("failed to create private channel")
			return
		}

		var (
			eb     = embeds.NewBuilder()
			locale = messages.FavouriteRemovedEmbed()
		)

		eb.Title(locale.Title).
			Description(locale.Description).
			AddField("ID", strconv.Itoa(artworkDB.ID), true).
			AddField("URL", messages.ClickHere(artworkDB.URL), true).
			AddField("NSFW", strconv.FormatBool(fav.NSFW), true)

		if len(artworkDB.Images) > 0 {
			eb.Thumbnail(artworkDB.Images[0])
		}

		s.SendMessage(ch.ID, "", eb.Build())
	}
}

/*

func OnPanic(b *bot.Bot) func(*gumi.Ctx, interface{}) {
	return func(ctx *gumi.Ctx, r interface{}) {
		b.Log.Errorf("%v", r)
	}
}

func NotCommand(b *bot.Bot) func(*gumi.Ctx) error {
	return func(ctx *gumi.Ctx) error {
		guild, err := b.Store.Guild(ctx, ctx.Event.GuildID)
		if err != nil {
			return err
		}

		allSent := make([]*cache.MessageInfo, 0)
		if len(guild.ArtChannels) == 0 || arrays.AnyString(guild.ArtChannels, ctx.Event.ChannelID) {
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
			user, err := b.Store.User(ctx, ctx.Event.Author.ID)
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

func OnNoPerms(b *bot.Bot) func(ctx *gumi.Ctx) error {
	return func(ctx *gumi.Ctx) error {
		eb := embeds.NewBuilder()
		eb.FailureTemplate(messages.NoPerms())

		return ctx.ReplyEmbed(eb.Finalize())
	}
}

func OnNSFW(b *bot.Bot) func(ctx *gumi.Ctx) error {
	return func(ctx *gumi.Ctx) error {
		eb := embeds.NewBuilder()

		eb.FailureTemplate(messages.NSFWCommand(ctx.Command.Name))

		return ctx.ReplyEmbed(eb.Finalize())
	}
}

func OnExecute(b *bot.Bot) func(ctx *gumi.Ctx) error {
	return func(ctx *gumi.Ctx) error {
		b.Log.Infof("Executing command [%v]. Arguments: [%v]. Guild ID: %v, channel ID: %v", ctx.Command.Name, ctx.Args.Raw, ctx.Event.GuildID, ctx.Event.ChannelID)

		b.Metrics.IncrementCommand()
		return nil
	}
}

*/
