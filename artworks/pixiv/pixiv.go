package pixiv

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/VTGare/boe-tea-go/artworks"
	"github.com/VTGare/boe-tea-go/artworks/embed"
	"github.com/VTGare/boe-tea-go/internal/arrays"
	"github.com/VTGare/boe-tea-go/internal/dgoutils"
	"github.com/VTGare/boe-tea-go/messages"
	"github.com/VTGare/boe-tea-go/store"
	"github.com/bwmarrin/discordgo"
	"github.com/everpcpc/pixiv"
)

type Pixiv struct {
	app       *pixiv.AppPixivAPI
	proxyHost string
	regex     *regexp.Regexp
}

type Artwork struct {
	ID          string
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
	return artworks.NewError(p, func() (artworks.Artwork, error) {
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

		tags := make([]string, 0)
		nsfw := false
		for _, tag := range illust.Tags {
			if tag.Name == "R-18" {
				nsfw = true
			}

			tags = dgoutils.Ternary(tag.TranslatedName != "",
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

		errImages := []string{
			"limit_sanity_level_360.png",
			"limit_unknown_360.png",
		}

		for _, img := range errImages {
			if images[0].Original == fmt.Sprintf("https://s.pximg.net/common/images/%s", img) {
				return nil, artworks.ErrRateLimited
			}
		}

		return &Artwork{
			ID:          id,
			url:         "https://www.pixiv.net/en/artworks/" + id,
			Title:       illust.Title,
			Author:      dgoutils.Ternary(illust.User != nil, illust.User.Name, "Unknown"),
			Tags:        tags,
			Images:      images,
			NSFW:        nsfw,
			Type:        illust.Type,
			Pages:       illust.PageCount,
			Likes:       illust.TotalBookmarks,
			CreatedAt:   illust.CreateDate,
			AIGenerated: illust.IllustAIType == pixiv.IllustAITypeAIGenerated,

			proxy: p.proxyHost,
		}, nil
	})
}

func (p *Pixiv) Enabled(g *store.Guild) bool {
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
	eb := &embed.Embed{
		Title:       a.Title,
		Username:    a.Author,
		FieldName1:  "Likes",
		FieldValue1: strconv.Itoa(a.Likes),
		FieldName2:  "Original quality",
		URL:         a.url,
		Timestamp:   a.CreatedAt,
		Footer:      footer,
		AIGenerated: a.AIGenerated,
	}

	if tagsEnabled && len(a.Tags) > 0 {
		tags := arrays.Map(a.Tags, func(s string) string {
			return fmt.Sprintf("[%v](https://pixiv.net/en/tags/%v/artworks)", s, s)
		})
		eb.Tags = strings.Join(tags, " • ")
	}

	for _, image := range a.Images {
		eb.Images = append(eb.Images, image.previewProxy(a.proxy))
		eb.FieldValue2 = append(eb.FieldValue2, messages.ClickHere(image.originalProxy(a.proxy)))
	}

	return eb.ToEmbed(), nil
}

func (a *Artwork) URL() string {
	return a.url
}

func (a *Artwork) Len() int {
	return a.Pages
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
