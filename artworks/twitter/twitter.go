package twitter

import (
	"fmt"
	"time"

	"github.com/VTGare/boe-tea-go/artworks"
	"github.com/VTGare/boe-tea-go/store"

	"github.com/bwmarrin/discordgo"
)

type Twitter struct {
	twitterMatcher
	providers []artworks.Provider
}

type Artwork struct {
	Videos      []Video
	Photos      []string
	ID          string
	FullName    string
	Username    string
	Content     string
	Permalink   string
	Timestamp   time.Time
	Likes       int
	Replies     int
	Retweets    int
	NSFW        bool
	AIGenerated bool
}

type Video struct {
	URL     string
	Preview string
}

func New() artworks.Provider {
	return &Twitter{
		providers: []artworks.Provider{newNitter(), newFxTwitter(), newScraper()},
	}
}

func (t *Twitter) Find(id string) (artworks.Artwork, error) {
	return &Artwork{ID: id, Permalink: fmt.Sprintf("https://twitter.com/i/status/%v", id)}, nil

	// var (
	// 	artwork artworks.Artwork
	// 	errs    []error
	// )

	// for _, provider := range t.providers {
	// 	var err error
	// 	artwork, err = provider.Find(id)
	// 	if err != nil {
	// 		errs = append(errs, err)
	// 		continue
	// 	}

	// 	tweet := artwork.(*Artwork)
	// 	if tweet.Username == "" {
	// 		continue
	// 	}

	// 	return artwork, nil
	// }

	// return &Artwork{}, errors.Join(errs...)
}

func (artwork *Artwork) StoreArtwork() *store.Artwork {
	media := make([]string, 0, len(artwork.Photos)+len(artwork.Videos))

	media = append(media, artwork.Photos...)
	for _, video := range artwork.Videos {
		media = append(media, video.Preview)
	}

	return &store.Artwork{
		Author: artwork.Username,
		URL:    artwork.Permalink,
		Images: media,
	}
}

// MessageSends transforms an artwork to discordgo embeds.
func (a *Artwork) MessageSends(footer string, _ bool) ([]*discordgo.MessageSend, error) {
	// Temporary return an empty array while Twitter API doesn't work.
	return []*discordgo.MessageSend{}, nil

	// eb := embeds.NewBuilder()
	// if a.Username == "" && a.Len() == 0 {
	// 	eb.Title("❎ Tweet doesn't exist.")
	// 	eb.Description("Twitter API doesn't respond or the tweet has been deleted.\n\nLately unsafe tweets may appear as deleted, I'm looking for a workaround!")
	// 	eb.Footer(footer, "")

	// 	return []*discordgo.MessageSend{
	// 		{Embeds: []*discordgo.MessageEmbed{eb.Finalize()}},
	// 	}, nil
	// }

	// eb.URL(a.Permalink).Description(a.Content).Timestamp(a.Timestamp)
	// eb.AddField("Retweets", strconv.Itoa(a.Retweets), true)
	// eb.AddField("Likes", strconv.Itoa(a.Likes), true)

	// if footer != "" {
	// 	eb.Footer(footer, "")
	// }

	// if a.AIGenerated {
	// 	eb.AddField("⚠️ Disclaimer", "This artwork is AI-generated.")
	// }

	// if len(a.Videos) > 0 {
	// 	video := a.Videos[0]

	// 	resp, err := http.Get(video.URL)
	// 	if err != nil {
	// 		return nil, fmt.Errorf("error downloading twitter video: %w", err)
	// 	}
	// 	defer resp.Body.Close()

	// 	b, err := io.ReadAll(resp.Body)
	// 	if err != nil {
	// 		return nil, fmt.Errorf("error reading twitter video: %w", err)
	// 	}

	// 	uri, err := url.Parse(video.URL)
	// 	if err != nil {
	// 		return nil, err
	// 	}

	// 	splits := strings.Split(uri.Path, "/")

	// 	eb.Title(fmt.Sprintf("%v (%v)", a.FullName, a.Username))
	// 	msg := &discordgo.MessageSend{
	// 		Embeds: []*discordgo.MessageEmbed{eb.Finalize()},
	// 		Files: []*discordgo.File{
	// 			{
	// 				Name:   splits[len(splits)-1],
	// 				Reader: bytes.NewReader(b),
	// 			},
	// 		},
	// 	}

	// 	return []*discordgo.MessageSend{msg}, nil
	// }

	// length := len(a.Photos)
	// tweets := make([]*discordgo.MessageSend, 0, length)
	// if length > 1 {
	// 	eb.Title(fmt.Sprintf("%v (%v) | Page %v / %v", a.FullName, a.Username, 1, length))
	// } else {
	// 	eb.Title(fmt.Sprintf("%v (%v)", a.FullName, a.Username))
	// }

	// if length > 0 {
	// 	eb.Image(a.Photos[0])
	// }

	// tweets = append(tweets, &discordgo.MessageSend{
	// 	Embeds: []*discordgo.MessageEmbed{eb.Finalize()},
	// })

	// if len(a.Photos) > 1 {
	// 	for ind, photo := range a.Photos[1:] {
	// 		eb := embeds.NewBuilder()

	// 		eb.Title(fmt.Sprintf("%v (%v) | Page %v / %v", a.FullName, a.Username, ind+2, length)).URL(a.Permalink)
	// 		eb.Image(photo).Timestamp(a.Timestamp)

	// 		if footer != "" {
	// 			eb.Footer(footer, "")
	// 		}

	// 		tweets = append(tweets, &discordgo.MessageSend{Embeds: []*discordgo.MessageEmbed{eb.Finalize()}})
	// 	}
	// }

	// return tweets, nil
}

func (a *Artwork) URL() string {
	return a.Permalink
}

func (a *Artwork) Len() int {
	if len(a.Videos) != 0 {
		return 1
	}

	return len(a.Photos)
}
