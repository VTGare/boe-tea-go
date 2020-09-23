package tsuita

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/ReneKroon/ttlcache"
	"github.com/gocolly/colly/v2"
	"github.com/sirupsen/logrus"
)

var (
	TwitterRegex = regexp.MustCompile(`https?://(?:mobile.)?twitter.com/(?:\S+)/status/(\d+)(?:\?s=\d\d)?`)

	twitterCache *ttlcache.Cache
	nitterURL    = "https://nitter.net"
)

func init() {
	twitterCache = ttlcache.NewCache()
	twitterCache.SetTTL(1 * time.Hour)
}

type Tweet struct {
	FullName  string
	Username  string
	Snowflake string
	URL       string
	Content   string
	Timestamp string
	Likes     int
	Comments  int
	Retweets  int
	Gallery   []TwitterMedia
}

type TwitterMedia struct {
	URL      string
	Animated bool
}

func GetTweet(uri string) (*Tweet, error) {
	var (
		res   = &Tweet{Gallery: make([]TwitterMedia, 0)}
		match = TwitterRegex.FindStringSubmatch(uri)
	)

	if len(match) == 0 {
		return nil, errors.New("invalid twitter url")
	}

	res.Snowflake = match[1]
	if cache, ok := twitterCache.Get(res.Snowflake); ok {
		logrus.Infof("Found a cached tweet. Snowflake: %v", res.Snowflake)
		return cache.(*Tweet), nil
	}

	logrus.Infof("Fetching a tweet. Snowflake: %v", res.Snowflake)
	nitter := fmt.Sprintf("https://nitter.net/i/status/%v", res.Snowflake)
	c := colly.NewCollector()

	c.OnHTML(".main-tweet .still-image", func(e *colly.HTMLElement) {
		imageURL := nitterURL + e.Attr("href")
		res.Gallery = append(res.Gallery, TwitterMedia{
			URL:      imageURL,
			Animated: false,
		})
	})

	c.OnHTML(".main-tweet .gif", func(e *colly.HTMLElement) {
		imageURL := nitterURL + e.ChildAttr("source", "src")
		res.Gallery = append(res.Gallery, TwitterMedia{
			URL:      imageURL,
			Animated: true,
		})
	})

	parse := func(s string) int {
		if strings.Contains(s, ",") {
			s = strings.ReplaceAll(s, ",", "")
		}
		num, _ := strconv.Atoi(s)
		return num
	}
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

	err := c.Visit(nitter)

	if err != nil {
		return nil, err
	}

	c.Wait()

	logrus.Infof("Fetched a tweet successfully. URL: %v. Images: %v", res.URL, len(res.Gallery))
	twitterCache.Set(match[1], res)

	res.URL = fmt.Sprintf("https://twitter.com/%v/status/%v", strings.TrimLeft(res.Username, "@"), res.Snowflake)
	return res, nil
}
