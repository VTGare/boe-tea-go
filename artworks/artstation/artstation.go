package artstation

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"time"

	"github.com/VTGare/boe-tea-go/artworks"
	"github.com/VTGare/boe-tea-go/store"
	"github.com/VTGare/embeds"
	"github.com/bwmarrin/discordgo"
	"github.com/microcosm-cc/bluemonday"
)

type Artstation struct {
	regex *regexp.Regexp
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

	AIGenerated bool
	CreatedAt   time.Time `json:"created_at,omitempty"`
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
	return &Artstation{
		regex: regexp.MustCompile(`(?i)https://(?:www\.)?artstation\.com/artwork/([\w\-]+)`),
	}
}

func (as *Artstation) Find(id string) (artworks.Artwork, error) {
	return artworks.NewError(as, func() (artworks.Artwork, error) {
		reqURL := fmt.Sprintf("https://www.artstation.com/projects/%v.json", id)
		resp, err := http.Get(reqURL)
		if err != nil {
			return nil, err
		}

		defer resp.Body.Close()

		res := &ArtstationResponse{}
		err = json.NewDecoder(resp.Body).Decode(res)
		if err != nil {
			return nil, err
		}

		res.AIGenerated = artworks.IsAIGenerated(res.Tags...)

		return res, nil
	})
}

func (as *Artstation) Match(url string) (string, bool) {
	res := as.regex.FindStringSubmatch(url)
	if res == nil {
		return "", false
	}

	return res[1], true
}

func (*Artstation) Enabled(g *store.Guild) bool {
	return g.Artstation
}

func (artwork *ArtstationResponse) StoreArtwork() *store.Artwork {
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

func (artwork *ArtstationResponse) MessageSends(footer string, tagsEnabled bool) ([]*discordgo.MessageSend, error) {
	if len(artwork.Assets) == 0 {
		eb := embeds.NewBuilder()
		eb.Title("❎ An error has occured.")
		eb.Description("Artwork has been deleted or the ID does not exist.")
		eb.Footer(footer, "")

		return []*discordgo.MessageSend{
			{Embeds: []*discordgo.MessageEmbed{eb.Finalize()}},
		}, nil
	}

	eb := &artworks.Embed{
		Title:       artwork.Title,
		Username:    artwork.User.Name,
		FieldName1:  "Likes",
		FieldValue1: strconv.Itoa(artwork.LikesCount),
		FieldName2:  "Views",
		FieldValue2: []string{strconv.Itoa(artwork.ViewsCount)},
		URL:         artwork.Permalink,
		Timestamp:   artwork.CreatedAt,
		Footer:      footer,
		AIGenerated: artwork.AIGenerated,
	}

	if tagsEnabled {
		eb.Description = bluemonday.StrictPolicy().Sanitize(artwork.Description)
	}

	for _, image := range artwork.Assets {
		eb.Images = append(eb.Images, image.ImageURL)
	}

	return eb.ToEmbed(), nil
}

func (artwork *ArtstationResponse) URL() string {
	return artwork.Permalink
}

func (artwork *ArtstationResponse) Len() int {
	return len(artwork.Assets)
}
