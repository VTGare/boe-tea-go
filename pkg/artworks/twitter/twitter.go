package twitter

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/ReneKroon/ttlcache"
	"github.com/VTGare/boe-tea-go/pkg/artworks"
	"github.com/VTGare/boe-tea-go/pkg/messages"
	models "github.com/VTGare/boe-tea-go/pkg/models/artworks"
	"github.com/VTGare/boe-tea-go/pkg/models/guilds"
	"github.com/VTGare/embeds"
	"github.com/bwmarrin/discordgo"
	"github.com/gocolly/colly/v2"
)

type Twitter struct {
	cache  *ttlcache.Cache
	nitter []string
}

//Artwork is a tweet struct with a media gallery.
type Artwork struct {
	FullName  string
	Username  string
	Snowflake string
	url       string
	Content   string
	Timestamp string
	Likes     int
	Comments  int
	Retweets  int
	Gallery   Gallery
}

//Media is tweet's media file which can be an image, a GIF, or a video. M3U8 is only present with video tweets.
type Media struct {
	URL      string
	M3U8     string
	Animated bool
}

//Gallery is an array of tweet's media files: images, GIFs, or videos.
type Gallery []*Media

//New creates a new Twitter artwork provider.
func New() artworks.Provider {
	c := ttlcache.NewCache()
	c.SetTTL(30 * time.Minute)

	return &Twitter{cache: c, nitter: []string{
		"https://nitter.snopyta.org",
		"https://nitter.42l.fr",
		"https://nitter.nixnet.services",
		"https://nitter.himiko.cloud",
		"https://nitter.cc",
		"https://nitter.net",
	}}
}

func (t Twitter) Match(s string) (string, bool) {
	u, err := url.ParseRequestURI(s)
	if err != nil {
		return "", false
	}

	if !strings.Contains(u.Host, "twitter.com") {
		return "", false
	}

	parts := strings.FieldsFunc(u.Path, func(r rune) bool {
		return r == '/'
	})

	if len(parts) < 3 {
		return "", false
	}

	parts = parts[2:]
	if parts[0] == "status" {
		parts = parts[1:]
	}

	snowflake := parts[0]
	if _, err := strconv.ParseUint(snowflake, 10, 64); err != nil {
		return "", false
	}

	return snowflake, true
}

func (t Twitter) Find(snowflake string) (artworks.Artwork, error) {
	if a, ok := t.get(snowflake); ok {
		return a, nil
	}

	for _, nitter := range t.nitter {
		a, err := t.scrapeTwitter(snowflake, nitter)
		if err != nil {
			continue
		}

		t.set(snowflake, a)
		return a, nil
	}

	return nil, nil
}

func (t Twitter) Enabled(g *guilds.Guild) bool {
	return g.Twitter
}

