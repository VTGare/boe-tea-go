package twitter

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/VTGare/boe-tea-go/artworks"
	"github.com/VTGare/boe-tea-go/internal/spool"
	"github.com/VTGare/boe-tea-go/store"

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
	id          string
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
		twitterMatcher: twitterMatcher{
			regex: regexp.MustCompile(`^(?:mobile\.)?(?:(?:fix(?:up|v))?x|(?:[fv]x)?twitter)\.com$`),
		},
	}
}

func (t *Twitter) Find(id string) (artworks.Artwork, error) {
	return artworks.WrapError(t, func() (artworks.Artwork, error) {
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

// Render returns the artwork as data for the render module.
func (a *Artwork) Render() (artworks.Rendered, error) {
	if a.FullName == "" && a.Len() == 0 {
		return artworks.Rendered{
			Title:       "❎ Tweet doesn't exist.",
			Description: "The tweet is NSFW or doesn't exist.\n\nUnsafe tweets can't be embedded due to API changes.",
		}, nil
	}

	rendered := artworks.Rendered{
		Title:       fmt.Sprintf("%v (%v)", a.FullName, a.Username),
		URL:         a.Permalink,
		Timestamp:   a.Timestamp,
		Description: artworks.EscapeMarkdown(a.Content),
		AIGenerated: a.AIGenerated,
	}

	if a.Retweets > 0 {
		rendered.Fields = append(rendered.Fields, artworks.RenderedField{
			Name:   "Retweets",
			Value:  strconv.Itoa(a.Retweets),
			Inline: true,
		})
	}

	if a.Likes > 0 {
		rendered.Fields = append(rendered.Fields, artworks.RenderedField{
			Name:   "Likes",
			Value:  strconv.Itoa(a.Likes),
			Inline: true,
		})
	}

	if len(a.Videos) > 0 {
		files := make([]*discordgo.File, 0, len(a.Videos))
		for _, video := range a.Videos {
			file, err := downloadVideo(video.URL)
			if err != nil {
				spool.RemoveFiles(files)

				return artworks.Rendered{}, err
			}

			files = append(files, file)
		}

		rendered.Files = files

		return rendered, nil
	}

	for _, photo := range a.Photos {
		rendered.Images = append(rendered.Images, artworks.RenderedImage{Preview: photo})
	}

	return rendered, nil
}

func (a *Artwork) ID() string {
	return a.id
}

func downloadVideo(fileURL string) (*discordgo.File, error) {
	spool.Acquire()
	defer spool.Release()

	resp, err := http.Get(fileURL)
	if err != nil {
		return nil, fmt.Errorf("error downloading twitter video: %w", err)
	}
	defer resp.Body.Close()

	uri, err := url.Parse(fileURL)
	if err != nil {
		return nil, err
	}

	splits := strings.Split(uri.Path, "/")

	tmp, err := spool.Download("bt-video-*.mp4", resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error spooling twitter video: %w", err)
	}

	return &discordgo.File{
		Name:   splits[len(splits)-1],
		Reader: tmp,
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
