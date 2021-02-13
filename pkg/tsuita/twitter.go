package tsuita

import (
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/ReneKroon/ttlcache"
	"github.com/gocolly/colly/v2"
	"github.com/sirupsen/logrus"
)

type Tsuita struct {
	TwitterRegex    *regexp.Regexp
	cache           *ttlcache.Cache
	nitterInstances []string
}

func NewTsuita() *Tsuita {
	cache := ttlcache.NewCache()
	cache.SetTTL(1 * time.Hour)

	return &Tsuita{
		cache:        cache,
		TwitterRegex: regexp.MustCompile(`https?://(?:mobile.)?twitter.com/(?:\S+)/status/(\d+)(?:\?s=\d\d)?`),
		nitterInstances: []string{
			"https://nitter.snopyta.org",
			"https://nitter.42l.fr",
			"https://nitter.nixnet.services",
			"https://nitter.himiko.cloud",
			"https://nitter.cc",
			"https://nitter.net",
		},
	}
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

func (t *Tweet) Images() []string {
	images := make([]string, 0)
	for _, i := range t.Gallery {
		if !i.Animated {
			images = append(images, i.URL)
		}
	}

	return images
}

func (ts *Tsuita) GetTweet(uri string) (*Tweet, error) {
	var (
		match = ts.TwitterRegex.FindStringSubmatch(uri)
	)

	if match == nil {
		return nil, ErrBadURL
	}

	snowflake := match[1]
	if cache, ok := ts.cache.Get(snowflake); ok {
		return cache.(*Tweet), nil
	}

	for _, inst := range ts.nitterInstances {
		logrus.Infof("Trying to scrape a Tweet. Instance: %v. Snowflake: %v.", inst, snowflake)
		tweet, err := ts.scrape(inst, snowflake)
		if err != nil {
			logrus.Warnf("Instance failed to scrape a tweet. Instance: %v. Snowflake: %v. Error: %v", inst, snowflake, err)
			continue
		}

		ts.cache.Set(tweet.Snowflake, tweet)
		return tweet, nil
	}

	return nil, ErrRateLimitReached
}

func (ts *Tsuita) scrape(inst, snowflake string) (*Tweet, error) {
	var (
		res = &Tweet{Gallery: make([]TwitterMedia, 0), Snowflake: snowflake}
	)

	nitter := fmt.Sprintf(inst+"/i/status/%v", res.Snowflake)
	c := colly.NewCollector()

	c.OnHTML(".main-tweet .still-image", func(e *colly.HTMLElement) {
		imageURL := inst + e.Attr("href")

		imageURL = strings.Replace(imageURL, inst+"/pic/media%2F", "https://pbs.twimg.com/media/", 1)
		imageURL = strings.TrimSuffix(imageURL, "%3Fname%3Dorig")
		res.Gallery = append(res.Gallery, TwitterMedia{
			URL:      imageURL,
			Animated: false,
		})
	})

	c.OnHTML(".main-tweet .gif", func(e *colly.HTMLElement) {
		gif := strings.Replace(e.ChildAttr("source", "src"), "/pic/", "https://", 1)
		gif, _ = url.QueryUnescape(gif)
		res.Gallery = append(res.Gallery, TwitterMedia{
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

	err := c.Visit(nitter)

	if err != nil {
		return nil, err
	}

	c.Wait()

	res.URL = fmt.Sprintf("https://twitter.com/%v/status/%v", strings.TrimLeft(res.Username, "@"), res.Snowflake)

	return res, nil
}

func parse(s string) int {
	if strings.Contains(s, ",") {
		s = strings.ReplaceAll(s, ",", "")
	}

	num, _ := strconv.Atoi(s)
	return num
}
