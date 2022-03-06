package post

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"sync/atomic"
	"time"

	"github.com/VTGare/boe-tea-go/artworks"
	"github.com/VTGare/boe-tea-go/artworks/pixiv"
	"github.com/VTGare/boe-tea-go/artworks/twitter"
	"github.com/VTGare/boe-tea-go/bot"
	"github.com/VTGare/boe-tea-go/internal/arikawautils"
	"github.com/VTGare/boe-tea-go/internal/arikawautils/embeds"
	"github.com/VTGare/boe-tea-go/internal/arrays"
	"github.com/VTGare/boe-tea-go/internal/cache"
	"github.com/VTGare/boe-tea-go/messages"
	"github.com/VTGare/boe-tea-go/repost"
	"github.com/VTGare/boe-tea-go/store"
	"github.com/bwmarrin/discordgo"
	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/gateway"
	"github.com/diamondburned/arikawa/v3/state"
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
	bot      *bot.Bot
	state    *state.State
	event    *gateway.MessageCreateEvent
	urls     []string
	indices  map[int]struct{}
	skipMode SkipMode
}

type fetchResult struct {
	Artworks []artworks.Artwork
	Reposts  []*repost.Repost
	Matched  int
}

func New(bot *bot.Bot, state *state.State, event *gateway.MessageCreateEvent, urls ...string) *Post {
	return &Post{
		bot:      bot,
		state:    state,
		event:    event,
		urls:     urls,
		indices:  make(map[int]struct{}),
		skipMode: SkipModeNone,
	}
}

func (p *Post) Send() ([]*cache.MessageInfo, error) {
	guild, err := p.bot.Store.Guild(context.Background(), p.event.GuildID.String())
	if err != nil {
		return nil, err
	}

	guildID, err := arikawautils.GuildID(guild.ID)
	if err != nil {
		return nil, err
	}

	res, err := p.fetch(guild, p.event.ChannelID, false)
	if err != nil {
		return nil, err
	}

	if len(res.Reposts) > 0 {
		if guild.Repost == "strict" {
			perm, _ := arikawautils.MemberHasPermission(
				p.state,
				guildID,
				p.bot.Me.ID,
				discordgo.PermissionAdministrator|discordgo.PermissionManageMessages,
			)

			if perm && res.Matched == len(res.Reposts) {
				p.state.DeleteMessage(p.event.ChannelID, p.event.ID, "Art repost.")
			}
		}

		p.sendReposts(guild, res.Reposts, 15*time.Second)
	}

	return p.send(guild, p.event.ChannelID, res.Artworks, false)
}

