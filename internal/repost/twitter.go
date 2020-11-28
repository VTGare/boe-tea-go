package repost

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/VTGare/boe-tea-go/internal/database"
	"github.com/VTGare/boe-tea-go/internal/embeds"
	"github.com/VTGare/boe-tea-go/pkg/tsuita"
	"github.com/bwmarrin/discordgo"
)

var (
	twitterLogo = "https://abs.twimg.com/icons/apple-touch-icon-192x192.png"
)

func (a *ArtPost) SendTwitter(s *discordgo.Session, tweetMap map[string]bool, skipFirst bool) ([][]*discordgo.MessageSend, error) {
	var (
		tweets = make([][]*discordgo.MessageSend, 0)
		guild  = database.GuildCache[a.event.GuildID]
	)

	t, err := a.fetchTwitterPosts(tweetMap)
	if err != nil {
		return nil, err
	}

	var flag bool
	for _, tweet := range t {
		if len(tweet.Gallery) > 0 {
			flag = true
			break
		}
	}

	if flag && a.event != nil && guild.Reactions {
		s.MessageReactionAdd(a.event.ChannelID, a.event.ID, "💖")
		s.MessageReactionAdd(a.event.ChannelID, a.event.ID, "🤤")
	}

	if skipFirst {
		new := make([]*tsuita.Tweet, 0)
		for _, m := range t {
			if len(m.Gallery) > 1 {
				new = append(new, m)
			}
		}
		t = new
	}

	if len(t) > 0 {
		for _, m := range t {
			embeds, err := a.tweetToEmbeds(m, skipFirst)
			if len(embeds) > 0 {
				if a.IsCrosspost {
					embeds[0].Content = fmt.Sprintf("<%v>", m.URL)
				}

				if err != nil {
					return nil, err
				}
				tweets = append(tweets, embeds)
			}
		}
	}
	return tweets, nil
}

func (a *ArtPost) fetchTwitterPosts(tweets map[string]bool) ([]*tsuita.Tweet, error) {
	var (
		tweetChan = make(chan *tsuita.Tweet, len(tweets))
		errChan   = make(chan error)
		wg        = &sync.WaitGroup{}
	)

	wg.Add(len(tweets))
	for t := range tweets {
		go func(t string) {
			defer wg.Done()
			tweet, err := tsuita.GetTweet(fmt.Sprintf("https://twitter.com/i/web/status/%v", t))
			if err != nil {
				errChan <- err
			} else {
				tweetChan <- tweet
			}
			return
		}(t)
	}

	go func() {
		wg.Wait()
		close(errChan)
		close(tweetChan)
	}()

	for err := range errChan {
		return nil, err
	}

	posts := make([]*tsuita.Tweet, 0)
	for post := range tweetChan {
		posts = append(posts, post)
	}

	return posts, nil
}

func (a *ArtPost) tweetToEmbeds(tweet *tsuita.Tweet, skipFirst bool) ([]*discordgo.MessageSend, error) {
	var (
		messages = make([]*discordgo.MessageSend, 0)
		ind      = 0
	)

	if skipFirst {
		switch len(tweet.Gallery) {
		case 0:
			return messages, nil
		case 1:
			return messages, nil
		default:
			ind++
		}
	}

	for ind, media := range tweet.Gallery[ind:] {
		if skipFirst {
			ind++
		}

		title := ""
		if len(tweet.Gallery) > 1 {
			title = fmt.Sprintf("%v (%v) | Page %v/%v", tweet.FullName, tweet.Username, ind+1, len(tweet.Gallery))
		} else {
			title = fmt.Sprintf("%v (%v)", tweet.FullName, tweet.Username)
		}

		var (
			eb  = embeds.NewBuilder()
			msg = &discordgo.MessageSend{}
		)

		eb.Title(title).URL(tweet.URL).TimestampString(tweet.Timestamp).Footer("Twitter", twitterLogo)
		eb.AddField("Retweets", strconv.Itoa(tweet.Retweets), true).AddField("Likes", strconv.Itoa(tweet.Likes), true)

		if ind == 0 {
			eb.Description(tweet.Content)
		}

		if media.Animated {
			resp, err := http.Get(media.URL)
			if err != nil {
				return nil, err
			}
			defer resp.Body.Close()

			filename := media.URL[strings.LastIndex(media.URL, "/")+1:]
			msg.File = &discordgo.File{
				Name:   filename,
				Reader: resp.Body,
			}
		} else {
			eb.Image(media.URL)
		}

		if a.IsCrosspost {
			eb.Author(fmt.Sprintf("Crosspost requested by %v", a.event.Author.String()), "", a.event.Author.AvatarURL(""))
		}

		msg.Embed = eb.Finalize()
		messages = append(messages, msg)
	}

	return messages, nil
}
