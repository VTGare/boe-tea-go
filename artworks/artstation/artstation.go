package artstation

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"time"

	"github.com/VTGare/boe-tea-go/artworks"
	"github.com/VTGare/boe-tea-go/internal/arikawautils/embeds"
	"github.com/VTGare/boe-tea-go/store"
	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/microcosm-cc/bluemonday"
	"go.uber.org/atomic"
)

type Artstation struct {
	regex *regexp.Regexp
	hits  atomic.Int64
}

type ArtstationResponse struct {
	Title       string   `json:"title,omitempty"`
	Description string   `json:"description,omitempty"`
	Permalink   string   `json:"permalink,omitempty"`
	CoverURL    string   `json:"cover_url,omitempty"`
	HashID      string   `json:"hash_id,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	Assets      []*Asset `json:"assets,omitempty"`
	User        *User    `json:"user,omitempty"`

	Medium     *Category   `json:"medium,omitempty"`
	Mediums    []*Category `json:"mediums,omitempty"`
	Categories []*Category `json:"categories,omitempty"`

	ViewsCount    int `json:"views_count,omitempty"`
	LikesCount    int `json:"likes_count,omitempty"`
	CommentsCount int `json:"comments_count,omitempty"`

	HideAsAdult         bool `json:"hide_as_adult,omitempty"`
	VisibleOnArtstation bool `json:"visible_on_artstation,omitempty"`

	CreatedAt time.Time `json:"created_at,omitempty"`
}

type Asset struct {
	Title             string `json:"title,omitempty"`
	TitleFormatted    string `json:"title_formatted,omitempty"`
	ImageURL          string `json:"image_url,omitempty"`
	Width             int    `json:"width,omitempty"`
	Height            int    `json:"height,omitempty"`
	AssetType         string `json:"asset_type,omitempty"`
	HasImage          bool   `json:"has_image,omitempty"`
	HasEmbeddedPlayer bool   `json:"has_embedded_player,omitempty"`
}

type User struct {
	Name            string `json:"username,omitempty"`
	Headline        string `json:"headline,omitempty"`
	FullName        string `json:"full_name,omitempty"`
	Permalink       string `json:"permalink,omitempty"`
	MediumAvatarURL string `json:"medium_avatar_url,omitempty"`
	LargeAvatarURL  string `json:"large_avatar_url,omitempty"`
	SmallAvatarURL  string `json:"small_avatar_url,omitempty"`
}

type Category struct {
	ID   int
	Name string
}

func New() artworks.Provider {
	r := regexp.MustCompile(`(?i)https:\/\/(?:www\.)?artstation\.com\/artwork\/([\w\-]+)`)

	return &Artstation{
		regex: r,
	}
}

func (as Artstation) Find(id string) (artworks.Artwork, error) {
	reqURL := fmt.Sprintf("https://www.artstation.com/projects/%v.json", id)
	resp, err := http.Get(reqURL)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	var res ArtstationResponse
	err = json.NewDecoder(resp.Body).Decode(&res)
	if err != nil {
		return nil, err
	}

	as.hits.Add(1)
	return res, nil
}

func (as Artstation) Match(url string) (string, bool) {
	res := as.regex.FindStringSubmatch(url)
	if res == nil {
		return "", false
	}

	return res[1], true
}

func (Artstation) Enabled(g *store.Guild) bool {
	return true
}

func (as Artstation) Hits() int64 {
	return as.hits.Load()
}

func (artwork ArtstationResponse) StoreArtwork() *store.Artwork {
	images := make([]string, 0, len(artwork.Assets))
	for _, asset := range artwork.Assets {
		images = append(images, asset.ImageURL)
	}

	return &store.Artwork{
		Title:  artwork.Title,
		Author: artwork.User.Name,
		URL:    artwork.Permalink,
		Images: images,
	}
}

func (artwork ArtstationResponse) MessageSends(footer string) ([]api.SendMessageData, error) {
	var (
		length = len(artwork.Assets)
		pages  = make([]api.SendMessageData, 0, length)
		eb     = embeds.NewBuilder()
	)

	if length == 0 {
		eb.Title("❎ An error has occured.")
		eb.Description("Artwork has been deleted or the ID does not exist.")
		eb.Footer(footer, "")

		return []api.SendMessageData{{
			Embeds: []discord.Embed{eb.Build()},
		}}, nil
	}

	if length > 1 {
		eb.Title(fmt.Sprintf("%v by %v | Page %v / %v", artwork.Title, artwork.User.Name, 1, length))
	} else {
		eb.Title(fmt.Sprintf("%v by %v", artwork.Title, artwork.User.Name))
	}

	desc := bluemonday.StrictPolicy().Sanitize(artwork.Description)
	eb.Description(desc)

	eb.URL(
		artwork.URL(),
	).AddField(
		"Likes", strconv.Itoa(artwork.LikesCount), true,
	).AddField(
		"Views", strconv.Itoa(artwork.ViewsCount), true,
	).Timestamp(
		artwork.CreatedAt,
	).Footer(
		footer, "",
	)

	eb.Image(artwork.Assets[0].ImageURL)
	pages = append(pages, api.SendMessageData{Embeds: []discord.Embed{eb.Build()}})
	if length > 1 {
		for ind, image := range artwork.Assets[1:] {
			eb := embeds.NewBuilder()

			eb.Title(fmt.Sprintf("%v by %v | Page %v / %v", artwork.Title, artwork.User.Name, ind+2, length))
			eb.Image(image.ImageURL)
			eb.URL(artwork.URL()).Timestamp(artwork.CreatedAt).Footer(footer, "")

			eb.AddField("Likes", strconv.Itoa(artwork.LikesCount), true)
			pages = append(pages, api.SendMessageData{Embeds: []discord.Embed{eb.Build()}})
		}
	}

	return pages, nil
}

func (artwork ArtstationResponse) URL() string {
	return artwork.Permalink
}

func (artwork ArtstationResponse) Len() int {
	return len(artwork.Assets)
}
