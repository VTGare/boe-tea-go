package twitter

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/VTGare/boe-tea-go/artworks"
	"github.com/VTGare/boe-tea-go/store"
	"github.com/VTGare/embeds"

	"github.com/bwmarrin/discordgo"
)

// Common Twitter errors
var (
	ErrTweetNotFound  = errors.New("tweet not found")
	ErrPrivateAccount = errors.New("unable to view this tweet because account is private")
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
		providers: []artworks.Provider{newFxTwitter()},
	}
}

func (t *Twitter) Find(id string) (artworks.Artwork, error) {
	var (
		artwork artworks.Artwork
		errs    []error
	)

	for _, provider := range t.providers {
		var err error
		artwork, err = provider.Find(id)
		if errors.Is(err, ErrTweetNotFound) || errors.Is(err, ErrPrivateAccount) {
			return nil, artworks.NewError(t, err)
		}

		if err != nil {
			errs = append(errs, err)
			continue
		}

		return artwork, nil
	}

	return &Artwork{}, artworks.NewError(t, errors.Join(errs...))
}

func (a *Artwork) StoreArtwork() *store.Artwork {
	media := make([]string, 0, len(a.Photos)+len(a.Videos))

	media = append(media, a.Photos...)
	for _, video := range a.Videos {
		media = append(media, video.Preview)
	}

	return &store.Artwork{
		Author: a.Username,
		URL:    a.Permalink,
		Images: media,
	}
}

// MessageSends transforms an artwork to discordgo embeds.
func (a *Artwork) MessageSends(footer string, _ bool) ([]*discordgo.MessageSend, error) {
	eb := embeds.NewBuilder()
	if a.FullName == "" && a.Len() == 0 {
		eb.Title("❎ Tweet doesn't exist.")
		eb.Description("The tweet is NSFW or doesn't exist.\n\nUnsafe tweets can't be embedded due to API changes.")
		eb.Footer(footer, "")

		return []*discordgo.MessageSend{
			{Embeds: []*discordgo.MessageEmbed{eb.Finalize()}},
		}, nil
	}

	eb.URL(a.Permalink).Description(artworks.EscapeMarkdown(a.Content)).Timestamp(a.Timestamp)

	if a.Retweets > 0 {
		eb.AddField("Retweets", strconv.Itoa(a.Retweets), true)
	}

	if a.Likes > 0 {
		eb.AddField("Likes", strconv.Itoa(a.Likes), true)
	}

	if footer != "" {
		eb.Footer(footer, "")
	}

	if a.AIGenerated {
		eb.AddField("⚠️ Disclaimer", "This artwork is AI-generated.")
	}

	if len(a.Videos) > 0 {
		return a.videoEmbed(eb)
	}

	length := len(a.Photos)
	tweets := make([]*discordgo.MessageSend, 0, length)
	if length > 1 {
		eb.Title(fmt.Sprintf("%v (%v) | Page %v / %v", a.FullName, a.Username, 1, length))
	} else {
		eb.Title(fmt.Sprintf("%v (%v)", a.FullName, a.Username))
	}

	if length > 0 {
		eb.Image(a.Photos[0])
	}

	tweets = append(tweets, &discordgo.MessageSend{
		Embeds: []*discordgo.MessageEmbed{eb.Finalize()},
	})

	if len(a.Photos) > 1 {
		for ind, photo := range a.Photos[1:] {
			eb := embeds.NewBuilder()

			eb.Title(fmt.Sprintf("%v (%v) | Page %v / %v", a.FullName, a.Username, ind+2, length)).URL(a.Permalink)
			eb.Image(photo).Timestamp(a.Timestamp)

			if footer != "" {
				eb.Footer(footer, "")
			}

			tweets = append(tweets, &discordgo.MessageSend{Embeds: []*discordgo.MessageEmbed{eb.Finalize()}})
		}
	}

	return tweets, nil
}

func (a *Artwork) videoEmbed(eb *embeds.Builder) ([]*discordgo.MessageSend, error) {
	files := make([]*discordgo.File, 0, len(a.Videos))
	for _, video := range a.Videos {
		file, err := downloadVideo(video.URL)
		if err != nil {
			return nil, err
		}

		files = append(files, file)
	}

	eb.Title(fmt.Sprintf("%v (%v)", a.FullName, a.Username))
	msg := &discordgo.MessageSend{
		Embeds: []*discordgo.MessageEmbed{eb.Finalize()},
		Files:  files,
	}

	return []*discordgo.MessageSend{msg}, nil
}

func downloadVideo(fileURL string) (*discordgo.File, error) {
	resp, err := http.Get(fileURL)
	if err != nil {
		return nil, fmt.Errorf("error downloading twitter video: %w", err)
	}

	defer resp.Body.Close()

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading twitter video: %w", err)
	}

	uri, err := url.Parse(fileURL)
	if err != nil {
		return nil, err
	}

	splits := strings.Split(uri.Path, "/")

	return &discordgo.File{
		Name:   splits[len(splits)-1],
		Reader: bytes.NewReader(b),
	}, nil
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
