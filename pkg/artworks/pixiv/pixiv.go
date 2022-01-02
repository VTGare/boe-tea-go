package pixiv

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/ReneKroon/ttlcache"
	"github.com/VTGare/boe-tea-go/internal/arrays"
	"github.com/VTGare/boe-tea-go/pkg/artworks"
	"github.com/VTGare/boe-tea-go/pkg/messages"
	models "github.com/VTGare/boe-tea-go/pkg/models/artworks"
	"github.com/VTGare/boe-tea-go/pkg/models/guilds"
	"github.com/VTGare/embeds"
	"github.com/bwmarrin/discordgo"
	"github.com/everpcpc/pixiv"
)

type Pixiv struct {
	app   *pixiv.AppPixivAPI
	cache *ttlcache.Cache
	regex *regexp.Regexp
}

type Artwork struct {
	ID     string
	Type   string
	Author string
	Title  string
	Likes  int
	Pages  int
	Tags   []string
	Images []*Image
	NSFW   bool
	Ugoira *Ugoira
	url    string
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

	return &Pixiv{
		app:   pixiv.NewApp(),
		cache: cache,
		regex: regexp.MustCompile(
			`(?i)http(?:s)?:\/\/(?:www\.)?pixiv\.net\/(?:en\/)?(?:artworks\/|member_illust\.php\?)(?:mode=medium\&)?(?:illust_id=)?([0-9]+)`,
		),
	}, nil
}

func (p Pixiv) Match(s string) (string, bool) {
	res := p.regex.FindStringSubmatch(s)
	if res == nil {
		return "", false
	}

	return res[1], true
}

func (p Pixiv) Find(id string) (artworks.Artwork, error) {
	if a, ok := p.get(id); ok {
		return a, nil
	}

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

	artwork := &Artwork{
		ID:     id,
		url:    "https://pixiv.net/en/artworks/" + id,
		Title:  illust.Title,
		Author: author,
		Tags:   tags,
		Images: images,
		NSFW:   nsfw,
		Type:   illust.Type,
		Pages:  illust.PageCount,
		Likes:  illust.TotalBookmarks,
	}

	if illust.Type == "ugoira" {
		ugoira, err := p.app.UgoiraMetadata(i)

		if err != nil {
			return nil, err
		}

		artwork.Ugoira = &Ugoira{ugoira.UgoiraMetadataUgoiraMetadata}
	}

	p.set(id, artwork)
	return artwork, nil
}

func (p Pixiv) Enabled(g *guilds.Guild) bool {
	return g.Pixiv
}

func (p Pixiv) get(id string) (*Artwork, bool) {
	a, ok := p.cache.Get(id)
	if !ok {
		return nil, ok
	}

	return a.(*Artwork), ok
}

func (p Pixiv) set(id string, artwork *Artwork) {
	p.cache.Set(id, artwork)
}

func (a Artwork) ToModel() *models.ArtworkInsert {
	return &models.ArtworkInsert{
		Title:  a.Title,
		Author: a.Author,
		URL:    a.url,
		Images: a.imageURLs(),
	}
}

func (a Artwork) MessageSends(footer string, hasTags bool) ([]*discordgo.MessageSend, error) {
	var (
		length = len(a.Images)
		pages  = make([]*discordgo.MessageSend, 0, length)
		eb     = embeds.NewBuilder()
	)

	if length == 0 {
		eb.Title("❎ An error has occured.")
		eb.Description("Pixiv artwork has been deleted or the ID does not exist.")
		eb.Footer(footer, "")

		return []*discordgo.MessageSend{
			{Embed: eb.Finalize()},
		}, nil
	}

	if length > 1 {
		eb.Title(fmt.Sprintf("%v by %v | Page %v / %v", a.Title, a.Author, 1, length))
	} else {
		eb.Title(fmt.Sprintf("%v by %v", a.Title, a.Author))
	}

	if hasTags {
		tags := arrays.MapString(a.Tags, func(s string) string {
			return fmt.Sprintf("[%v](https://pixiv.net/en/tags/%v/artworks)", s, s)
		})

		eb.Description(fmt.Sprintf("**Tags**\n%v", strings.Join(tags, " • ")))
	}

	eb.URL(
		a.url,
	).AddField(
		"Likes", strconv.Itoa(a.Likes), true,
	).AddField(
		"Original quality",
		messages.ClickHere(a.Images[0].originalProxy()),
		true,
	).Timestamp(
		time.Now(),
	)

	if footer != "" {
		eb.Footer(footer, "")
	}

	eb.Image(a.Images[0].previewProxy())
	pages = append(pages, &discordgo.MessageSend{Embed: eb.Finalize()})
	if length > 1 {
		for ind, image := range a.Images[1:] {
			eb := embeds.NewBuilder()

			eb.Title(fmt.Sprintf("%v by %v | Page %v / %v", a.Title, a.Author, ind+2, length))
			eb.Image(image.previewProxy())
			eb.URL(a.url).Timestamp(time.Now())

			if footer != "" {
				eb.Footer(footer, "")
			}

			eb.AddField("Likes", strconv.Itoa(a.Likes), true)
			eb.AddField("Original quality", messages.ClickHere(image.originalProxy()), true)

			pages = append(pages, &discordgo.MessageSend{Embed: eb.Finalize()})
		}
	}

	return pages, nil
}

func (a Artwork) URL() string {
	return a.url
}

func (a Artwork) Len() int {
	return a.Pages
}

func (a Artwork) imageURLs() []string {
	urls := make([]string, 0, len(a.Images))

	for _, img := range a.Images {
		urls = append(urls, img.originalProxy())
	}

	return urls
}

func (i Image) originalProxy() string {
	return strings.Replace(i.Original, "https://i.pximg.net", "https://boe-tea-pximg.herokuapp.com", 1)
}

func (i Image) previewProxy() string {
	return strings.Replace(i.Preview, "https://i.pximg.net", "https://boe-tea-pximg.herokuapp.com", 1)
}
