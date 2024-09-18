package post

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/VTGare/boe-tea-go/artworks"
	"github.com/VTGare/boe-tea-go/artworks/twitter"
	"github.com/VTGare/boe-tea-go/bot"
	"github.com/VTGare/boe-tea-go/internal/arrays"
	"github.com/VTGare/boe-tea-go/internal/cache"
	"github.com/VTGare/boe-tea-go/internal/dgoutils"
	"github.com/VTGare/boe-tea-go/messages"
	"github.com/VTGare/boe-tea-go/repost"
	"github.com/VTGare/boe-tea-go/store"
	"github.com/VTGare/embeds"
	"github.com/VTGare/gumi"
	"github.com/bwmarrin/discordgo"
	"golang.org/x/sync/errgroup"
)

// SkipMode is an enum that configures what Indices are skipped from the send function
type SkipMode int

// SkipMode enum
const (
	SkipModeNone SkipMode = iota
	SkipModeInclude
	SkipModeExclude
)

type Post struct {
	Bot            *bot.Bot
	Ctx            *gumi.Ctx
	Urls           []string
	Indices        map[int]struct{}
	SkipMode       SkipMode
	CrosspostMode  bool
	ExcludeChannel bool
}

type fetchResult struct {
	Artworks []artworks.Artwork
	Reposts  []*repost.Repost
	Matched  int
}

func New(bot *bot.Bot, gctx *gumi.Ctx, skip SkipMode, urls ...string) *Post {
	return &Post{
		Bot:            bot,
		Ctx:            gctx,
		Urls:           urls,
		Indices:        make(map[int]struct{}),
		SkipMode:       skip,
		CrosspostMode:  false,
		ExcludeChannel: false,
	}
}

func (p *Post) Send(ctx context.Context) error {
	guild, err := p.Bot.Store.Guild(ctx, p.Ctx.Event.GuildID)
	if err != nil {
		return fmt.Errorf("failed to get a guild: %w", err)
	}

	user, err := p.Bot.Store.User(ctx, p.Ctx.Event.Author.ID)
	if err != nil {
		return fmt.Errorf("failed to get a user: %w", err)
	}

	if user.Ignore && p.Ctx.Command == nil {
		return nil
	}

	res, err := p.fetch(ctx, guild, p.Ctx.Event.ChannelID)
	if err != nil {
		return fmt.Errorf("failed to fetch artworks: %w", err)
	}

	p.handleReposts(guild, res)

	sent, err := p.sendMessages(guild, p.Ctx.Event.ChannelID, res.Artworks)
	if err != nil {
		return err
	}

	allSent := make([]*cache.MessageInfo, 0)
	allSent = append(allSent, sent...)

	if group, ok := user.FindGroup(p.Ctx.Event.ChannelID); user.Crosspost && ok {
		// If channels were successfully excluded, crosspost to trimmed channels.
		// Otherwise, don't crosspost at all.
		if p.ExcludeChannel {
			excludedChannels := make(map[string]struct{})
			for _, arg := range strings.Fields(p.Ctx.Args.Raw) {
				id := dgoutils.TrimmerRaw(arg)
				excludedChannels[id] = struct{}{}
			}

			filtered := arrays.Filter(group.Children, func(s string) bool {
				_, ok := excludedChannels[s]
				return !ok
			})

			if len(group.Children) > len(filtered) {
				group.Children = filtered
			}
		}

		sent, err = p.Crosspost(ctx, user.ID, group)
		if err != nil {
			return err
		}
		allSent = append(allSent, sent...)
	}

	if len(allSent) < 1 {
		return nil
	}

	p.Bot.EmbedCache.Set(
		p.Ctx.Event.Author.ID,
		p.Ctx.Event.ChannelID,
		p.Ctx.Event.ID,
		true,
		allSent...,
	)

	for _, msg := range allSent {
		p.Bot.EmbedCache.Set(
			p.Ctx.Event.Author.ID,
			msg.ChannelID,
			msg.MessageID,
			false,
		)
	}

	return nil
}

