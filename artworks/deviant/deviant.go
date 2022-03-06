package deviant

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/VTGare/boe-tea-go/artworks"
	"github.com/VTGare/boe-tea-go/internal/arikawautils/embeds"
	"github.com/VTGare/boe-tea-go/internal/arrays"
	"github.com/VTGare/boe-tea-go/messages"
	"github.com/VTGare/boe-tea-go/store"
	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/discord"
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
	Favourites   int
	Comments     int
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
		regex: regexp.MustCompile(`(?i)https:\/\/(?:www\.)?deviantart\.com\/[\w]+\/art\/([\w\-]+)`),
	}
}

func (d DeviantArt) Find(id string) (artworks.Artwork, error) {
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

	return &Artwork{
		Title: res.Title,
		Author: &Author{
			Name: res.AuthorName,
			URL:  res.AuthorURL,
		},
		ImageURL:     res.URL,
		ThumbnailURL: res.ThumbnailURL,
		Tags:         strings.Split(res.Tags, ", "),
		Views:        res.Community.Statistics.Attributes.Views,
		Favourites:   res.Community.Statistics.Attributes.Favorites,
		Comments:     res.Community.Statistics.Attributes.Comments,
		CreatedAt:    res.Pubdate,
		url:          res.AuthorURL + "/art/" + id,
	}, nil
}

func (d DeviantArt) Match(s string) (string, bool) {
	res := d.regex.FindStringSubmatch(s)
	if res == nil {
		return "", false
	}

	return res[1], true
}

func (d DeviantArt) Enabled(g *store.Guild) bool {
	return g.Deviant
}

func (a Artwork) MessageSends(footer string) ([]api.SendMessageData, error) {
	eb := embeds.NewBuilder()

	eb.Title(
		fmt.Sprintf("%v by %v", a.Title, a.Author.Name),
	)
	eb.Image(a.ImageURL).URL(a.url).Timestamp(a.CreatedAt)

	tags := arrays.MapString(a.Tags, func(s string) string {
		return messages.NamedLink(
			s, "https://www.deviantart.com/tag/"+s,
		)
	})

	eb.Description("**Tags:**\n" + strings.Join(tags, " • "))

	eb.AddField(
		"Views", strconv.Itoa(a.Views), true,
	).AddField(
		"Favourites", strconv.Itoa(a.Favourites), true,
	)

	eb.Footer(footer, "")
	return []api.SendMessageData{{
		Embeds: []discord.Embed{eb.Build()},
	}}, nil
}

func (a Artwork) StoreArtwork() *store.Artwork {
	return &store.Artwork{
		Title:  a.Title,
		Author: a.Author.Name,
		URL:    a.url,
		Images: []string{a.ImageURL},
	}
}

func (a Artwork) URL() string {
	return a.url
}

func (a Artwork) Len() int {
	return 1
}
