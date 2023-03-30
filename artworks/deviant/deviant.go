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
    artworks.ProvBase
}

type Artwork struct {
    artworks.ArtBase
	Title        string
	Author       *Author
	ImageURL     string
	ThumbnailURL string
	Tags         []string
	Views        int
	Favorites    int
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
    prov := artworks.ProvBase{}
    prov.Regex = regexp.MustCompile(`(?i)https:\/\/(?:www\.)?deviantart\.com\/[\w]+\/art\/([\w\-]+)`)
	return &DeviantArt{prov}
}

func (d *DeviantArt) Find(id string) (artworks.Artwork, error) {
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
		Favorites:    res.Community.Statistics.Attributes.Favorites,
		Comments:     res.Community.Statistics.Attributes.Comments,
		CreatedAt:    res.Pubdate,
		url:          res.AuthorURL + "/art/" + id,
	}, nil
}

func (d *DeviantArt) Enabled(g *store.Guild) bool {
	return g.Deviant
}

func (a *Artwork) MessageSends(footer string, hasTags bool) ([]*discordgo.MessageSend, error) {
	eb := embeds.NewBuilder()

	eb.Title(fmt.Sprintf("%v by %v", a.Title, a.Author.Name)).
		Image(a.ImageURL).
		URL(a.url).
		Timestamp(a.CreatedAt).
		AddField("Views", strconv.Itoa(a.Views), true).
		AddField("Favorites", strconv.Itoa(a.Favorites), true)

	if hasTags {
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

	return []*discordgo.MessageSend{
		{Embeds: []*discordgo.MessageEmbed{eb.Finalize()}},
	}, nil
}

func (a *Artwork) GetTitle() string { return a.Title }
func (a *Artwork) GetAuthor() string { return a.Author.Name }
func (a *Artwork) GetURL() string { return a.url }
func (a *Artwork) GetImages() []string { return []string{a.ImageURL} }
func (a *Artwork) Len() int { return 1 }
