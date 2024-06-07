package deviant

import (
	"encoding/json"
	"github.com/VTGare/boe-tea-go/artworks/embed"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/VTGare/boe-tea-go/artworks"
	"github.com/VTGare/boe-tea-go/internal/arrays"
	"github.com/VTGare/boe-tea-go/messages"
	"github.com/VTGare/boe-tea-go/store"
	"github.com/bwmarrin/discordgo"
)

type DeviantArt struct {
	regex *regexp.Regexp
}

type Artwork struct {
	Title        string
	Author       *Author
	ImageURL     string
	ThumbnailURL string
	Tags         []string
	Views        int
	Favorites    int
	Comments     int
	AIGenerated  bool
	CreatedAt    time.Time
	url          string
}

type Author struct {
	Name string
	URL  string
}

type deviantEmbed struct {
	Title        string    `json:"title,omitempty"`
	Category     string    `json:"category,omitempty"`
	URL          string    `json:"url,omitempty"`
	ThumbnailURL string    `json:"thumbnail_url,omitempty"`
	AuthorName   string    `json:"author_name,omitempty"`
	AuthorURL    string    `json:"author_url,omitempty"`
	Safety       string    `json:"safety,omitempty"`
	Pubdate      time.Time `json:"pubdate,omitempty"`
	Community    struct {
		Statistics struct {
			Attributes struct {
				Views     int `json:"views,omitempty"`
				Favorites int `json:"favorites,omitempty"`
				Comments  int `json:"comments,omitempty"`
				Downloads int `json:"downloads,omitempty"`
			} `json:"_attributes,omitempty"`
		} `json:"statistics,omitempty"`
	} `json:"community,omitempty"`
	Tags string `json:"tags,omitempty"`
}

func New() artworks.Provider {
	return &DeviantArt{
		regex: regexp.MustCompile(`(?i)https://(?:www\.)?deviantart\.com/[\w]+/art/([\w\-]+)`),
	}
}

func (d *DeviantArt) Find(id string) (artworks.Artwork, error) {
	return artworks.NewError(d, func() (artworks.Artwork, error) {
		reqURL := "https://backend.deviantart.com/oembed?url=" + url.QueryEscape("deviantart.com/art/"+id)
		resp, err := http.Get(reqURL)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		var res deviantEmbed
		err = json.NewDecoder(resp.Body).Decode(&res)
		if err != nil {
			return nil, err
		}

		artwork := &Artwork{
			Title: res.Title,
			Author: &Author{
				Name: res.AuthorName,
				URL:  res.AuthorURL,
			},
			ImageURL:     res.URL,
			ThumbnailURL: res.ThumbnailURL,
			Tags:         strings.Split(res.Tags, ", "),
			Views:        res.Community.Statistics.Attributes.Views,
			Favorites:    res.Community.Statistics.Attributes.Favorites,
			Comments:     res.Community.Statistics.Attributes.Comments,
			CreatedAt:    res.Pubdate,
			url:          res.AuthorURL + "/art/" + id,
		}

		artwork.AIGenerated = artworks.IsAIGenerated(artwork.Tags...)

		return artwork, nil
	})
}

func (d *DeviantArt) Match(s string) (string, bool) {
	res := d.regex.FindStringSubmatch(s)
	if res == nil {
		return "", false
	}

	return res[1], true
}

func (d *DeviantArt) Enabled(g *store.Guild) bool {
	return g.Deviant
}

func (a *Artwork) MessageSends(footer string, tagsEnabled bool) ([]*discordgo.MessageSend, error) {
	eb := &embed.Embed{
		Title:       a.Title,
		Username:    a.Author.Name,
		FieldName1:  "Views",
		FieldValue1: strconv.Itoa(a.Views),
		FieldName2:  "Favorites",
		FieldValue2: []string{strconv.Itoa(a.Favorites)},
		Images:      []string{a.ImageURL},
		URL:         a.url,
		Timestamp:   a.CreatedAt,
		Footer:      footer,
		AIGenerated: a.AIGenerated,
	}

	if tagsEnabled && len(a.Tags) > 0 {
		tags := arrays.Map(a.Tags, func(s string) string {
			return messages.NamedLink(s, "https://www.deviantart.com/tag/"+s)
		})
		eb.Tags = strings.Join(tags, " • ")
	}

	return eb.ToEmbed(), nil
}

func (a *Artwork) StoreArtwork() *store.Artwork {
	return &store.Artwork{
		Title:  a.Title,
		Author: a.Author.Name,
		URL:    a.url,
		Images: []string{a.ImageURL},
	}
}

func (a *Artwork) URL() string {
	return a.url
}

func (a *Artwork) Len() int {
	return 1
}
