package post

import (
	"context"
	"fmt"
	"math/rand"
	"reflect"
	"sync"
	"sync/atomic"
	"time"

	"github.com/VTGare/boe-tea-go/internal/arrays"
	"github.com/VTGare/boe-tea-go/internal/cache"
	"github.com/VTGare/boe-tea-go/internal/dgoutils"
	"github.com/VTGare/boe-tea-go/pkg/artworks"
	"github.com/VTGare/boe-tea-go/pkg/artworks/pixiv"
	"github.com/VTGare/boe-tea-go/pkg/artworks/twitter"
	"github.com/VTGare/boe-tea-go/pkg/bot"
	"github.com/VTGare/boe-tea-go/pkg/messages"
	"github.com/VTGare/boe-tea-go/pkg/models/guilds"
	"github.com/VTGare/boe-tea-go/pkg/repost"
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
	bot      *bot.Bot
	ctx      *gumi.Ctx
	urls     []string
	indices  map[int]struct{}
	skipMode SkipMode
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
	guild, err := p.bot.Guilds.FindOne(context.Background(), p.ctx.Event.GuildID)
	if err != nil {
		return nil, err
	}

	res, err := p.fetch(guild, p.ctx.Event.ChannelID, false)
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

	return p.send(guild, p.ctx.Event.ChannelID, res.Artworks, false)
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
				if _, err := p.bot.Users.DeleteFromGroup(context.Background(), userID, group, channelID); err != nil {
					log.Errorf("Failed to remove a channel from user's group. Error: %v", err)
				}

				return
			}

			guild, err := p.bot.Guilds.FindOne(context.Background(), ch.GuildID)
			if err != nil {
				log.Infof("Couldn't crosspost. Find Guild error: %v", err)
				return
			}

			if guild.Crosspost {
				if len(guild.ArtChannels) == 0 || arrays.AnyString(guild.ArtChannels, ch.ID) {
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

func (p *Post) fetch(guild *guilds.Guild, channelID string, crosspost bool) (*fetchResult, error) {
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

					if guild.Reactions && p.ctx.Command == nil {
						p.addReactions(p.ctx.Event.Message)
					}

					var isRepost bool
					if guild.Repost != "disabled" {
						rep, _ := p.bot.RepostDetector.Find(channelID, id)
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
								ChannelID: channelID,
								MessageID: p.ctx.Event.ID,
							},
							24*time.Hour,
						)

						if err != nil {
							p.bot.Log.Errorf("Error creating a repost: %v", err)
						}
					}

					//Only post the picture if the provider is enabled or
					//the function is called from a command.
					if provider.Enabled(guild) || p.ctx.Command != nil {
						artwork, err := provider.Find(id)
						if err != nil {
							return err
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

func (p *Post) sendReposts(guild *guilds.Guild, reposts []*repost.Repost, timeout time.Duration) {
	local := messages.RepostEmbed()

	eb := embeds.NewBuilder()
	eb.Title(local.Title)
	for _, rep := range reposts {
		eb.AddField(
			fmt.Sprintf("Artwork ID: %v", rep.ID),
			fmt.Sprintf(
				"**%v:** %v\n**%v:** %v\n**URL:** %v",
				local.OriginalMessage, messages.ClickHere(fmt.Sprintf("https://discord.com/channels/%v/%v/%v", rep.GuildID, rep.ChannelID, rep.MessageID)),
				local.ExpiresIn, time.Until(rep.Expire).Round(time.Second),
				messages.ClickHere(rep.URL),
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

func (p *Post) send(guild *guilds.Guild, channelID string, artworks []artworks.Artwork, crosspost bool) ([]*cache.MessageInfo, error) {
	if len(artworks) == 0 {
		return nil, nil
	}

	for range artworks {
		p.bot.Metrics.IncrementArtwork()
	}

	allMessages, err := p.generateMessages(artworks, channelID, crosspost)
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
		msg, _ := p.ctx.Session.ChannelMessageSendComplex(channelID, send)

		if msg != nil {
			sent = append(sent, &cache.MessageInfo{
				MessageID: msg.ID,
				ChannelID: msg.ChannelID,
			})

			//If URL doesn't exist then the embed contains an error message, instead of an artwork.
			if guild.Reactions && send.Embed != nil {
				if send.Embed.URL != "" {
					p.addReactions(msg)
				}
			}
		}
	}

	if count > guild.Limit {
		first := allMessages[0][0]
		first.Content = messages.LimitExceeded(guild.Limit, count)
		if crosspost {
			first.Content = first.Embed.URL + "\n" + first.Content
		}

		sendMessage(first)
		if len(allMessages) > 1 {
			for _, messages := range allMessages[1:] {
				if crosspost {
					messages[0].Content = messages[0].Embed.URL
				}

				sendMessage(messages[0])
			}
		}
	} else {
		for _, messages := range allMessages {
			for _, message := range messages {
				if crosspost {
					message.Content = message.Embed.URL
				}

				sendMessage(message)
			}
		}
	}

	return sent, nil
}

func (p *Post) generateMessages(artworks []artworks.Artwork, channelID string, crosspost bool) ([][]*discordgo.MessageSend, error) {
	messageSends := make([][]*discordgo.MessageSend, 0, len(artworks))
	for _, artwork := range artworks {
		if artwork != nil {
			skipFirst := false

			switch artwork := artwork.(type) {
			case *twitter.Artwork:
				//Skip first Twitter embed if not a command.
				if p.ctx.Command == nil && !crosspost {
					skipFirst = true
				}
			case *pixiv.Artwork:
				ch, err := p.ctx.Session.Channel(channelID)
				if err != nil {
					return nil, err
				}

				// TODO: send feedback instead
				// Silently skip NSFW artworks in safe channels
				if !ch.NSFW && artwork.NSFW {
					continue
				}
			}

			//Random number generator for a quote.
			s := rand.NewSource(time.Now().Unix())
			r := rand.New(s)

			var quote string
			if l := len(p.bot.Config.Quotes); l > 0 {
				quote = p.bot.Config.Quotes[r.Intn(l)].Content
			}

			sends, err := artwork.MessageSends(quote)
			if err != nil {
				return nil, err
			}

			if skipFirst {
				sends = sends[1:]
			}

			if crosspost {
				for _, msg := range sends {
					msg.Embed.Author = &discordgo.MessageEmbedAuthor{
						Name:    messages.CrosspostBy(p.ctx.Event.Author.String()),
						IconURL: p.ctx.Event.Author.AvatarURL(""),
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
