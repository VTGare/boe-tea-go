package twitter

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/VTGare/boe-tea-go/artworks"
)

type twitterSyndication struct {
	twitterMatcher
	client http.Client
}

type twitterTweet struct {
	Entities      twitterEntities `json:"entities,omitempty"`
	ID            string          `json:"id_str,omitempty"`
	Text          string          `json:"text,omitempty"`
	User          twitterUser     `json:"user,omitempty"`
	Photos        []twitterPhoto  `json:"photos,omitempty"`
	Video         *twitterVideo   `json:"video,omitempty"`
	QuotedTweet   *twitterTweet   `json:"quoted_tweet,omitempty"`
	CreatedAt     time.Time       `json:"created_at,omitempty"`
	RetweetCount  int             `json:"retweet_count,omitempty"`
	FavoriteCount int             `json:"favorite_count,omitempty"`
	ReplyCount    int             `json:"reply_count,omitempty"`
}

type twitterEntities struct {
	Hashtags []struct {
		Text string
	}
}

type twitterUser struct {
	ID           string `json:"id_str,omitempty"`
	Name         string `json:"name,omitempty"`
	ProfileImage string `json:"profile_image_url_https,omitempty"`
	ScreenName   string `json:"screen_name,omitempty"`
}

type twitterPhoto struct {
	URL string
}

type twitterVideo struct {
	Poster   string
	Variants []struct {
		Src string
	}
}

func newSyndication() artworks.Provider {
	return &twitterSyndication{
		twitterMatcher: twitterMatcher{},
		client:         http.Client{},
	}
}

func (ts *twitterSyndication) Find(id string) (artworks.Artwork, error) {
	url := fmt.Sprintf("https://cdn.syndication.twimg.com/tweet-result?id=%v&lang=en", id)
	resp, err := ts.client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("http get: %w", err)
	}

	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusNotFound:
		fallthrough
	case http.StatusTooManyRequests:
		return &Artwork{}, nil
	}

	tweet := &twitterTweet{}
	if err := json.NewDecoder(resp.Body).Decode(tweet); err != nil {
		return nil, fmt.Errorf("json decode: %w", err)
	}

	photos := make([]string, 0, len(tweet.Photos))
	for _, photo := range tweet.Photos {
		photos = append(photos, photo.URL)
	}

	videos := []Video{}
	if tweet.Video != nil && len(tweet.Video.Variants) >= 2 {
		videos = append(videos, Video{
			Preview: tweet.Video.Poster,
			URL:     tweet.Video.Variants[1].Src,
		})
	}

	art := &Artwork{
		ID:        tweet.ID,
		FullName:  tweet.User.Name,
		Username:  "@" + tweet.User.ScreenName,
		Content:   tweet.Text,
		Likes:     tweet.FavoriteCount,
		Replies:   tweet.ReplyCount,
		Retweets:  tweet.RetweetCount,
		Timestamp: tweet.CreatedAt,
		Photos:    photos,
		Videos:    videos,
		NSFW:      false,
		Permalink: fmt.Sprintf("https://twitter.com/%v/status/%v", tweet.User.ScreenName, tweet.ID),
	}

	hashtags := make([]string, 0, len(tweet.Entities.Hashtags))
	for _, hashtag := range tweet.Entities.Hashtags {
		hashtags = append(hashtags, hashtag.Text)
	}

	art.AIGenerated = artworks.IsAIGenerated(hashtags...)
	return art, nil
}