func (t Twitter) scrapeTwitter(snowflake, nitter string) (*Artwork, error) {
	var (
		res      = &Artwork{Snowflake: snowflake}
		visitURL = fmt.Sprintf(nitter+"/i/status/%v", res.Snowflake)
		c        = colly.NewCollector()
	)

	c.OnHTML(".main-tweet .still-image", func(e *colly.HTMLElement) {
		imageURL := nitter + e.Attr("href")

		imageURL = strings.Replace(imageURL, nitter+"/pic/media%2F", "https://pbs.twimg.com/media/", 1)
		imageURL = strings.TrimSuffix(imageURL, "%3Fname%3Dorig")
		res.Gallery = append(res.Gallery, &Media{
			URL:      imageURL,
			Animated: false,
		})
	})

	c.OnHTML(".main-tweet .gif", func(e *colly.HTMLElement) {
		gif := strings.Replace(e.ChildAttr("source", "src"), "/pic/", "https://", 1)
		gif, _ = url.QueryUnescape(gif)
		res.Gallery = append(res.Gallery, &Media{
			URL:      gif,
			Animated: true,
		})
	})

	c.OnHTML(".main-tweet .icon-container", func(e *colly.HTMLElement) {
		children := e.DOM.Children()

		switch {
		case children.HasClass("icon-comment"):
			num := strings.TrimSpace(e.Text)
			res.Comments = parse(num)
		case children.HasClass("icon-retweet"):
			num := strings.TrimSpace(e.Text)
			res.Retweets = parse(num)
		case children.HasClass("icon-heart"):
			num := strings.TrimSpace(e.Text)
			res.Likes = parse(num)
		}
	})

	c.OnHTML(".main-tweet .tweet-date", func(e *colly.HTMLElement) {
		t, _ := time.Parse("2/1/2006, 15:04:05", e.ChildAttr("a", "title"))
		res.Timestamp = t.Format(time.RFC3339)
	})

	c.OnHTML(".main-tweet .tweet-content", func(e *colly.HTMLElement) {
		res.Content = e.Text
	})

	c.OnHTML(".main-tweet .fullname", func(e *colly.HTMLElement) {
		res.FullName = e.Text
	})

	c.OnHTML(".main-tweet .username", func(e *colly.HTMLElement) {
		res.Username = e.Text
	})

	err := c.Visit(visitURL)

	if err != nil {
		return nil, err
	}

	c.Wait()

	res.url = fmt.Sprintf("https://twitter.com/%v/status/%v", strings.TrimLeft(res.Username, "@"), res.Snowflake)

	return res, nil
}

func parse(s string) int {
	if strings.Contains(s, ",") {
		s = strings.ReplaceAll(s, ",", "")
	}

	num, _ := strconv.Atoi(s)
	return num
}

func (t Twitter) get(snowflake string) (*Artwork, bool) {
	a, ok := t.cache.Get(snowflake)
	if !ok {
		return nil, ok
	}

	return a.(*Artwork), ok
}

func (t Twitter) set(snowflake string, artwork *Artwork) {
	t.cache.Set(snowflake, artwork)
}

//ToModel transforms an artwork to an insertable to database artwork model.
func (a Artwork) ToModel() *models.ArtworkInsert {
	return &models.ArtworkInsert{
		Title:  "",
		Author: a.Username,
		URL:    a.url,
		Images: a.Gallery.Strings(),
	}
}

//Embeds transforms an artwork to DiscordGo embeds.
func (a Artwork) Embeds(footer string) []*discordgo.MessageEmbed {
	var (
		eb     = embeds.NewBuilder()
		length = a.Gallery.Len()
	)

	tweets := make([]*discordgo.MessageEmbed, 0, length)

	if length > 1 {
		eb.Title(fmt.Sprintf("%v (%v) | Page %v / %v", a.FullName, a.Username, 1, length))
	} else {
		eb.Title(fmt.Sprintf("%v (%v)", a.FullName, a.Username))
	}

	eb.URL(a.url).Description(a.Content).TimestampString(a.Timestamp).Footer(footer, "")
	eb.AddField("Retweets", strconv.Itoa(a.Retweets), true)
	eb.AddField("Likes", strconv.Itoa(a.Likes), true)
	if length > 0 {
		art := a.Gallery[0]

		if art.Animated {
			eb.AddField("Video", messages.ClickHere(art.URL))
		} else {
			eb.Image(art.URL)
		}
	}

	tweets = append(tweets, eb.Finalize())

	if length > 1 {
		for ind, art := range a.Gallery[1:] {
			eb := embeds.NewBuilder()

			eb.Title(fmt.Sprintf("%v (%v) | Page %v / %v", a.FullName, a.Username, ind+2, length)).URL(a.url)
			eb.Image(art.URL).Footer(footer, "").TimestampString(a.Timestamp)

			tweets = append(tweets, eb.Finalize())
		}
	}

	return tweets
}

func (a Artwork) URL() string {
	return a.url
}

//Len returns the length of Tweets gallery.
func (g Gallery) Len() int { return len(g) }

//Strings returns an array of strings with tweet's media URLs.
func (g Gallery) Strings() []string {
	ss := make([]string, 0, g.Len())

	for _, media := range g {
		ss = append(ss, media.URL)
	}

	return ss
}
