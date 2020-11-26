package ugoira

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/ReneKroon/ttlcache"
	"github.com/VTGare/boe-tea-go/utils"
	"github.com/VTGare/pixiv"
	log "github.com/sirupsen/logrus"
)

var (
	kotoriBase = "https://api.kotori.love/pixiv/image/"
	app        *pixiv.AppPixivAPI
	pixivCache *ttlcache.Cache
	goodWaifus = map[string]bool{"星街すいせい": true, "ヨルハ二号B型": true, "2B": true, "牧瀬紅莉栖": true, "宝鐘マリン": true}
)

type PixivPost struct {
	ID        string
	Type      string
	Author    string
	Title     string
	URL       string
	Likes     int
	Pages     int
	Ugoira    *Ugoira
	Tags      []string
	Images    *PixivImages
	NSFW      bool
	GoodWaifu bool
}

type PixivImages struct {
	Preview  []*PixivImage
	Original []*PixivImage
}

func (p *PixivImages) ToArray() []string {
	images := make([]string, 0, len(p.Preview))
	for _, img := range p.Preview {
		images = append(images, img.Kotori)
	}

	return images
}

type PixivImage struct {
	Kotori        string
	PixivCat      string
	PixivCatProxy string
}

func newPixivImage(url string, id uint64, manga bool, page int) *PixivImage {
	pixivCat := ""
	if manga {
		pixivCat = fmt.Sprintf("https://pixiv.cat/%v-%v.png", id, page+1)
	} else {
		pixivCat = fmt.Sprintf("https://pixiv.cat/%v.png", id)
	}

	return &PixivImage{
		Kotori:        kotoriBase + strings.TrimPrefix(url, "https://"),
		PixivCat:      pixivCat,
		PixivCatProxy: strings.Replace(url, "i.pximg.net", "i.pixiv.cat", 1),
	}
}

func init() {
	pixivEmail := os.Getenv("PIXIV_EMAIL")
	if pixivEmail == "" {
		log.Fatalln("PIXIV_EMAIL env does not exist")
	}

	pixivPassword := os.Getenv("PIXIV_PASSWORD")
	if pixivPassword == "" {
		log.Fatalln("PIXIV_PASSWORD env does not exist")
	}

	_, err := pixiv.Login(pixivEmail, pixivPassword)
	if err != nil {
		log.Warnln("pixiv.Login():", err)
	} else {
		app = pixiv.NewApp()
		utils.IsPixivUp = true
	}

	pixivCache = ttlcache.NewCache()
	pixivCache.SetTTL(60 * time.Minute)
}

func (p *PixivPost) DownloadUgoira() error {
	u, err := NewUgoira(p.ID)
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
func GetPixivPost(id string) (*PixivPost, error) {
	if post, ok := pixivCache.Get(id); ok {
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
	illust, err := getIllust(pid)

	if err != nil {
		log.Warnln(err)
		return nil, err
	}

	if illust.MetaSinglePage != nil {
		if illust.MetaSinglePage.OriginalImageURL != "" {
			original := newPixivImage(illust.MetaSinglePage.OriginalImageURL, illust.ID, false, 0)
			images.Original = append(images.Original, original)
			preview := newPixivImage(illust.Images.Large, illust.ID, false, 0)
			images.Preview = append(images.Preview, preview)
		}
	}

	for ind, page := range illust.MetaPages {
		original := newPixivImage(page.Images.Original, illust.ID, true, ind)
		images.Original = append(images.Original, original)
		preview := newPixivImage(page.Images.Large, illust.ID, true, ind)
		images.Preview = append(images.Preview, preview)
	}

	nsfw := false
	goodwaifu := false
	tags := make([]string, 0)
	for _, t := range illust.Tags {
		if t.Name == "R-18" {
			nsfw = true
		}

		if _, ok := goodWaifus[t.Name]; ok {
			goodwaifu = true
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
		ID:        id,
		URL:       "https://pixiv.net/en/artworks/" + id,
		Author:    author,
		Type:      illust.Type,
		Title:     illust.Title,
		Tags:      tags,
		Pages:     illust.PageCount,
		Images:    images,
		Likes:     illust.TotalBookmarks,
		NSFW:      nsfw,
		GoodWaifu: goodwaifu,
	}

	pixivCache.Set(id, post)

	log.Infof("Fetched successfully! ID: %v. Pages: %v", post.ID, post.Pages)
	return post, nil
}

func getIllust(id uint64) (*pixiv.Illust, error) {
	ill, err := app.IllustDetail(id)
	if err != nil {
		return nil, err
	}

	return ill, nil
}

func getExtension(i *pixiv.Illust) string {
	return string(i.Images.Large[len(i.Images.Large)-3:])
}

func (p *PixivPost) Len() int {
	return len(p.Images.Original)
}