func (p *Post) Crosspost(userID discord.UserID, group string, channels []string) ([]*cache.MessageInfo, error) {
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

		channelID, err := arikawautils.ChannelID(channelID)
		if err != nil {
			return nil, err
		}

		go func(channelID discord.ChannelID) {
			defer wg.Done()
			ch, err := p.state.Channel(channelID)
			if err != nil {
				log.Infow("Couldn't crosspost. Error: %v", err)
				return
			}

			if _, err := p.state.Member(ch.GuildID, userID); err != nil {
				log.Infof("Removing a channel from user's group. User left the server.")
				if _, err := p.bot.Store.DeleteCrosspostChannel(context.Background(), userID.String(), group, channelID.String()); err != nil {
					log.Errorf("Failed to remove a channel from user's group. Error: %v", err)
				}

				return
			}

			guild, err := p.bot.Store.Guild(context.Background(), ch.GuildID.String())
			if err != nil {
				log.Infof("Couldn't crosspost. Find Guild error: %v", err)
				return
			}

			if guild.Crosspost {
				if len(guild.ArtChannels) == 0 || arrays.AnyString(guild.ArtChannels, ch.ID.String()) {
					res, err := p.fetch(guild, channelID, true)
					if err != nil {
						log.Infof("Couldn't crosspost. Fetch error: %v", err)
						return
					}

					sent, err := p.send(guild, channelID, res.Artworks, true)
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

func (p *Post) fetch(guild *store.Guild, channelID discord.ChannelID, crosspost bool) (*fetchResult, error) {
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
						rep, _ := p.bot.RepostDetector.Find(channelID.String(), id)
						if rep != nil {
							artworksChan <- rep

							//If crosspost don't do anything and move on with your life.
							if crosspost || guild.Repost == "strict" {
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
								ChannelID: channelID.String(),
								MessageID: p.event.ID.String(),
							},
							guild.RepostExpiration,
						)

						if err != nil {
							p.bot.Log.Errorf("Error creating a repost: %v", err)
						}
					}

					_, isTwitter := provider.(*twitter.Twitter)
					// Only post the picture if the provider is enabled
					// or the function is called from a command
					// or we're crossposting a twitter artwork.

					// TODO: or command != nil
					if provider.Enabled(guild) || (crosspost && isTwitter) {
						artwork, err := provider.Find(id)
						if err != nil {
							return err
						}

						// Only add reactions to the original message for Twitter links.
						if isTwitter && artwork != nil && artwork.Len() > 0 {
							// TODO: or command == nil
							if guild.Reactions && isTwitter {
								p.addReactions(p.event.Message)
							}
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

	msg, _ := p.state.SendEmbeds(p.event.ChannelID, eb.Build())
	if msg != nil {
		go func() {
			time.Sleep(timeout)

			p.state.DeleteMessage(msg.ChannelID, msg.ID, "")
		}()
	}
}

func (p *Post) send(guild *store.Guild, channelID discord.ChannelID, artworks []artworks.Artwork, crosspost bool) ([]*cache.MessageInfo, error) {
	if len(artworks) == 0 {
		return nil, nil
	}

	allMessages, err := p.generateMessages(guild, artworks, channelID, crosspost)
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
	sendMessage := func(data api.SendMessageData) {
		s := p.state
		if crosspost {
			guildID, _ := arikawautils.GuildID(guild.ID)
			shard, _ := p.bot.ShardManager.FromGuildID(guildID)
			s = shard.(*state.State)
		}

		msg, _ := s.SendMessageComplex(channelID, data)
		if msg != nil {
			sent = append(sent, &cache.MessageInfo{
				MessageID: msg.ID,
				ChannelID: msg.ChannelID,
			})

			//If URL doesn't exist then the embed contains an error message, instead of an artwork.
			if guild.Reactions && len(data.Embeds) != 0 {
				if data.Embeds[0].URL != "" {
					p.addReactions(*msg)
				}
			}
		}
	}

	if count > guild.Limit {
		first := allMessages[0][0]
		first.Content = messages.LimitExceeded(guild.Limit, count)
		if crosspost {
			first.Content = first.Embeds[0].URL + "\n" + first.Content
		}

		sendMessage(first)
		if len(allMessages) > 1 {
			for _, messages := range allMessages[1:] {
				if crosspost {
					messages[0].Content = messages[0].Embeds[0].URL
				}

				sendMessage(messages[0])
			}
		}
	} else {
		for _, messages := range allMessages {
			for _, message := range messages {
				if crosspost {
					message.Content = message.Embeds[0].URL
				}

				sendMessage(message)
			}
		}
	}

	return sent, nil
}

func (p *Post) generateMessages(guild *store.Guild, artworks []artworks.Artwork, channelID discord.ChannelID, crosspost bool) ([][]api.SendMessageData, error) {
	messageSends := make([][]api.SendMessageData, 0, len(artworks))
	for _, artwork := range artworks {
		if artwork != nil {
			skipFirst := false

			switch artwork := artwork.(type) {
			case *twitter.Artwork:
				//Skip first Twitter embed if not a command.

				// TODO: Skip if command.
				if !crosspost {
					skipFirst = true
				}
			case *pixiv.Artwork:
				ch, err := p.state.Channel(channelID)
				if err != nil {
					return nil, err
				}

				// TODO: send feedback instead
				// Silently skip NSFW artworks in safe channels
				if !ch.NSFW && artwork.NSFW {
					continue
				}
			}

			quote := p.bot.Config.RandomQuote(guild.NSFW)
			sends, err := artwork.MessageSends(quote)
			if err != nil {
				return nil, err
			}

			if skipFirst {
				sends = sends[1:]
			}

			if crosspost {
				for _, msg := range sends {
					msg.Embeds[0].Author = &discord.EmbedAuthor{
						Name: messages.CrosspostBy(p.event.Author.Tag()),
						Icon: p.event.Author.AvatarURL(),
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

func (p *Post) addReactions(msg discord.Message) {
	p.state.React(msg.ChannelID, msg.ID, "💖")
	p.state.React(msg.ChannelID, msg.ID, "🤤")
}

func (p *Post) skipArtworks(embeds []api.SendMessageData) []api.SendMessageData {
	if p.skipMode == SkipModeNone {
		return embeds
	}

	filtered := make([]api.SendMessageData, 0)
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
	}

	return filtered
}
