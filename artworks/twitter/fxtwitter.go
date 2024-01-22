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

var nonAlphanumericRegex = regexp.MustCompile(`[^\p{L}\p{N} -]+`)

type fxTwitter struct {
	twitterMatcher
	client *http.Client
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
			} `json:"videos,omitempty"`
		} `json:"media,omitempty"`
	} `json:"tweet,omitempty"`
}

func newFxTwitter() artworks.Provider {
	return &fxTwitter{
		twitterMatcher: twitterMatcher{},
		client:         &http.Client{},
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
		videos = append(videos, Video{
			Preview: v.ThumbnailURL,
			URL:     v.URL,
		})
	}

	photos := make([]string, 0, len(fxArtwork.Tweet.Media.Photos))
	for _, p := range fxArtwork.Tweet.Media.Photos {
		photos = append(photos, p.URL)
	}

	var username string
	if fxArtwork.Tweet.Author.Name != "" {
		username = "@" + fxArtwork.Tweet.Author.Name
	}

	artwork := &Artwork{
		Videos:    videos,
		Photos:    photos,
		ID:        fxArtwork.Tweet.ID,
		FullName:  fxArtwork.Tweet.Author.ScreenName,
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
		return nonAlphanumericRegex.ReplaceAllString(s, "")
	})...)

	return artwork, nil
}
