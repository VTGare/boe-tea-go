package pixiv

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/ReneKroon/ttlcache"
	"github.com/VTGare/boe-tea-go/internal/arrays"
	"github.com/VTGare/boe-tea-go/pkg/artworks"
	models "github.com/VTGare/boe-tea-go/pkg/models/artworks"
	"github.com/VTGare/embeds"
	"github.com/bwmarrin/discordgo"
	"github.com/everpcpc/pixiv"
)

type Pixiv struct {
	app   *pixiv.AppPixivAPI
	cache *ttlcache.Cache
}

type Artwork struct {
	ID     string
	Type   string
	Author string
	Title  string
	URL    string
	Likes  int
	Pages  int
	Tags   []string
	Images []*Image
	NSFW   bool
}

type Image struct {
	Preview  string
	Original string
}

func New(authToken, refreshToken string) (artworks.Provider, error) {
	_, err := pixiv.LoadAuth(authToken, refreshToken, time.Now())
	if err != nil {
		return nil, err
	}

	cache := ttlcache.NewCache()
	cache.SetTTL(30 * time.Minute)

	return &Pixiv{pixiv.NewApp(), cache}, nil
}

func (p Pixiv) Match(s string) (string, bool) {
	u, err := url.ParseRequestURI(s)
	if err != nil {
		return "", false
	}

	if !strings.Contains(u.Host, "pixiv.net") {
		return "", false
	}

	parts := strings.FieldsFunc(u.Path, func(r rune) bool {
		return r == '/'
	})

	if len(parts) == 0 || len(parts) == 1 && parts[0] == "en" {
		return "", false
	}

	if parts[0] == "en" {
		parts = parts[1:]
	}

	switch parts[0] {
	case "artworks":
		if len(parts) < 2 {
			return "", false
		}

		id := parts[1]
		if _, err := strconv.ParseUint(id, 10, 64); err != nil {
			return "", false
		}

		return id, true
	case "member_illust.php":
		query := u.Query()

		id := query.Get("illust_id")
		if _, err := strconv.ParseUint(id, 10, 64); err != nil {
			return "", false
		}

		return id, true
	default:
		return "", false
	}
}

func (p Pixiv) Find(id string) (artworks.Artwork, error) {
	i, err := strconv.ParseUint(id, 10, 64)
	if err != nil {
		return nil, err
	}

	illust, err := p.app.IllustDetail(i)
	if err != nil {
		return nil, err
	}

	author := ""
	if illust.User != nil {
		author = illust.User.Name
	} else {
		author = "Unknown"
	}

	tags := make([]string, 0)
	nsfw := false
	for _, tag := range illust.Tags {
		if tag.Name == "R-18" {
			nsfw = true
		}

		tags = append(tags, tag.Name)
	}

	images := make([]*Image, 0, illust.PageCount)
	if page := illust.MetaSinglePage; page != nil {
		if page.OriginalImageURL != "" {
			img := &Image{
				Original: page.OriginalImageURL,
				Preview:  illust.Images.Large,
			}

			images = append(images, img)
		}
	}

	for _, page := range illust.MetaPages {
		img := &Image{
			Original: page.Images.Original,
			Preview:  page.Images.Large,
		}

		images = append(images, img)
	}

	return &Artwork{
		ID:     id,
		URL:    "https://pixiv.net/en/artworks/" + id,
		Title:  illust.Title,
		Author: author,
		Tags:   tags,
		Images: images,
		NSFW:   nsfw,
		Type:   illust.Type,
		Pages:  illust.PageCount,
		Likes:  illust.TotalBookmarks,
	}, nil
}

func (a Artwork) ToModel() *models.ArtworkInsert {
	return &models.ArtworkInsert{
		Title:  a.Title,
		Author: a.Author,
		URL:    a.URL,
		Images: a.imageURLs(),
	}
}

func (a Artwork) Embeds(quote string) []*discordgo.MessageEmbed {
	var (
		length = len(a.Images)
		pages  = make([]*discordgo.MessageEmbed, 0, length)
		eb     = embeds.NewBuilder()
	)

	if length > 1 {
		eb.Title(fmt.Sprintf("%v by %v | Page %v / %v", a.Title, a.Author, 1, length))
	} else {
		eb.Title(fmt.Sprintf("%v by %v", a.Title, a.Author))
	}

	eb.URL(a.URL)
	eb.Image(a.Images[0].previewPixivMoe())

	tags := arrays.MapString(a.Tags, func(s string) string {
		return fmt.Sprintf("[%v](https://pixiv.net/en/tags/%v/artworks)", s, s)
	})
	eb.Description(fmt.Sprintf("**Tags**\n%v", strings.Join(tags, " • ")))

	eb.AddField("Likes", strconv.Itoa(a.Likes), true)
	eb.AddField("Original quality", fmt.Sprintf("[Click here](%v)", a.Images[0].originalPixivMoe()), true)
	eb.Timestamp(time.Now()).Footer(quote, "")
	pages = append(pages, eb.Finalize())

	if length > 1 {
		for ind, image := range a.Images[1:] {
			eb := embeds.NewBuilder()

			eb.Title(fmt.Sprintf("%v by %v | Page %v / %v", a.Title, a.Author, ind+2, length))
			eb.Image(image.previewPixivMoe())
			eb.URL(a.URL).Timestamp(time.Now()).Footer(quote, "")
			eb.AddField("Likes", strconv.Itoa(a.Likes), true)
			eb.AddField("Original quality", fmt.Sprintf("[Click here](%v)", image.originalPixivMoe()), true)

			pages = append(pages, eb.Finalize())
		}
	}

	return pages
}

func (a Artwork) imageURLs() []string {
	urls := make([]string, 0, len(a.Images))

	for _, img := range a.Images {
		urls = append(urls, img.originalPixivMoe())
	}

	return urls
}

func (i Image) originalPixivMoe() string {
	return "https://api.pixiv.moe/image/" + strings.TrimPrefix(i.Original, "https://")
}

func (i Image) previewPixivMoe() string {
	return "https://api.pixiv.moe/image/" + strings.TrimPrefix(i.Preview, "https://")
}
