package services

import (
	"errors"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gocolly/colly/v2"
)

var (
	TwitterRegex = regexp.MustCompile(`https?://twitter.com/(\S+)/status/(\d+)`)
)

type Tweet struct {
	Author    string
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
	if !TwitterRegex.MatchString(uri) {
		return nil, errors.New("invalid twitter url")
	}

	uri = strings.ReplaceAll(uri, "twitter.com", "nitter.net")
	c := colly.NewCollector()
	res := &Tweet{
		Gallery: make([]TwitterMedia, 0),
	}

	c.OnHTML(".main-tweet .still-image", func(e *colly.HTMLElement) {
		escapedLink := strings.TrimPrefix(e.Attr("href"), `/pic/`)
		imageURL, _ := url.QueryUnescape(escapedLink)
		res.Gallery = append(res.Gallery, TwitterMedia{
			URL:      imageURL,
			Animated: false,
		})
	})

	c.OnHTML(".main-tweet .gif", func(e *colly.HTMLElement) {
		escapedLink := strings.TrimPrefix(e.ChildAttr("source", "src"), "/gif/")
		imageURL, _ := url.QueryUnescape(escapedLink)
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
		res.Author = e.Text
	})

	err := c.Visit(uri)

	if err != nil {
		return nil, err
	}

	c.Wait()
	return res, nil
}