func (p *Post) Crosspost(ctx context.Context, userID string, group *store.Group) ([]*cache.MessageInfo, error) {
	user, err := p.Bot.Store.User(ctx, userID)
	if err != nil {
		return nil, err
	}

	if user.Ignore && p.Ctx.Command == nil {
		return []*cache.MessageInfo{}, nil
	}

	if group.IsPair {
		group.Children = arrays.Remove(group.Children, p.Ctx.Event.Message.ChannelID)
	}

	var (
		wg      = sync.WaitGroup{}
		msgChan = make(chan []*cache.MessageInfo, len(group.Children))
	)

	wg.Add(len(group.Children))
	for _, channelID := range group.Children {
		log := p.Bot.Log.With(
			"user_id", userID,
			"group", group,
			"channel_id", channelID,
		)

		go func(channelID string) {
			defer wg.Done()
			ch, err := p.Ctx.Session.Channel(channelID)
			if err != nil {
				log.With("error", err).Info("failed to crosspost")
				return
			}

			if _, err := p.Ctx.Session.GuildMember(ch.GuildID, userID); err != nil {
				log.Debug("member left the server, removing crosspost channel")
				if _, err := p.Bot.Store.DeleteCrosspostChannel(ctx, userID, group.Name, channelID); err != nil {
					log.With("error", err).Error("failed to remove a channel from user's group")
				}

				return
			}

			guild, err := p.Bot.Store.Guild(ctx, ch.GuildID)
			if err != nil {
				log.With("error", err).Info("failed to find guild")
				return
			}

			if guild.Crosspost {
				if len(guild.ArtChannels) == 0 || slices.Contains(guild.ArtChannels, ch.ID) {
					p.CrosspostMode = true
					res, err := p.fetch(ctx, guild, channelID)
					if err != nil {
						log.With("error", err).Error("failed to fetch artworks")
						return
					}

					sent, err := p.sendMessages(guild, channelID, res.Artworks)
					if err != nil {
						log.With("error", err).Error("failed to send messages")
						return
					}

					msgChan <- sent
				}
			}
		}(channelID)
	}

	go func() {
		wg.Wait()
		close(msgChan)
	}()

	sent := make([]*cache.MessageInfo, 0)
	for msg := range msgChan {
		sent = append(sent, msg...)
	}

	return sent, nil
}

func (p *Post) fetch(ctx context.Context, guild *store.Guild, channelID string) (*fetchResult, error) {
	var (
		log = p.Bot.Log.With(
			"guild_id", guild.ID,
			"channel_id", channelID,
		)

		matched      = make(map[string]struct{})
		artworksChan = make(chan any, len(p.Urls)*2)
	)

	wg, gctx := errgroup.WithContext(ctx)
	for _, url := range p.Urls {
		for _, provider := range p.Bot.ArtworkProviders {
			id, ok := provider.Match(url)
			if !ok {
				continue
			}

			// If this artwork ID was matched before, skip it.
			if _, ok := matched[id]; ok {
				break
			}

			matched[id] = struct{}{}

			var (
				provider = provider
				url      = url
			)

			wg.Go(func() error {
				log := log.With(
					"provider", reflect.TypeOf(provider),
					"url", url,
				)
				log.Debug("matched a url")

				var isRepost bool
				if guild.Repost != store.GuildRepostDisabled {
					rep, err := p.Bot.RepostDetector.Find(gctx, channelID, id)
					if err != nil && !errors.Is(err, repost.ErrNotFound) {
						log.Error("failed to find a repost")
					}

					if rep != nil {
						artworksChan <- rep
						if p.CrosspostMode || guild.Repost == store.GuildRepostStrict {
							return nil
						}

						isRepost = true
					}

					if !isRepost {
						err := p.Bot.RepostDetector.Create(
							gctx,
							&repost.Repost{
								ID:        id,
								URL:       url,
								GuildID:   guild.ID,
								ChannelID: channelID,
								MessageID: p.Ctx.Event.ID,
							},
							guild.RepostExpiration,
						)
						if err != nil {
							log.With("error", err).Error("error creating a repost")
						}
					}
				}

				_, isTwitter := provider.(*twitter.Twitter)

				// Only post the artwork any of the following is true:
				// - The provider is enabled in guild settings.
				// - The function is called from a command
				// - Crossposting a Twitter artwork. Bypasses Guild settings by design.
				if provider.Enabled(guild) || p.Ctx.Command != nil || (p.CrosspostMode && isTwitter) {
					var (
						artwork artworks.Artwork
						key     = fmt.Sprintf("%T:%v", provider, id)
					)

					if i, ok := p.Bot.ArtworkCache.Get(key); ok {
						artwork = i.(artworks.Artwork)
					} else {
						var err error
						artwork, err = provider.Find(id)
						if err != nil {
							return err
						}

						p.Bot.ArtworkCache.Set(key, artwork, 0)
					}

					// Only add reactions to the original message for Twitter links.
					if guild.Reactions && p.Ctx.Command == nil && isTwitter && artwork != nil && artwork.Len() > 0 && !p.CrosspostMode {
						err := p.addBookmarkReactions(p.Ctx.Event.Message)
						if err != nil {
							log.With("error", err).Debug("failed to add bookmark reactions")
						}
					}

					go func() {
						p.Bot.Stats.IncrementArtwork(provider)
					}()

					artworksChan <- artwork
				}

				return nil
			})

			break
		}
	}

	if err := wg.Wait(); err != nil {
		return nil, err
	}

	close(artworksChan)

	res := &fetchResult{
		Artworks: make([]artworks.Artwork, 0),
		Reposts:  make([]*repost.Repost, 0),
		Matched:  len(matched),
	}

	for art := range artworksChan {
		switch art := art.(type) {
		case *repost.Repost:
			res.Reposts = append(res.Reposts, art)
		case artworks.Artwork:
			res.Artworks = append(res.Artworks, art)
		}
	}

	return res, nil
}

