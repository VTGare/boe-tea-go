package twitter

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/VTGare/boe-tea-go/artworks/embed"
	"io"
	"net/http"
	"net/url"
	"regexp"
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
	re := regexp.MustCompile(`^(?:mobile\.)?(?:(?:fix(?:up|v))?x|(?:[fv]x)?twitter)\.com$`)

	return &Twitter{
		providers: []artworks.Provider{newFxTwitter(re)},
		twitterMatcher: twitterMatcher{
			regex: re,
		},
	}
}

func (t *Twitter) Find(id string) (artworks.Artwork, error) {
	return artworks.NewError(t, func() (artworks.Artwork, error) {
		var (
			artwork artworks.Artwork
			errs    []error
		)

		for _, provider := range t.providers {
			var err error
			artwork, err = provider.Find(id)
			if errors.Is(err, ErrTweetNotFound) || errors.Is(err, ErrPrivateAccount) {
				return nil, err
			}

			if err != nil {
				errs = append(errs, err)
				continue
			}

			return artwork, nil
		}

		return &Artwork{}, errors.Join(errs...)
	})
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
	if a.FullName == "" && a.Len() == 0 {
		eb := embeds.NewBuilder()
		eb.Title("❎ Tweet doesn't exist.")
		eb.Description("The tweet is NSFW or doesn't exist.\n\nUnsafe tweets can't be embedded due to API changes.")
		eb.Footer(footer, "")

		return []*discordgo.MessageSend{
			{Embeds: []*discordgo.MessageEmbed{eb.Finalize()}},
		}, nil
	}

	eb := &embed.Embed{
		Title:       a.FullName,
		Username:    a.Username,
		Description: a.Content,
		FieldName1:  "Likes",
		FieldValue1: strconv.Itoa(a.Likes),
		FieldName2:  "Retweets",
		FieldValue2: []string{strconv.Itoa(a.Retweets)},
		URL:         a.Permalink,
		Timestamp:   a.Timestamp,
		AIGenerated: a.AIGenerated,
	}

	if len(a.Videos) > 0 {
		return a.videoEmbed(eb)
	}

	for _, image := range a.Photos {
		eb.Images = append(eb.Images, image)
	}

	return eb.ToEmbed(), nil
}

func (a *Artwork) videoEmbed(eb *embed.Embed) ([]*discordgo.MessageSend, error) {
	for _, video := range a.Videos {
		file, err := downloadVideo(video.URL)
		if err != nil {
			return nil, err
		}

		eb.Files = append(eb.Files, file)
	}

	return eb.ToEmbed(), nil
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
