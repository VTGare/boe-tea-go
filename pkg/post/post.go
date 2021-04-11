package post

import (
	"context"
	"fmt"
	"math/rand"
	"reflect"
	"sync/atomic"
	"time"

	"github.com/VTGare/boe-tea-go/internal/arrays"
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

type Post struct {
	bot  *bot.Bot
	ctx  *gumi.Ctx
	urls []string
}

type fetchResult struct {
	Artworks []artworks.Artwork
	Reposts  []*repost.Repost
	Matched  int
}

func New(bot *bot.Bot, ctx *gumi.Ctx, urls ...string) *Post {
	return &Post{
		bot:  bot,
		ctx:  ctx,
		urls: urls,
	}
}

func (p *Post) Send() error {
	dm := p.ctx.Event.GuildID == ""

	var guild *guilds.Guild
	if dm {
		guild = guilds.UserGuild()
	} else {
		var err error
		guild, err = p.bot.Guilds.FindOne(context.Background(), p.ctx.Event.GuildID)
		if err != nil {
			return err
		}
	}

	res, err := p.fetch(guild, p.ctx.Event.ChannelID)
	if err != nil {
		return err
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

		p.sendReposts(guild, res.Reposts, p.ctx.Event.ChannelID, p.ctx.Event.ID)
	}

	return p.send(guild, p.ctx.Event.ChannelID, res.Artworks, false)
}

func (p *Post) Crosspost(userID, group string, channels []string) error {
	wg, _ := errgroup.WithContext(context.Background())
	for _, channelID := range channels {
		channelID := channelID

		wg.Go(func() error {
			ch, err := p.ctx.Session.Channel(channelID)
			if err != nil {
				return nil
			}

			if _, err := p.ctx.Session.GuildMember(ch.GuildID, userID); err != nil {
				p.bot.Log.Infof(
					"Couldn't crosspost. User: %v. Group: %v. Channel: %v. Error: %v. Removing the channel from user's group.",
					userID, group, channelID, err,
				)

				if _, err := p.bot.Users.DeleteFromGroup(context.Background(), userID, group, channelID); err != nil {
					p.bot.Log.Errorf(
						"Failed to remove a channel from user's group. User: %v. Group: %v. Channel: %v. Error: %v",
						userID, group, channelID, err,
					)
				}

				return nil
			}

			guild, err := p.bot.Guilds.FindOne(context.Background(), ch.GuildID)
			if err != nil {
				return nil
			}

			if guild.Crosspost && len(guild.ArtChannels) == 0 || arrays.AnyString(guild.ArtChannels, ch.ID) {
				res, err := p.fetch(guild, channelID)
				if err != nil {
					return err
				}

				err = p.send(guild, channelID, res.Artworks, true)
				if err != nil {
					return err
				}
			}

			return nil
		})
	}

	if err := wg.Wait(); err != nil {
		return err
	}

	return nil
}

func (p *Post) providers(guild *guilds.Guild) []artworks.Provider {
	providers := make([]artworks.Provider, 0)

	for _, provider := range p.bot.ArtworkProviders {
		switch provider.(type) {
		case *twitter.Twitter:
			if !guild.Twitter {
				continue
			}
		case *pixiv.Pixiv:
			if !guild.Pixiv {
				continue
			}
		}

		providers = append(providers, provider)
	}

	return providers
}

func (p *Post) fetch(guild *guilds.Guild, channelID string) (*fetchResult, error) {
	var (
		wg, _        = errgroup.WithContext(context.Background())
		providers    = p.providers(guild)
		matched      int64
		artworksChan = make(chan interface{}, len(p.urls))
	)

	for _, url := range p.urls {
		url := url //shadowing loop variables to pass them to wg.Go. It's required otherwise variables will stay the same every loop.

		wg.Go(func() error {
			for _, provider := range providers {
				if id, ok := provider.Match(url); ok {
					p.bot.Log.Infof("Matched a URL: %v. Provider: %v", url, reflect.TypeOf(provider))
					atomic.AddInt64(&matched, 1)

					if guild.Repost != "disabled" {
						if rep, _ := p.bot.RepostDetector.Find(channelID, id); rep != nil {
							artworksChan <- rep
							break
						}
					}

					artwork, err := provider.Find(id)
					if err != nil {
						return err
					}

					artworksChan <- artwork
					if guild.Repost != "disabled" {
						err = p.bot.RepostDetector.Create(&repost.Repost{
							ID:        id,
							URL:       url,
							GuildID:   guild.ID,
							ChannelID: channelID,
							MessageID: p.ctx.Event.ID,
						}, 24*time.Hour)

						if err != nil {
							p.bot.Log.Errorf("Error creating a repost: %v", err)
						}
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

func (p *Post) sendReposts(guild *guilds.Guild, reposts []*repost.Repost, channelID, messageID string) {
	local := messages.RepostEmbed()

	eb := embeds.NewBuilder()
	eb.Title(local.Title)
	for _, rep := range reposts {
		eb.AddField(
			rep.ID,
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
			time.Sleep(15 * time.Second)

			p.ctx.Session.ChannelMessageDelete(msg.ChannelID, msg.ID)
		}()
	}
}

func (p *Post) send(guild *guilds.Guild, channelID string, artworks []artworks.Artwork, crosspost bool) error {
	allEmbeds := make([][]*discordgo.MessageEmbed, 0, len(artworks))

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
					return err
				}

				//Silently skip NSFW artworks in safe channels
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

			embeds := artwork.Embeds(quote)
			if skipFirst {
				embeds = embeds[1:]
			}

			if crosspost {
				for _, embed := range embeds {
					embed.Author = &discordgo.MessageEmbedAuthor{
						Name:    messages.CrosspostBy(p.ctx.Event.Author.String()),
						IconURL: p.ctx.Event.Author.AvatarURL(""),
					}
				}
			}

			if len(embeds) > 0 {
				allEmbeds = append(allEmbeds, embeds)
			}
		}
	}

	count := 0
	for _, embeds := range allEmbeds {
		count += len(embeds)
	}

	if count > guild.Limit {
		first := allEmbeds[0][0]
		p.ctx.Session.ChannelMessageSendComplex(channelID, &discordgo.MessageSend{
			Content: messages.LimitExceeded(guild.Limit, count),
			Embed:   first,
		})

		if len(allEmbeds) > 1 {
			for _, embeds := range allEmbeds[1:] {
				p.ctx.Session.ChannelMessageSendEmbed(channelID, embeds[0])
			}
		}
	} else {
		for _, embeds := range allEmbeds {
			for _, embed := range embeds {
				p.ctx.Session.ChannelMessageSendEmbed(channelID, embed)
			}
		}
	}

	return nil
}