func (p *Post) handleReposts(guild *store.Guild, res *fetchResult) error {
	log := p.Bot.Log.With(
		"guild_id", guild.ID,
		"user_id", p.Ctx.Event.Author.ID,
	)

	if len(res.Reposts) == 0 {
		return nil
	}

	if guild.Repost == store.GuildRepostStrict {
		perm, err := dgoutils.MemberHasPermission(
			p.Ctx.Session,
			guild.ID,
			p.Ctx.Session.State.User.ID,
			discordgo.PermissionAdministrator|discordgo.PermissionManageMessages,
		)
		if err != nil {
			log.With("error", err).Warn("failed to check delete message perms")
		}

		if perm && res.Matched == len(res.Reposts) {
			var (
				channelID = p.Ctx.Event.ChannelID
				messageID = p.Ctx.Event.ID
			)

			err := p.Ctx.Session.ChannelMessageDelete(channelID, messageID)
			if err != nil {
				log.With(
					"channel_id", channelID,
					"message_id", messageID,
				).Warn("failed to delete original repost message")
			}
		}
	}

	locale := messages.RepostEmbed()

	eb := embeds.NewBuilder()
	eb.Title(locale.Title)
	for ind, rep := range res.Reposts {
		eb.AddField(
			fmt.Sprintf("#%v | %v", ind+1, rep.ID),
			fmt.Sprintf(
				"**%v %v**\n**URL:** %v\n\n%v",
				locale.Expires, messages.RelativeTimestamp(rep.ExpiresAt),
				rep.URL,
				messages.NamedLink(
					locale.OriginalMessage,
					fmt.Sprintf("https://discord.com/channels/%v/%v/%v", rep.GuildID, rep.ChannelID, rep.MessageID),
				),
			),
		)
	}

	msg, err := p.Ctx.Session.ChannelMessageSendEmbed(p.Ctx.Event.ChannelID, eb.Finalize())
	if err != nil {
		return fmt.Errorf("failed to send message to discord: %w", err)
	}

	dgoutils.ExpireMessage(p.Bot, p.Ctx.Session, msg)
	return nil
}

func (p *Post) sendMessages(guild *store.Guild, channelID string, artworks []artworks.Artwork) ([]*cache.MessageInfo, error) {
	sent := make([]*cache.MessageInfo, 0)
	if len(artworks) == 0 {
		return sent, nil
	}

	allMessages, err := p.generateMessages(guild, artworks)
	if err != nil {
		return nil, err
	}

	if len(allMessages) == 0 {
		return sent, nil
	}

	mediaCount := 0
	for _, artwork := range artworks {
		mediaCount += artwork.Len()
	}

	// It only happens from commands so only first artwork should be affected.
	allMessages[0] = p.skipArtworks(allMessages[0])
	sendMessage := func(message *discordgo.MessageSend) error {
		s := p.Ctx.Session
		if p.CrosspostMode {
			guildID, err := strconv.ParseInt(guild.ID, 10, 64)
			if err != nil {
				return fmt.Errorf("failed to parse guild id: %w", err)
			}

			s = p.Bot.ShardManager.SessionForGuild(guildID)
		}

		msg, err := s.ChannelMessageSendComplex(channelID, message)
		if err != nil {
			return fmt.Errorf("failed to send message: %w", err)
		}

		sent = append(sent, &cache.MessageInfo{MessageID: msg.ID, ChannelID: msg.ChannelID})

		// If URL isn't set then it's an error embed.
		// If media count equals 0, it's most likely a Tweet without images and can't be bookmarked.
		if guild.Reactions && len(message.Embeds) > 0 && message.Embeds[0].URL != "" && mediaCount != 0 {
			err := p.addBookmarkReactions(msg)
			if err != nil && !strings.Contains(err.Error(), "403") {
				return fmt.Errorf("failed to add reactions: %w", err)
			}
		}

		return nil
	}

	allMessages = p.handleLimit(allMessages, guild.Limit)
	if p.CrosspostMode {
		first := allMessages[0][0]

		first.Content = first.Embeds[0].URL + "\n" + first.Content
	}

	log := p.Bot.Log.With(
		"guild_id", guild.ID,
		"channel_id", channelID,
		"crosspost", p.CrosspostMode,
	)

	for _, messages := range allMessages {
		for _, message := range messages {
			err := sendMessage(message)
			if err != nil {
				log.With(err).Warn("failed to send artwork message")
			}
		}
	}

	return sent, nil
}

