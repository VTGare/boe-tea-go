package twitter

import (
	"fmt"
	"html"

	"github.com/VTGare/boe-tea-go/artworks"
	twitterscraper "github.com/n0madic/twitter-scraper"
)

type twitterScraper struct {
	twitterMatcher

	scraper *twitterscraper.Scraper
}

func newScraper() artworks.Provider {
	return &twitterScraper{
		twitterMatcher: twitterMatcher{},
		scraper:        twitterscraper.New(),
	}
}

func (ts *twitterScraper) Find(id string) (artworks.Artwork, error) {
	tweet, err := ts.scraper.GetTweet(id)
	if err != nil {
		return nil, fmt.Errorf("get tweet: %w", err)
	}

	profile, err := ts.scraper.GetProfile(tweet.Username)
	if err != nil {
		return nil, fmt.Errorf("get profile: %w", err)
	}

	videos := make([]Video, 0, len(tweet.Videos))
	for _, v := range tweet.Videos {
		videos = append(videos, Video{
			URL:     v.URL,
			Preview: v.Preview,
		})
	}

	art := &Artwork{
		ID:        id,
		FullName:  profile.Name,
		Username:  "@" + tweet.Username,
		Content:   html.UnescapeString(tweet.Text),
		Likes:     tweet.Likes,
		Replies:   tweet.Replies,
		Retweets:  tweet.Retweets,
		Timestamp: tweet.TimeParsed,
		Photos:    tweet.Photos,
		Videos:    videos,
		NSFW:      tweet.SensitiveContent,
		Permalink: tweet.PermanentURL,
	}

	if tweet.QuotedStatus != nil {
		art.Photos = append(art.Photos, tweet.QuotedStatus.Photos...)
	}

	return art, nil
}
