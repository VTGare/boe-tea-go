package ugoira

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/ReneKroon/ttlcache"
	"github.com/VTGare/boe-tea-go/internal/database"
	"github.com/VTGare/pixiv"
	log "github.com/sirupsen/logrus"
)

type App struct {
	app   *pixiv.AppPixivAPI
	cache *ttlcache.Cache
}

func NewApp(token, refreshToken string) (*App, error) {
	_, err := pixiv.LoadAuth(token, refreshToken, time.Now().Add(3600))
	if err != nil {
		log.Warnln("pixiv.Login():", err)
		return nil, err
	}

	app := pixiv.NewApp()
	cache := ttlcache.NewCache()
	cache.SetTTL(60 * time.Minute)
	return &App{
		app:   app,
		cache: cache,
	}, nil
}

type PixivPost struct {
	ID     string
	Type   string
	Author string
	Title  string
	URL    string
	Likes  int
	Pages  int
	Ugoira *Ugoira
	Tags   []string
	Images *PixivImages
	NSFW   bool
}

type PixivImages struct {
	Preview  []*PixivImage
	Original []*PixivImage
}

func (p *PixivImages) ToArray() []string {
	images := make([]string, 0, len(p.Preview))
	switch database.DevSet.PixivReverseProxy {
	case database.PixivMoe:
		for _, img := range p.Preview {
			images = append(images, img.PixivMoe)
		}
	case database.PixivCat:
		for _, img := range p.Preview {
			images = append(images, img.PixivCat)
		}
	case database.PixivCatProxy:
		for _, img := range p.Preview {
			images = append(images, img.PixivCatProxy)
		}
	}

	return images
}

type PixivImage struct {
	PixivMoe      string
	PixivCat      string
	PixivCatProxy string
}

func (a *App) newPixivImage(url string, id uint64, manga bool, page int) *PixivImage {
	pixivCat := ""
	if manga {
		pixivCat = fmt.Sprintf("https://pixiv.cat/%v-%v.png", id, page+1)
	} else {
		pixivCat = fmt.Sprintf("https://pixiv.cat/%v.png", id)
	}

	return &PixivImage{
		PixivMoe:      "https://api.pixiv.moe/image/" + strings.TrimPrefix(url, "https://"),
		PixivCat:      pixivCat,
		PixivCatProxy: strings.Replace(url, "i.pximg.net", "i.pixiv.cat", 1),
	}
}

func (a *App) DownloadUgoira(p *PixivPost) error {
	u, err := a.NewUgoira(p.ID)
	if err != nil {
		return err
	}
	err = u.toWebm()
	if err != nil {
		return err
	}

	p.Ugoira = u
	return nil
}

//GetPixivPost perfoms a Pixiv API call and returns an array of high-resolution image URLs
func (a *App) GetPixivPost(id string) (*PixivPost, error) {
	if post, ok := a.cache.Get(id); ok {
		log.Infof("Found cached pixiv post %v", id)
		return post.(*PixivPost), nil
	}

	var (
		images = &PixivImages{make([]*PixivImage, 0), make([]*PixivImage, 0)}
	)

	pid, err := strconv.ParseUint(id, 10, 0)
	if err != nil {
		return nil, err
	}

	log.Infof("Fetching Pixiv post. ID: %v", id)
	illust, err := a.getIllust(pid)

	if err != nil {
		log.Warnln(err)
		return nil, err
	}

	if illust.MetaSinglePage != nil {
		if illust.MetaSinglePage.OriginalImageURL != "" {
			original := a.newPixivImage(illust.MetaSinglePage.OriginalImageURL, illust.ID, false, 0)
			images.Original = append(images.Original, original)
			preview := a.newPixivImage(illust.Images.Large, illust.ID, false, 0)
			images.Preview = append(images.Preview, preview)
		}
	}

	for ind, page := range illust.MetaPages {
		original := a.newPixivImage(page.Images.Original, illust.ID, true, ind)
		images.Original = append(images.Original, original)
		preview := a.newPixivImage(page.Images.Large, illust.ID, true, ind)
		images.Preview = append(images.Preview, preview)
	}

	nsfw := false
	tags := make([]string, 0)
	for _, t := range illust.Tags {
		if t.Name == "R-18" {
			nsfw = true
		}

		tags = append(tags, t.Name)
	}

	author := ""
	if illust.User != nil {
		author = illust.User.Name
	} else {
		author = "Unknown"
	}
	post := &PixivPost{
		ID:     id,
		URL:    "https://pixiv.net/en/artworks/" + id,
		Author: author,
		Type:   illust.Type,
		Title:  illust.Title,
		Tags:   tags,
		Pages:  illust.PageCount,
		Images: images,
		Likes:  illust.TotalBookmarks,
		NSFW:   nsfw,
	}

	a.cache.Set(id, post)

	log.Infof("Fetched successfully! ID: %v. Pages: %v", post.ID, post.Pages)
	return post, nil
}

func (a *App) getIllust(id uint64) (*pixiv.Illust, error) {
	ill, err := a.app.IllustDetail(id)
	if err != nil {
		return nil, err
	}

	return ill, nil
}

func (p *PixivPost) Len() int {
	return len(p.Images.Original)
}
