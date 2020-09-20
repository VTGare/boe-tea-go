package ugoira

import (
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/ReneKroon/ttlcache"
	"github.com/everpcpc/pixiv"
	log "github.com/sirupsen/logrus"
)

var (
	//Kotori is a switch for Pixiv reverse proxy engine. In case if one breaks it would be possible to switch it from outside.
	Kotori = false

	reverseProxy = map[bool]func(string) string{
		true: func(s string) string {
			return kotoriBase + strings.TrimPrefix(s, "https://")
		},
		false: func(s string) string {
			return strings.Replace(s, "i.pximg.net", "i.pixiv.cat", 1)
		},
	}
	kotoriBase = "https://api.kotori.love/pixiv/image/"
	app        *pixiv.AppPixivAPI
	pixivCache *ttlcache.Cache
	goodWaifus = map[string]bool{"すいせい": true, "ヨルハ二号B型": true, "2B": true, "牧瀬紅莉栖": true, "宝鐘マリン": true}
)

type PixivPost struct {
	ID             string
	Type           string
	Author         string
	Title          string
	Likes          int
	Pages          int
	Ugoira         *Ugoira
	Tags           []string
	LargeImages    []string
	OriginalImages []string
	NSFW           bool
	GoodWaifu      bool
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
		log.Fatalln(err)
	}
	app = pixiv.NewApp()

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
	var (
		largeImages    = make([]string, 0)
		originalImages = make([]string, 0)
	)

	if post, ok := pixivCache.Get(id); ok {
		log.Infof("Found cached pixiv post %s", id)
		return post.(*PixivPost), nil
	}

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

	if illust.MetaSinglePage.OriginalImageURL != "" {
		firstPage := reverseProxy[Kotori](illust.MetaSinglePage.OriginalImageURL)
		originalImages = append(originalImages, firstPage)
		firstpageLarge := reverseProxy[Kotori](illust.Images.Large)
		largeImages = append(largeImages, firstpageLarge)
	}

	for _, page := range illust.MetaPages {
		originalLink := reverseProxy[Kotori](page.Images.Original)
		largeLink := reverseProxy[Kotori](page.Images.Large)
		largeImages = append(largeImages, largeLink)
		originalImages = append(originalImages, originalLink)
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

	post := &PixivPost{
		ID:             id,
		Author:         illust.User.Name,
		Type:           illust.Type,
		Title:          illust.Title,
		Tags:           tags,
		Pages:          illust.PageCount,
		LargeImages:    largeImages,
		OriginalImages: originalImages,
		Likes:          illust.TotalBookmarks,
		NSFW:           nsfw,
		GoodWaifu:      goodwaifu,
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