func (p *Post) generateMessages(guild *store.Guild, artworks []artworks.Artwork) ([][]*discordgo.MessageSend, error) {
	messageSends := make([][]*discordgo.MessageSend, 0, len(artworks))
	for _, artwork := range artworks {
		if artwork != nil {
			var quote string
			if guild.FlavorText {
				quote = p.Bot.Config.RandomQuote(guild.NSFW)
			}

			sends, err := artwork.MessageSends(quote, guild.Tags)
			if err != nil {
				return nil, err
			}

			if p.skipFirst(guild, artwork) {
				sends = sends[1:]
			}

			for _, msg := range sends {
				if len(msg.Embeds) == 0 {
					continue
				}

				if p.CrosspostMode {
					msg.Embeds[0].Author = &discordgo.MessageEmbedAuthor{
						Name:    messages.CrosspostBy(p.Ctx.Event.Author.Username),
						IconURL: p.Ctx.Event.Author.AvatarURL(""),
					}
				} else {
					msg.AllowedMentions = &discordgo.MessageAllowedMentions{} // disable reference ping.
					msg.Reference = &discordgo.MessageReference{
						GuildID:   p.Ctx.Event.GuildID,
						ChannelID: p.Ctx.Event.ChannelID,
						MessageID: p.Ctx.Event.ID,
					}
				}
			}

			if len(sends) > 0 {
				messageSends = append(messageSends, sends)
			}
		}
	}

	return messageSends, nil
}

func (p *Post) skipArtworks(embeds []*discordgo.MessageSend) []*discordgo.MessageSend {
	filtered := make([]*discordgo.MessageSend, 0)
	switch p.SkipMode {
	case SkipModeExclude:
		for ind, val := range embeds {
			if _, ok := p.Indices[ind+1]; !ok {
				filtered = append(filtered, val)
			}
		}
	case SkipModeInclude:
		for ind, val := range embeds {
			if _, ok := p.Indices[ind+1]; ok {
				filtered = append(filtered, val)
			}
		}
	case SkipModeNone:
		return embeds
	}

	return filtered
}

func (p *Post) skipFirst(guild *store.Guild, a artworks.Artwork) bool {
	if !guild.SkipFirst {
		return false
	}

	if p.Ctx.Command != nil {
		return false
	}

	tweet, isTwitter := a.(*twitter.Artwork)
	if !isTwitter {
		return false
	}

	if a.Len() == 0 {
		return true
	}

	if len(tweet.Videos) > 0 || p.CrosspostMode {
		return false
	}

	return true
}

func (*Post) handleLimit(allMessages [][]*discordgo.MessageSend, limit int) [][]*discordgo.MessageSend {
	count := 0
	for _, messages := range allMessages {
		count += len(messages)
	}

	if count <= limit {
		return allMessages
	}

	allMessages[0][0].Content = messages.LimitExceeded(limit, len(allMessages), count)
	if len(allMessages) == 1 {
		allMessages[0] = allMessages[0][:limit]
		return allMessages
	}

	filtered := make([][]*discordgo.MessageSend, 0, limit)
	for _, messages := range allMessages {
		if len(messages) > 0 {
			filtered = append(filtered, []*discordgo.MessageSend{messages[0]})
		}
	}

	return filtered
}

func (p *Post) addBookmarkReactions(msg *discordgo.Message) error {
	reactions := []string{"💖", "🤤"}
	for _, reaction := range reactions {
		err := p.Ctx.Session.MessageReactionAdd(msg.ChannelID, msg.ID, reaction)
		if err != nil {
			return err
		}
	}

	return nil
}
