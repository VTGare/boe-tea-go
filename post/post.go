package post

import (
	"context"
	"fmt"
	"reflect"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

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

//SkipMode is an enum that configures what indices are skipped from the send function
type SkipMode int

//SkipMode enum
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
		return nil, err
	}

	res, err := p.fetch(guild, p.ctx.Event.ChannelID)
	if err != nil {
		return nil, err
	}

	if len(res.Reposts) > 0 {
		if guild.Repost == "strict" {
			perm, _ := dgoutils.MemberHasPermission(
				p.ctx.Session,
				guild.ID,
				p.ctx.Session.State.User.ID,
				discordgo.PermissionAdministrator|discordgo.PermissionManageMessages,
			)

			if perm && res.Matched == len(res.Reposts) {
				p.ctx.Session.ChannelMessageDelete(p.ctx.Event.ChannelID, p.ctx.Event.ID)
			}
		}

		p.sendReposts(guild, res.Reposts, 15*time.Second)
	}

	return p.send(guild, p.ctx.Event.ChannelID, res.Artworks)
}

func (p *Post) Crosspost(userID, group string, channels []string) ([]*cache.MessageInfo, error) {
	var (
		wg      = sync.WaitGroup{}
		msgChan = make(chan []*cache.MessageInfo, len(channels))
	)

	wg.Add(len(channels))
	for _, channelID := range channels {
		log := p.bot.Log.With(
			"userID", userID,
			"group", group,
			"channelID", channelID,
		)

		go func(channelID string) {
			defer wg.Done()
			ch, err := p.ctx.Session.Channel(channelID)
			if err != nil {
				log.Infow("Couldn't crosspost. Error: %v", err)
				return
			}

			if _, err := p.ctx.Session.GuildMember(ch.GuildID, userID); err != nil {
				log.Infof("Removing a channel from user's group. User left the server.")
				if _, err := p.bot.Store.DeleteCrosspostChannel(context.Background(), userID, group, channelID); err != nil {
					log.Errorf("Failed to remove a channel from user's group. Error: %v", err)
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
		wg, _        = errgroup.WithContext(context.Background())
		matched      int64
		artworksChan = make(chan interface{}, len(p.urls)*2)
	)

	for _, url := range p.urls {
		url := url //shadowing loop variables to pass them to wg.Go. It's required otherwise variables will stay the same every loop.

		wg.Go(func() error {
			for _, provider := range p.bot.ArtworkProviders {
				if id, ok := provider.Match(url); ok {
					p.bot.Log.Infof("Matched a URL: %v. Provider: %v", url, reflect.TypeOf(provider))
					atomic.AddInt64(&matched, 1)

					var isRepost bool
					if guild.Repost != "disabled" {
						rep, _ := p.bot.RepostDetector.Find(channelID, id)
						if rep != nil {
							artworksChan <- rep

							//If crosspost don't do anything and move on with your life.
							if p.crosspost || guild.Repost == "strict" {
								return nil
							}

							isRepost = true
						}
					}

					if guild.Repost != "disabled" && !isRepost {
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
							p.bot.Log.Errorf("error creating a repost: %v", err)
						}
					}

					_, isTwitter := provider.(*twitter.Twitter)
					// Only post the picture if the provider is enabled
					// or the function is called from a command
					// or we're crossposting a twitter artwork.
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
							p.addReactions(p.ctx.Event.Message)
						}

						artworksChan <- artwork
					}

					break
				}
			}

			return nil
		})
	}

	if err := wg.Wait(); err != nil {
		return nil, err
	}

	close(artworksChan)

	res := &fetchResult{
		Artworks: make([]artworks.Artwork, 0),
		Reposts:  make([]*repost.Repost, 0),
		Matched:  int(matched),
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

func (p *Post) sendReposts(guild *store.Guild, reposts []*repost.Repost, timeout time.Duration) {
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
				messages.NamedLink(locale.OriginalMessage, fmt.Sprintf("https://discord.com/channels/%v/%v/%v", rep.GuildID, rep.ChannelID, rep.MessageID)),
			),
		)
	}

	msg, _ := p.ctx.Session.ChannelMessageSendEmbed(p.ctx.Event.ChannelID, eb.Finalize())
	if msg != nil {
		go func() {
			time.Sleep(timeout)

			p.ctx.Session.ChannelMessageDelete(msg.ChannelID, msg.ID)
		}()
	}
}

