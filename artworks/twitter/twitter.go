package twitter

import (
	"bytes"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/VTGare/boe-tea-go/artworks"
	"github.com/VTGare/boe-tea-go/artworks/twitter/nitter"
	"github.com/VTGare/boe-tea-go/store"
	"github.com/VTGare/embeds"

	"github.com/bwmarrin/discordgo"
	twitterscraper "github.com/n0madic/twitter-scraper"
)

type Twitter struct {
    artworks.TwitBase
	scraper  *twitterscraper.Scraper
	fallback artworks.Provider
}

type Artwork struct {
    artworks.ArtBase
	Videos    []twitterscraper.Video
	Photos    []string
	ID        string
	FullName  string
	Username  string
	Content   string
	Permalink string
	Timestamp time.Time
	Likes     int
	Replies   int
	Retweets  int
	NSFW      bool
}

type Category struct {
	ID   int
	Name string
}

func New() artworks.Provider {
	return &Twitter{
		scraper:  twitterscraper.New(),
		fallback: nitter.New(),
	}
}

func (t *Twitter) Find(id string) (artworks.Artwork, error) {
	tweet, err := t.scraper.GetTweet(id)
	if err != nil {
		a, err := t.fallback.Find(id)
		if err != nil {
			return nil, err
		}

		nitter, ok := a.(*nitter.Artwork)
		if !ok {
			return nil, errors.New("Twitter API is down. Please use `bt!feedback` command to contact the developer.")
		}

		return convertNitter(nitter), nil
	}

	profile, err := t.scraper.GetProfile(tweet.Username)
	if err != nil {
		return nil, err
	}

	art := &Artwork{
		ID:        id,
		FullName:  profile.Name,
		Username:  "@" + tweet.Username,
		Content:   html.UnescapeString(tweet.Text),
		Likes:     tweet.Likes,
		Replies:   tweet.Replies,
		Retweets:  tweet.Retweets,
		Timestamp: tweet.TimeParsed,
		Photos:    tweet.Photos,
		Videos:    tweet.Videos,
		NSFW:      tweet.SensitiveContent,
		Permalink: tweet.PermanentURL,
	}

	if tweet.QuotedStatus != nil {
		art.Photos = append(art.Photos, tweet.QuotedStatus.Photos...)
		art.Videos = append(art.Videos, tweet.QuotedStatus.Videos...)
	}

	return art, nil
}

func (Twitter) Enabled(g *store.Guild) bool {
	return g.Twitter
}

// Embeds transforms an artwork to DiscordGo embeds.
func (a *Artwork) MessageSends(footer string, _ bool) ([]*discordgo.MessageSend, error) {
	eb := embeds.NewBuilder()
	eb.URL(a.Permalink).Description(a.Content).Timestamp(a.Timestamp)
	eb.AddField("Retweets", strconv.Itoa(a.Retweets), true)
	eb.AddField("Likes", strconv.Itoa(a.Likes), true)

	if footer != "" {
		eb.Footer(footer, "")
	}

	if len(a.Videos) > 0 {
		video := a.Videos[0]

		resp, err := http.Get(video.URL)
		if err != nil {
			return nil, fmt.Errorf("error downloading twitter video: %w", err)
		}
		defer resp.Body.Close()

		b, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("error reading twitter video: %w", err)
		}

		uri, err := url.Parse(video.URL)
		if err != nil {
			return nil, err
		}

		splits := strings.Split(uri.Path, "/")

		eb.Title(fmt.Sprintf("%v (%v)", a.FullName, a.Username))
		msg := &discordgo.MessageSend{
			Embeds: []*discordgo.MessageEmbed{eb.Finalize()},
			Files: []*discordgo.File{
				{
					Name:   splits[len(splits)-1],
					Reader: bytes.NewReader(b),
				},
			},
		}

		return []*discordgo.MessageSend{msg}, nil
	}

	length := len(a.Photos)
	tweets := make([]*discordgo.MessageSend, 0, length)
	if length > 1 {
		eb.Title(fmt.Sprintf("%v (%v) | Page %v / %v", a.FullName, a.Username, 1, length))
	} else {
		eb.Title(fmt.Sprintf("%v (%v)", a.FullName, a.Username))
	}

	if length > 0 {
		eb.Image(a.Photos[0])
	}

	tweets = append(tweets, &discordgo.MessageSend{
		Embeds: []*discordgo.MessageEmbed{eb.Finalize()},
	})

	if len(a.Photos) > 1 {
		for ind, photo := range a.Photos[1:] {
			eb := embeds.NewBuilder()

			eb.Title(fmt.Sprintf("%v (%v) | Page %v / %v", a.FullName, a.Username, ind+2, length)).URL(a.Permalink)
			eb.Image(photo).Timestamp(a.Timestamp)

			if footer != "" {
				eb.Footer(footer, "")
			}

			tweets = append(tweets, &discordgo.MessageSend{Embeds: []*discordgo.MessageEmbed{eb.Finalize()}})
		}
	}

	return tweets, nil
}

func (a *Artwork) GetAuthor() string { return a.Username }
func (a *Artwork) GetURL() string { return a.Permalink }

func (a *Artwork) GetImages() []string {
    media := make([]string, 0, len(a.Photos)+len(a.Videos))
	media = append(media, a.Photos...)
	for _, video := range a.Videos {
		media = append(media, video.Preview)
	}
    return media
}

func (a *Artwork) Len() int {
	if len(a.Videos) != 0 {
		return 1
	}
	return len(a.Photos)
}

func convertNitter(a *nitter.Artwork) *Artwork {
	var (
		videos = make([]twitterscraper.Video, 0)
		photos = make([]string, 0)
	)

	for _, media := range a.Gallery {
		switch media.Type {
		case nitter.MediaTypeGIF:
			videos = append(videos, twitterscraper.Video{URL: media.URL})
		case nitter.MediaTypeImage:
			photos = append(photos, media.URL)
		}
	}

	return &Artwork{
		ID:        a.Snowflake,
		FullName:  a.FullName,
		Username:  a.Username,
		Content:   a.Content,
		Likes:     a.Likes,
		Replies:   a.Comments,
		Retweets:  a.Retweets,
		Timestamp: a.Timestamp,
		Videos:    videos,
		Photos:    photos,
		NSFW:      true, // Fallback method is only used for NSFW artworks.
		Permalink: a.GetURL(),
	}
}
