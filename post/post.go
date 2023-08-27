package post

import (
	"context"
	"errors"
	"fmt"
	"reflect"
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

// SkipMode is an enum that configures what indices are skipped from the send function
type SkipMode int

// SkipMode enum
const (
	SkipModeNone SkipMode = iota
	SkipModeInclude
	SkipModeExclude
)

type Post struct {
	bot       *bot.Bot
	ctx       *gumi.Ctx
	urls      []string
	indices   map[int]struct{}
	skipMode  SkipMode
	crosspost bool
}

type fetchResult struct {
	Artworks []artworks.Artwork
	Reposts  []*repost.Repost
	Matched  int
}

func New(bot *bot.Bot, ctx *gumi.Ctx, urls ...string) *Post {
	return &Post{
		bot:      bot,
		ctx:      ctx,
		urls:     urls,
		indices:  make(map[int]struct{}),
		skipMode: SkipModeNone,
	}
}

func (p *Post) Send() ([]*cache.MessageInfo, error) {
	guild, err := p.bot.Store.Guild(context.Background(), p.ctx.Event.GuildID)
	if err != nil {
		return nil, fmt.Errorf("failed to get a guild: %w", err)
	}

	user, err := p.bot.Store.User(context.Background(), p.ctx.Event.Author.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get a user: %w", err)
	}

	if user.Ignore && p.ctx.Command == nil {
		return []*cache.MessageInfo{}, nil
	}

	res, err := p.fetch(guild, p.ctx.Event.ChannelID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch artworks: %w", err)
	}

	log := p.bot.Log.With(
		"guild_id", guild.ID,
		"user_id", user.ID,
	)

	if len(res.Reposts) > 0 {
		if guild.Repost == store.GuildRepostStrict {
			perm, err := dgoutils.MemberHasPermission(
				p.ctx.Session,
				guild.ID,
				p.ctx.Session.State.User.ID,
				discordgo.PermissionAdministrator|discordgo.PermissionManageMessages,
			)

			if err != nil {
				log.With("error", err).Warn("failed to check if boe tea has permissions")
			}

			if perm && res.Matched == len(res.Reposts) {
				var (
					channelID = p.ctx.Event.ChannelID
					messageID = p.ctx.Event.ID
				)

				err := p.ctx.Session.ChannelMessageDelete(channelID, messageID)
				if err != nil {
					log.With(
						"channel_id", channelID,
						"message_id", messageID,
					).Warn("failed to delete a message")
				}
			}
		}

		err := p.sendReposts(res.Reposts)
		if err != nil {
			log.Warn("failed to send reposts")
		}
	}

	return p.send(guild, p.ctx.Event.ChannelID, res.Artworks)
}

func (p *Post) Crosspost(userID string, group *store.Group) ([]*cache.MessageInfo, error) {
	user, err := p.bot.Store.User(context.Background(), userID)
	if err != nil {
		return nil, err
	}

	if user.Ignore && p.ctx.Command == nil {
		return []*cache.MessageInfo{}, nil
	}

	if group.IsPair {
		group.Children = arrays.Remove(group.Children, p.ctx.Event.Message.ChannelID)
	}

	var (
		wg      = sync.WaitGroup{}
		msgChan = make(chan []*cache.MessageInfo, len(group.Children))
	)

	wg.Add(len(group.Children))
	for _, channelID := range group.Children {
		log := p.bot.Log.With(
			"user_id", userID,
			"group", group,
			"channel_id", channelID,
		)

		go func(channelID string) {
			defer wg.Done()
			ch, err := p.ctx.Session.Channel(channelID)
			if err != nil {
				log.Infow("Couldn't crosspost. Error: %v", err)
				return
			}

			if _, err := p.ctx.Session.GuildMember(ch.GuildID, userID); err != nil {
				log.Debug("member left the server, removing crosspost channel")
				if _, err := p.bot.Store.DeleteCrosspostChannel(context.Background(), userID, group.Name, channelID); err != nil {
					log.With("error", err).Error("failed to remove a channel from user's group")
				}

				return
			}

			guild, err := p.bot.Store.Guild(context.Background(), ch.GuildID)
			if err != nil {
				log.Infof("Couldn't crosspost. Find Guild error: %v", err)
				return
			}

			if guild.Crosspost {
				if len(guild.ArtChannels) == 0 || arrays.Any(guild.ArtChannels, ch.ID) {
					p.crosspost = true
					res, err := p.fetch(guild, channelID)
					if err != nil {
						log.Infof("Couldn't crosspost. Fetch error: %v", err)
						return
					}

					sent, err := p.send(guild, channelID, res.Artworks)
					if err != nil {
						log.Infof("Couldn't crosspost. Send error: %v", err)
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

func (p *Post) SetSkip(indices map[int]struct{}, mode SkipMode) {
	p.indices = indices
	p.skipMode = mode
}

func (p *Post) fetch(guild *store.Guild, channelID string) (*fetchResult, error) {
	var (
		log = p.bot.Log.With(
			"guild_id", guild.ID,
			"channel_id", channelID,
		)

		wg, _        = errgroup.WithContext(context.Background())
		matched      = make(map[string]struct{})
		artworksChan = make(chan interface{}, len(p.urls)*2)
	)

	for _, url := range p.urls {
		for _, provider := range p.bot.ArtworkProviders {
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
					rep, err := p.bot.RepostDetector.Find(channelID, id)
					if err != nil && !errors.Is(err, repost.ErrNotFound) {
						log.Error("failed to find a repost")
					}

					if rep != nil {
						artworksChan <- rep
						if p.crosspost || guild.Repost == store.GuildRepostStrict {
							return nil
						}

						isRepost = true
					}
				}

				if guild.Repost != store.GuildRepostDisabled && !isRepost {
					err := p.bot.RepostDetector.Create(
						&repost.Repost{
							ID:        id,
							URL:       url,
							GuildID:   guild.ID,
							ChannelID: channelID,
							MessageID: p.ctx.Event.ID,
						},
						guild.RepostExpiration,
					)

					if err != nil {
						log.With("error", err).Error("error creating a repost")
					}
				}

				_, isTwitter := provider.(*twitter.Twitter)
				// Only post the artwork any of the following is true:
				// - The provider is enabled in guild settings.
				// - The function is called from a command
				// - Crossposting a Twitter artwork. Bypasses Guild settings by design.
				if provider.Enabled(guild) || p.ctx.Command != nil || (p.crosspost && isTwitter) {
					var (
						artwork artworks.Artwork
						key     = fmt.Sprintf("%T:%v", provider, id)
					)

					if i, ok := p.bot.ArtworkCache.Get(key); ok {
						artwork = i.(artworks.Artwork)
					} else {
						var err error
						artwork, err = provider.Find(id)
						if err != nil {
							return err
						}

						p.bot.ArtworkCache.Set(key, artwork, 0)
					}

					// Only add reactions to the original message for Twitter links.
					if guild.Reactions && p.ctx.Command == nil && isTwitter && artwork != nil && artwork.Len() > 0 && !p.crosspost {
						err := p.addBookmarkReactions(p.ctx.Event.Message)
						if err != nil {
							log.With("error", err).Debug("failed to add bookmark reactions")
						}
					}

					go func() {
						p.bot.Stats.IncrementArtwork(provider)
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

func (p *Post) sendReposts(reposts []*repost.Repost) error {
	locale := messages.RepostEmbed()

	eb := embeds.NewBuilder()
	eb.Title(locale.Title)
	for ind, rep := range reposts {
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

	msg, err := p.ctx.Session.ChannelMessageSendEmbed(p.ctx.Event.ChannelID, eb.Finalize())
	if err != nil {
		return fmt.Errorf("failed to send message to discord: %w", err)
	}

	warning := "failed to delete a detected repost message"
	messages.ExpireMessage(p.bot, p.ctx.Session, msg, warning)

	return nil
}

// TODO: for the love of God rewrite this entire thing
func (p *Post) send(guild *store.Guild, channelID string, artworks []artworks.Artwork) ([]*cache.MessageInfo, error) {
	sentMessages := make([]*cache.MessageInfo, 0)
	if len(artworks) == 0 {
		return sentMessages, nil
	}

	allMessages, err := p.generateMessages(guild, artworks, channelID)
	if err != nil {
		return nil, err
	}

	if len(allMessages) == 0 {
		return sentMessages, nil
	}

	mediaCount := 0
	for _, artwork := range artworks {
		mediaCount += artwork.Len()
	}

	// It only happens from commands so only first artwork should be affected.
	allMessages[0] = p.skipArtworks(allMessages[0])
	sendMessage := func(send *discordgo.MessageSend) error {
		s := p.ctx.Session
		if p.crosspost {
			guildID, err := strconv.ParseInt(guild.ID, 10, 64)
			if err != nil {
				return fmt.Errorf("failed to parse guild id: %w", err)
			}

			s = p.bot.ShardManager.SessionForGuild(guildID)
		}

		msg, err := s.ChannelMessageSendComplex(channelID, send)
		if err != nil {
			return fmt.Errorf("failed to send message: %w", err)
		}

		sentMessages = append(sentMessages, &cache.MessageInfo{MessageID: msg.ID, ChannelID: msg.ChannelID})

		// If URL isn't set then it's an error embed.
		// If media count equals 0, it's most likely a Tweet without images and it can't be bookmarked.
		if guild.Reactions && len(send.Embeds) > 0 && send.Embeds[0].URL != "" && mediaCount != 0 {
			err := p.addBookmarkReactions(msg)
			if err != nil && !strings.Contains(err.Error(), "403") {
				return fmt.Errorf("failed to add reactions: %w", err)
			}
		}

		return nil
	}

	allMessages = p.handleLimit(allMessages, guild.Limit)
	if p.crosspost {
		var (
			first = allMessages[0][0]
		)

		first.Content = first.Embeds[0].URL + "\n" + first.Content
	}

	log := p.bot.Log.With(
		"guild_id", guild.ID,
		"channel_id", channelID,
		"crosspost", p.crosspost,
	)

	for _, messages := range allMessages {
		for _, message := range messages {
			err := sendMessage(message)
			if err != nil {
				log.With(err).Warn("failed to send artwork message")
			}
		}
	}

	return sentMessages, nil
}

func (p *Post) generateMessages(guild *store.Guild, artworks []artworks.Artwork, channelID string) ([][]*discordgo.MessageSend, error) {
	messageSends := make([][]*discordgo.MessageSend, 0, len(artworks))
	for _, artwork := range artworks {
		if artwork != nil {
			var quote string
			if guild.FlavorText {
				quote = p.bot.Config.RandomQuote(guild.NSFW)
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

				if p.crosspost {
					msg.Embeds[0].Author = &discordgo.MessageEmbedAuthor{
						Name:    messages.CrosspostBy(p.ctx.Event.Author.Username),
						IconURL: p.ctx.Event.Author.AvatarURL(""),
					}
				} else {
					msg.AllowedMentions = &discordgo.MessageAllowedMentions{} // disable reference ping.
					msg.Reference = &discordgo.MessageReference{
						GuildID:   p.ctx.Event.GuildID,
						ChannelID: p.ctx.Event.ChannelID,
						MessageID: p.ctx.Event.ID,
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

func (p *Post) addBookmarkReactions(msg *discordgo.Message) error {
	reactions := []string{"💖", "🤤"}
	for _, reaction := range reactions {
		err := p.ctx.Session.MessageReactionAdd(msg.ChannelID, msg.ID, reaction)
		if err != nil {
			return err
		}
	}

	return nil
}

func (p *Post) skipArtworks(embeds []*discordgo.MessageSend) []*discordgo.MessageSend {
	filtered := make([]*discordgo.MessageSend, 0)
	switch p.skipMode {
	case SkipModeExclude:
		for ind, val := range embeds {
			if _, ok := p.indices[ind+1]; !ok {
				filtered = append(filtered, val)
			}
		}
	case SkipModeInclude:
		for ind, val := range embeds {
			if _, ok := p.indices[ind+1]; ok {
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

	if p.ctx.Command != nil {
		return false
	}

	tweet, isTwitter := a.(*twitter.Artwork)
	if !isTwitter {
		return false
	}

	if a.Len() == 0 {
		return true
	}

	if len(tweet.Videos) > 0 || p.crosspost {
		return false
	}

	return true
}

func (p *Post) handleLimit(allMessages [][]*discordgo.MessageSend, limit int) [][]*discordgo.MessageSend {
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
