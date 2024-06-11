package twitter

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/VTGare/boe-tea-go/artworks"
	"github.com/VTGare/boe-tea-go/internal/arrays"
)

type fxTwitter struct {
	twitterMatcher
	client               *http.Client
	nonAlphanumericRegex *regexp.Regexp
}

type fxTwitterResponse struct {
	Code    int    `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
	Tweet   struct {
		URL    string `json:"url,omitempty"`
		ID     string `json:"id,omitempty"`
		Text   string `json:"text,omitempty"`
		Author struct {
			Name       string `json:"name,omitempty"`
			ScreenName string `json:"screen_name,omitempty"`
			AvatarURL  string `json:"avatar_url,omitempty"`
		} `json:"author,omitempty"`
		Replies          int   `json:"replies,omitempty"`
		Retweets         int   `json:"retweets,omitempty"`
		Likes            int   `json:"likes,omitempty"`
		CreatedTimestamp int64 `json:"created_timestamp,omitempty"`
		Media            struct {
			Photos []struct {
				Type string `json:"type,omitempty"`
				URL  string `json:"url,omitempty"`
			} `json:"photos,omitempty"`
			Videos []struct {
				Type         string `json:"type,omitempty"`
				URL          string `json:"url,omitempty"`
				ThumbnailURL string `json:"thumbnail_url,omitempty"`
				Variants     []struct {
					Bitrate     int    `json:"bitrate,omitempty"`
					ContentType string `json:"content_type,omitempty"`
					URL         string `json:"url,omitempty"`
				}
			} `json:"videos,omitempty"`
		} `json:"media,omitempty"`
	} `json:"tweet,omitempty"`
}

func newFxTwitter(re *regexp.Regexp) artworks.Provider {
	return &fxTwitter{
		client:               &http.Client{},
		nonAlphanumericRegex: regexp.MustCompile(`[^\p{L}\p{N} -]+`),
		twitterMatcher: twitterMatcher{
			regex: re,
		},
	}
}

func (fxt *fxTwitter) Find(id string) (artworks.Artwork, error) {
	url := fmt.Sprintf("https://api.fxtwitter.com/i/status/%v", id)

	resp, err := fxt.client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("http get: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		break
	case http.StatusNotFound:
		return nil, ErrTweetNotFound
	default:
		return nil, fmt.Errorf("unexpected response status: %v", resp.Status)
	}

	fxArtwork := &fxTwitterResponse{}
	if err := json.NewDecoder(resp.Body).Decode(fxArtwork); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}

	videos := make([]Video, 0, len(fxArtwork.Tweet.Media.Videos))
	for _, v := range fxArtwork.Tweet.Media.Videos {
		videoURL := v.URL // default to highest quality url

		// if at least 3 variants exist, pick second best quality to save bandwidth. the slice is sorted by bitrate by default.
		// first variant is always in m3u streaming format, so we need at least 3 variants to get this.
		if len(v.Variants) > 2 {
			secondBest := v.Variants[len(v.Variants)-2]
			videoURL = secondBest.URL
		}

		videos = append(videos, Video{
			Preview: v.ThumbnailURL,
			URL:     videoURL,
		})
	}

	photos := make([]string, 0, len(fxArtwork.Tweet.Media.Photos))
	for _, p := range fxArtwork.Tweet.Media.Photos {
		photos = append(photos, p.URL)
	}

	var username string
	if fxArtwork.Tweet.Author.Name != "" {
		username = "@" + fxArtwork.Tweet.Author.ScreenName
	}

	artwork := &Artwork{
		Videos:    videos,
		Photos:    photos,
		ID:        fxArtwork.Tweet.ID,
		FullName:  fxArtwork.Tweet.Author.Name,
		Username:  username,
		Content:   fxArtwork.Tweet.Text,
		Permalink: fxArtwork.Tweet.URL,
		Timestamp: time.Unix(fxArtwork.Tweet.CreatedTimestamp, 0),
		Likes:     fxArtwork.Tweet.Likes,
		Replies:   fxArtwork.Tweet.Replies,
		Retweets:  fxArtwork.Tweet.Retweets,
		NSFW:      true,
	}

	artwork.AIGenerated = artworks.IsAIGenerated(arrays.Map(strings.Fields(artwork.Content), func(s string) string {
		return fxt.nonAlphanumericRegex.ReplaceAllString(s, "")
	})...)

	return artwork, nil
}
