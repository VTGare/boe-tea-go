package nitter

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/VTGare/boe-tea-go/artworks"
	"github.com/VTGare/boe-tea-go/store"
	"github.com/VTGare/embeds"
	"github.com/bwmarrin/discordgo"
)

type MediaType int

const (
	MediaTypeImage MediaType = iota
	MediaTypeGIF
	MediaTypeVideo
)

type Nitter struct {
    artworks.TwitBase
	nitter []string
}

//Artwork is a tweet struct with a media gallery.
type Artwork struct {
    artworks.ArtBase
	FullName  string
	Username  string
	Snowflake string
	url       string
	Content   string
	Timestamp time.Time
	Likes     int
	Comments  int
	Retweets  int
	Gallery   Gallery
}

//Media is tweet's media file which can be an image, a GIF, or a video. M3U8 is only present with video tweets.
type Media struct {
	URL  string
	Type MediaType
}

//Gallery is an array of tweet's media files: images, GIFs, or videos.
type Gallery []*Media

//New creates a new Twitter artwork provider.
func New() artworks.Provider {
	return &Nitter{nitter: []string{
		"https://nitter.42l.fr",
		"https://nitter.pussthecat.org",
		"https://nitter.fdn.fr",
		"https://nitter.namazso.eu",
	}}
}

func (t *Nitter) Find(snowflake string) (artworks.Artwork, error) {
	var lastError error
	for _, nitter := range t.nitter {
		a, err := t.scrapeTwitter(snowflake, nitter)
		if err != nil {
			lastError = err
			continue
		}

		return a, nil
	}

	return nil, lastError
}

func (t *Nitter) Enabled(g *store.Guild) bool {
	return g.Twitter
}

func (t *Nitter) scrapeTwitter(snowflake, baseURL string) (*Artwork, error) {
	resp, err := http.Get(baseURL + "/i/status/" + snowflake)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// TODO: handle error 429.
	// switch resp.StatusCode {
	// case http.StatusTooManyRequests:
	// 	retryAfter := resp.Header.Get("Retry-After")
	// }

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("unexpected status code: %v", resp.Status)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to open a document: %w", err)
	}

	res := &Artwork{Snowflake: snowflake}
	res.Content = doc.Find(".main-tweet .tweet-content").Text()
	res.FullName = doc.Find(".main-tweet .fullname").Text()
	res.Username = doc.Find(".main-tweet .username").Text()

	doc.Find(".main-tweet .still-image").Each(func(_ int, image *goquery.Selection) {
		url, _ := image.Attr("href")

		imageURL := strings.Replace(baseURL+url, baseURL+"/pic/media%2F", "https://pbs.twimg.com/media/", 1)
		res.Gallery = append(res.Gallery, &Media{
			URL:  strings.TrimSuffix(imageURL, "%3Fname%3Dorig"),
			Type: MediaTypeImage,
		})
	})

	doc.Find(".main-tweet .gif").Each(func(i int, s *goquery.Selection) {
		src, _ := s.Find("source").Attr("src")

		gif := strings.Replace(src, "/pic/", "https://", 1)
		gif, _ = url.QueryUnescape(gif)
		res.Gallery = append(res.Gallery, &Media{
			URL:  gif,
			Type: MediaTypeGIF,
		})
	})

	res.Likes = parseCount(doc.Find(".main-tweet .icon-container").Has(".icon-heart").Text())
	res.Retweets = parseCount(doc.Find(".main-tweet .icon-container").Has(".icon-retweet").Text())
	res.Comments = parseCount(doc.Find(".main-tweet .icon-container").Has(".icon-comment").Text())

	date, _ := doc.Find(".main-tweet .tweet-date").Find("a").Attr("title")
	ts, _ := time.Parse("Jan 2, 2006 · 3:04 PM UTC", date)
	res.Timestamp = ts

	username := ""
	if res.Username == "" {
		username = "i"
	} else {
		username = strings.TrimLeft(res.Username, "@")
	}

	res.url = fmt.Sprintf("https://twitter.com/%v/status/%v", username, res.Snowflake)
	return res, nil
}

func parseCount(s string) int {
	s = strings.ReplaceAll(strings.TrimSpace(s), ",", "")

	num, _ := strconv.Atoi(s)
	return num
}

//Embeds transforms an artwork to DiscordGo embeds.
func (a *Artwork) MessageSends(footer string, _ bool) ([]*discordgo.MessageSend, error) {
	var (
		eb     = embeds.NewBuilder()
		length = a.Len()
	)

	tweets := make([]*discordgo.MessageSend, 0, length)

	if length > 1 {
		eb.Title(fmt.Sprintf("%v (%v) | Page %v / %v", a.FullName, a.Username, 1, length))
	} else {
		eb.Title(fmt.Sprintf("%v (%v)", a.FullName, a.Username))
	}

	eb.URL(a.url).Description(a.Content).Timestamp(a.Timestamp)
	eb.AddField("Retweets", strconv.Itoa(a.Retweets), true)
	eb.AddField("Likes", strconv.Itoa(a.Likes), true)

	if footer != "" {
		eb.Footer(footer, "")
	}

	msg := &discordgo.MessageSend{}
	if length > 0 {
		art := a.Gallery[0]

		switch art.Type {
		case MediaTypeGIF:
			resp, err := http.Get(art.URL)
			if err != nil {
				return nil, fmt.Errorf("error downloading twitter gif: %w", err)
			}
			defer resp.Body.Close()

			b, err := io.ReadAll(resp.Body)
			if err != nil {
				return nil, fmt.Errorf("error reading twitter gif: %w", err)
			}

			msg.Files = append(msg.Files, &discordgo.File{
				Name:   fmt.Sprintf("%v.mp4", a.Snowflake),
				Reader: bytes.NewReader(b),
			})
		case MediaTypeImage:
			eb.Image(art.URL)
		}
	}

	msg.Embed = eb.Finalize()
	tweets = append(tweets, msg)

	if length > 1 {
		for ind, art := range a.Gallery[1:] {
			eb := embeds.NewBuilder()

			eb.Title(fmt.Sprintf("%v (%v) | Page %v / %v", a.FullName, a.Username, ind+2, length)).URL(a.url)
			eb.Image(art.URL).Timestamp(a.Timestamp)

			if footer != "" {
				eb.Footer(footer, "")
			}

			tweets = append(tweets, &discordgo.MessageSend{Embeds: []*discordgo.MessageEmbed{eb.Finalize()}})
		}
	}

	return tweets, nil
}

func (a *Artwork) GetAuthor() string { return a.Username }
func (a *Artwork) GetURL() string { return a.url }
func (a *Artwork) Len() int { return len(a.Gallery) } //Len returns the length of Tweets gallery.

//GetImages returns an array of strings with tweet's media URLs.
func (a *Artwork) GetImages() []string {
	urls := make([]string, 0, a.Len())

	for _, media := range a.Gallery {
		urls = append(urls, media.URL)
	}
	return urls
}
