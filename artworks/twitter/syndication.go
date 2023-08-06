package twitter

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/VTGare/boe-tea-go/artworks"
)

type twitterSyndication struct {
	twitterMatcher
	client http.Client
}

type twitterTweet struct {
	Entities      twitterEntities       `json:"entities,omitempty"`
	ID            string                `json:"id_str,omitempty"`
	Text          string                `json:"text,omitempty"`
	User          twitterUser           `json:"user,omitempty"`
	MediaDetails  []twitterMediaDetails `json:"mediaDetails,omitempty"`
	QuotedTweet   *twitterTweet         `json:"quoted_tweet,omitempty"`
	CreatedAt     time.Time             `json:"created_at,omitempty"`
	RetweetCount  int                   `json:"retweet_count,omitempty"`
	FavoriteCount int                   `json:"favorite_count,omitempty"`
	ReplyCount    int                   `json:"reply_count,omitempty"`

	Tombstone *twitterTombstone `json:"tombstone,omitempty"`
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

type twitterMediaDetails struct {
	MediaURL  string            `json:"media_url_https,omitempty"`
	Type      string            `json:"type,omitempty"`
	VideoInfo *twitterVideoInfo `json:"video_info,omitempty"`
}

type twitterVideoInfo struct {
	Variants []struct {
		Bitrate     int    `json:"bitrate,omitempty"`
		ContentType string `json:"content_type,omitempty"`
		URL         string `json:"url,omitempty"`
	} `json:"variants,omitempty"`
}

type twitterTombstone struct {
	Text struct {
		Text string `json:"text,omitempty"`
	} `json:"text,omitempty"`
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

	if resp.StatusCode != http.StatusOK {
		switch resp.StatusCode {
		case http.StatusTooManyRequests:
			return nil, artworks.ErrRateLimited
		case http.StatusNotFound:
			return nil, ErrTweetNotFound
		default:
			return nil, ErrTweetNotFound
		}
	}

	tweet := &twitterTweet{}
	if err := json.NewDecoder(resp.Body).Decode(tweet); err != nil {
		return nil, fmt.Errorf("json decode: %w", err)
	}

	if tweet.Tombstone != nil {
		text := tweet.Tombstone.Text.Text
		if strings.Contains(text, "this account owner limits who can view their Tweets") {
			return nil, ErrPrivateAccount
		}

		return nil, ErrTweetNotFound
	}

	photos, videos, err := ts.handleMediaDetails(tweet.MediaDetails)
	if err != nil {
		return nil, fmt.Errorf("handle media details: %w", err)
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

func (ts *twitterSyndication) handleMediaDetails(mds []twitterMediaDetails) ([]string, []Video, error) {
	var (
		photos []string
		videos []Video
	)

	for _, md := range mds {
		switch md.Type {
		case "video":
			video, err := ts.handleVideo(md.VideoInfo, md.MediaURL)
			if err != nil {
				return nil, nil, fmt.Errorf("handle video: %w", err)
			}

			if video.URL != "" {
				videos = append(videos, video)
			} else {
				photos = append(photos, video.Preview)
			}

		case "photo":
			photos = append(photos, md.MediaURL)
		}
	}

	return photos, videos, nil
}

func (ts *twitterSyndication) handleVideo(video *twitterVideoInfo, poster string) (Video, error) {
	sort.SliceStable(video.Variants, func(i, j int) bool {
		return video.Variants[i].Bitrate > video.Variants[j].Bitrate
	})

	for _, variant := range video.Variants {
		if variant.ContentType != "video/mp4" {
			continue
		}

		resp, err := ts.client.Head(variant.URL)
		if err != nil {
			return Video{}, fmt.Errorf("head request: %w", err)
		}

		defer resp.Body.Close()

		size := float64(resp.ContentLength) / (1 << 20)
		if size > 25 {
			continue
		}

		return Video{
			Preview: poster,
			URL:     variant.URL,
		}, nil
	}

	return Video{Preview: poster}, nil
}
