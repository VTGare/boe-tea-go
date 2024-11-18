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
	"github.com/VTGare/boe-tea-go/internal/arrays"
	"github.com/VTGare/boe-tea-go/messages"
	"github.com/VTGare/boe-tea-go/store"
	"github.com/VTGare/embeds"
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

	id  string
	url string
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
		regex: regexp.MustCompile(`(?i)https?://(?:www\.)?deviantart\.com/(\w.+)/art/(\w.+)`),
	}
}

func (d *DeviantArt) Find(id string) (artworks.Artwork, error) {
	artwork, err := d._find(id)
	if err != nil {
		return nil, artworks.NewError(d, err)
	}

	return artwork, nil
}

func (*DeviantArt) _find(id string) (artworks.Artwork, error) {
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

		id:  id,
		url: res.AuthorURL + "/art/" + id,
	}

	artwork.AIGenerated = artworks.IsAIGenerated(artwork.Tags...)

	return artwork, nil
}

func (d *DeviantArt) Match(s string) (string, bool) {
	res := d.regex.FindStringSubmatch(s)
	if res == nil {
		return "", false
	}

	return res[1], true
}

func (*DeviantArt) Enabled(g *store.Guild) bool {
	return g.Deviant
}

func (a *Artwork) MessageSends(footer string, tagsEnabled bool) ([]*discordgo.MessageSend, error) {
	eb := embeds.NewBuilder()

	eb.Title(fmt.Sprintf("%v by %v", a.Title, a.Author.Name)).
		Image(a.ImageURL).
		URL(a.url).
		Timestamp(a.CreatedAt).
		AddField("Views", strconv.Itoa(a.Views), true).
		AddField("Favorites", strconv.Itoa(a.Favorites), true)

	if tagsEnabled && len(a.Tags) > 0 {
		tags := arrays.Map(a.Tags, func(s string) string {
			return messages.NamedLink(
				s, "https://www.deviantart.com/tag/"+s,
			)
		})

		eb.Description("**Tags:**\n" + strings.Join(tags, " • "))
	}

	if footer != "" {
		eb.Footer(footer, "")
	}

	if a.AIGenerated {
		eb.AddField("⚠️ Disclaimer", "This artwork is AI-generated.")
	}

	return []*discordgo.MessageSend{
		{Embeds: []*discordgo.MessageEmbed{eb.Finalize()}},
	}, nil
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

func (a *Artwork) ID() string {
	return a.id
}

func (*Artwork) Len() int {
	return 1
}
