package pixiv

import (
	"fmt"
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
	"github.com/everpcpc/pixiv"
	"github.com/julien040/go-ternary"
)

type Pixiv struct {
	app       *pixiv.AppPixivAPI
	proxyHost string
	regex     *regexp.Regexp
}

type Artwork struct {
	Type        string
	Author      string
	Title       string
	Likes       int
	Pages       int
	Tags        []string
	Images      []*Image
	NSFW        bool
	AIGenerated bool
	CreatedAt   time.Time

	id    string
	url   string
	proxy string
}

type Image struct {
	Preview  string
	Original string
}

func LoadAuth(authToken, refreshToken string) error {
	_, err := pixiv.LoadAuth(authToken, refreshToken, time.Now())
	if err != nil {
		return err
	}
	return nil
}

func New(proxyHost string) artworks.Provider {
	if proxyHost == "" {
		proxyHost = "https://boetea.dev"
	}

	return &Pixiv{
		app:       pixiv.NewApp(),
		proxyHost: proxyHost,
		regex:     regexp.MustCompile(`(?i)https?://(?:www\.)?pixiv\.net/(?:en/)?(?:artworks/|member_illust\.php\?)(?:mode=medium&)?(?:illust_id=)?([0-9]+)`),
	}
}

func (p *Pixiv) Match(s string) (string, bool) {
	res := p.regex.FindStringSubmatch(s)
	if res == nil {
		return "", false
	}

	return res[1], true
}

func (p *Pixiv) Find(id string) (artworks.Artwork, error) {
	return artworks.WrapError(p, func() (artworks.Artwork, error) {
		i, err := strconv.ParseUint(id, 10, 64)
		if err != nil {
			return nil, err
		}

		illust, err := p.app.IllustDetail(i)
		if err != nil {
			return nil, err
		}

		if illust.ID == 0 {
			return nil, artworks.ErrArtworkNotFound
		}

		author := ternary.If(illust.User != nil,
			illust.User.Name,
			"Unknown",
		)

		tags := make([]string, 0)
		nsfw := false
		for _, tag := range illust.Tags {
			if tag.Name == "R-18" {
				nsfw = true
			}

			tags = ternary.If(tag.TranslatedName != "",
				append(tags, tag.TranslatedName),
				append(tags, tag.Name),
			)
		}

		images := make([]*Image, 0, illust.PageCount)
		if page := illust.MetaSinglePage; page != nil {
			if page.OriginalImageURL != "" {
				img := &Image{
					Original: page.OriginalImageURL,
					Preview:  illust.Images.Medium,
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

		artwork := &Artwork{
			id:        id,
			url:       "https://www.pixiv.net/en/artworks/" + id,
			Title:     illust.Title,
			Author:    author,
			Tags:      tags,
			Images:    images,
			NSFW:      nsfw,
			Type:      illust.Type,
			Pages:     illust.PageCount,
			Likes:     illust.TotalBookmarks,
			CreatedAt: illust.CreateDate,

			proxy: p.proxyHost,
		}

		errImages := []string{
			"limit_sanity_level_360.png",
			"limit_unknown_360.png",
		}

		for _, img := range errImages {
			if artwork.Images[0].Original == fmt.Sprintf("https://s.pximg.net/common/images/%s", img) {
				return nil, artworks.ErrRateLimited
			}
		}

		if illust.IllustAIType == pixiv.IllustAITypeAIGenerated {
			artwork.AIGenerated = true
		}

		return artwork, nil
	})
}

// Name implements artworks.Provider.
func (*Pixiv) Name() string {
	return "pixiv"
}

func (*Pixiv) Enabled(g *store.Guild) bool {
	return g.Pixiv
}

func (a *Artwork) StoreArtwork() *store.Artwork {
	return &store.Artwork{
		Title:  a.Title,
		Author: a.Author,
		URL:    a.url,
		Images: a.imageURLs(),
	}
}

func (a *Artwork) MessageSends(footer string, tagsEnabled bool) ([]*discordgo.MessageSend, error) {
	var (
		length = len(a.Images)
		pages  = make([]*discordgo.MessageSend, 0, length)
		eb     = embeds.NewBuilder()
	)

	eb.Title(ternary.If(length > 1,
		fmt.Sprintf("%v by %v | Page %v / %v", a.Title, a.Author, 1, length),
		fmt.Sprintf("%v by %v", a.Title, a.Author),
	))

	if tagsEnabled && len(a.Tags) > 0 {
		tags := arrays.Map(a.Tags, func(s string) string {
			return fmt.Sprintf("[%v](https://pixiv.net/en/tags/%v/artworks)", s, s)
		})

		eb.Description(fmt.Sprintf("**Tags**\n%v", strings.Join(tags, " • ")))
	}

	eb.URL(a.url).
		AddField("Likes", strconv.Itoa(a.Likes), true).
		AddField("Original quality", messages.ClickHere(a.Images[0].originalProxy(a.proxy)), true).
		Timestamp(a.CreatedAt)

	if footer != "" {
		eb.Footer(footer, "")
	}

	if a.AIGenerated {
		eb.AddField("⚠️ Disclaimer", "This artwork is AI-generated.")
	}

	eb.Image(a.Images[0].previewProxy(a.proxy))
	pages = append(pages, &discordgo.MessageSend{Embeds: []*discordgo.MessageEmbed{eb.Finalize()}})
	if length > 1 {
		for ind, image := range a.Images[1:] {
			eb := embeds.NewBuilder()

			eb.Title(fmt.Sprintf("%v by %v | Page %v / %v", a.Title, a.Author, ind+2, length))
			eb.Image(image.previewProxy(a.proxy))
			eb.URL(a.url).Timestamp(a.CreatedAt)

			if footer != "" {
				eb.Footer(footer, "")
			}

			eb.AddField("Likes", strconv.Itoa(a.Likes), true)
			eb.AddField("Original quality", messages.ClickHere(image.originalProxy(a.proxy)), true)

			pages = append(pages, &discordgo.MessageSend{Embeds: []*discordgo.MessageEmbed{eb.Finalize()}})
		}
	}

	return pages, nil
}

func (a *Artwork) URL() string {
	return a.url
}

func (a *Artwork) Len() int {
	return a.Pages
}

func (a *Artwork) ID() string {
	return a.id
}

func (a *Artwork) imageURLs() []string {
	urls := make([]string, 0, len(a.Images))

	for _, img := range a.Images {
		urls = append(urls, img.originalProxy(a.proxy))
	}

	return urls
}

func (i Image) originalProxy(host string) string {
	return strings.Replace(i.Original, "https://i.pximg.net", host, 1)
}

func (i Image) previewProxy(host string) string {
	return strings.Replace(i.Preview, "https://i.pximg.net", host, 1)
}