func (p *Post) send(guild *store.Guild, channelID string, artworks []artworks.Artwork) ([]*cache.MessageInfo, error) {
	if len(artworks) == 0 {
		return nil, nil
	}

	lenArtworks := int64(len(artworks))
	p.bot.Metrics.IncrementArtwork(lenArtworks)

	allMessages, err := p.generateMessages(guild, artworks, channelID)
	if err != nil {
		return nil, err
	}

	if len(allMessages) == 0 {
		return nil, nil
	}

	//If skipMode not equals none, remove certain indices from the embeds array.
	//It only happens from the command so only one artwork should be affected.
	if p.skipMode != SkipModeNone {
		allMessages[0] = p.skipArtworks(allMessages[0])
	}

	count := 0
	for _, messages := range allMessages {
		count += len(messages)
	}

	sent := make([]*cache.MessageInfo, 0, count)
	sendMessage := func(send *discordgo.MessageSend) {
		var s *discordgo.Session
		if p.crosspost {
			guildID, _ := strconv.ParseInt(guild.ID, 10, 64)
			s = p.bot.ShardManager.SessionForGuild(guildID)
		} else {
			s = p.ctx.Session
		}

		msg, _ := s.ChannelMessageSendComplex(channelID, send)

		if msg != nil {
			sent = append(sent, &cache.MessageInfo{
				MessageID: msg.ID,
				ChannelID: msg.ChannelID,
			})

			//If URL doesn't exist then the embed contains an error message, instead of an artwork.
			// TODO: make sure artwork providers always use Embeds instead of embed.
			if guild.Reactions && (send.Embed != nil || len(send.Embeds) > 0) {
				var ok bool
				switch {
				case send.Embed != nil:
					ok = send.Embed.URL != ""
				case len(send.Embeds) > 0:
					ok = send.Embeds[0].URL != ""
				}

				if ok {
					p.addReactions(msg)
				}
			}
		}
	}

	first := allMessages[0][0]
	if count > guild.Limit {
		first.Content = messages.LimitExceeded(guild.Limit, count)
	}

	if p.crosspost {
		var embed *discordgo.MessageEmbed
		if first.Embed == nil {
			embed = first.Embeds[0]
		} else {
			embed = first.Embed
		}

		first.Content = embed.URL + "\n" + first.Content
	}

	for _, messages := range allMessages {
		for _, message := range messages {
			if !p.crosspost {
				message.AllowedMentions = &discordgo.MessageAllowedMentions{} // disable reference ping.
				message.Reference = &discordgo.MessageReference{
					GuildID:   p.ctx.Event.GuildID,
					ChannelID: p.ctx.Event.ChannelID,
					MessageID: p.ctx.Event.ID,
				}
			}

			sendMessage(message)
		}
	}

	return sent, nil
}

func (p *Post) generateMessages(guild *store.Guild, artworks []artworks.Artwork, channelID string) ([][]*discordgo.MessageSend, error) {
	messageSends := make([][]*discordgo.MessageSend, 0, len(artworks))
	for _, artwork := range artworks {
		if artwork != nil {
			var quote string
			if guild.FlavourText {
				quote = p.bot.Config.RandomQuote(guild.NSFW)
			}

			sends, err := artwork.MessageSends(quote, guild.Tags)
			if err != nil {
				return nil, err
			}

			if p.skipFirst(artwork) {
				sends = sends[1:]
			}

			if p.crosspost {
				for _, msg := range sends {
					if len(msg.Embeds) > 0 {
						msg.Embeds[0].Author = &discordgo.MessageEmbedAuthor{
							Name:    messages.CrosspostBy(p.ctx.Event.Author.String()),
							IconURL: p.ctx.Event.Author.AvatarURL(""),
						}
					} else if msg.Embed != nil {
						msg.Embed.Author = &discordgo.MessageEmbedAuthor{
							Name:    messages.CrosspostBy(p.ctx.Event.Author.String()),
							IconURL: p.ctx.Event.Author.AvatarURL(""),
						}
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

func (p *Post) addReactions(msg *discordgo.Message) {
	p.ctx.Session.MessageReactionAdd(
		msg.ChannelID, msg.ID, "💖",
	)

	p.ctx.Session.MessageReactionAdd(
		msg.ChannelID, msg.ID, "🤤",
	)
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

func (p *Post) skipFirst(a artworks.Artwork) bool {
	if p.ctx.Command != nil {
		return false
	}

	tweet, isTwitter := a.(twitter.Artwork)
	if !isTwitter {
		return false
	}

	if a.Len() == 0 {
		return true
	}

	if len(tweet.Videos) > 0 || tweet.NSFW || p.crosspost {
		return false
	}

	return true
}
