package twitter

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/VTGare/boe-tea-go/artworks"
)

type nitter struct {
	twitterMatcher
	aitagger  artworks.AITagger
	instances []string
}

func newNitter() artworks.Provider {
	return &nitter{instances: []string{
		"https://nittereu.moomoo.me",
		"https://nitter.fly.dev",
		"https://nitter.1d4.us",
		"https://notabird.site",
	}}
}

func (t *nitter) Find(snowflake string) (artworks.Artwork, error) {
	var lastError error
	for _, instance := range t.instances {
		a, err := t.scrapeTwitter(snowflake, instance)
		if err != nil {
			lastError = err
			continue
		}

		return a, nil
	}

	return nil, lastError
}

func (t *nitter) scrapeTwitter(snowflake, baseURL string) (*Artwork, error) {
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

	res := &Artwork{ID: snowflake}
	res.Content = doc.Find(".main-tweet .tweet-content").Text()
	res.FullName = doc.Find(".main-tweet .fullname").Text()
	res.Username = doc.Find(".main-tweet .username").Text()

	doc.Find(".main-tweet .still-image").Each(func(_ int, image *goquery.Selection) {
		url, _ := image.Attr("href")

		imageURL := strings.Replace(baseURL+url, baseURL+"/pic/media%2F", "https://pbs.twimg.com/media/", 1)
		res.Photos = append(res.Photos, strings.TrimSuffix(imageURL, "%3Fname%3Dorig"))
	})

	doc.Find(".main-tweet .gif").Each(func(i int, s *goquery.Selection) {
		src, _ := s.Find("source").Attr("src")
		poster, _ := s.Attr("poster")
		// https://pbs.twimg.com/tweet_video_thumb/FtsBwmIXoAIaOeu.jpg

		var (
			preview = strings.Replace(poster, "/pic/", "https://pbs.twimg.com/", 1)
			gif     = strings.Replace(src, "/pic/", "https://", 1)
		)

		gif, _ = url.QueryUnescape(gif)
		preview, _ = url.QueryUnescape(preview)

		res.Videos = append(res.Videos, Video{
			URL:     gif,
			Preview: preview,
		})
	})

	res.Likes = parseCount(doc.Find(".main-tweet .icon-container").Has(".icon-heart").Text())
	res.Retweets = parseCount(doc.Find(".main-tweet .icon-container").Has(".icon-retweet").Text())
	res.Replies = parseCount(doc.Find(".main-tweet .icon-container").Has(".icon-comment").Text())

	date, _ := doc.Find(".main-tweet .tweet-date").Find("a").Attr("title")
	ts, _ := time.Parse("Jan 2, 2006 · 3:04 PM UTC", date)
	res.Timestamp = ts

	username := ""
	if res.Username == "" {
		username = "i"
	} else {
		username = strings.TrimLeft(res.Username, "@")
	}

	res.Permalink = fmt.Sprintf("https://twitter.com/%v/status/%v", username, res.ID)
	res.AIGenerated = t.aitagger.AITag([]string{res.Content})
	return res, nil
}

func parseCount(s string) int {
	s = strings.ReplaceAll(strings.TrimSpace(s), ",", "")

	num, _ := strconv.Atoi(s)
	return num
}
