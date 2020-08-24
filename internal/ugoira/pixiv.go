package ugoira

import (
	"os"
	"strconv"
	"strings"

	"github.com/everpcpc/pixiv"
	log "github.com/sirupsen/logrus"
)

var (
	baseURL = "https://api.kotori.love/pixiv/image/"
	app     *pixiv.AppPixivAPI
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
		firstPage := baseURL + strings.TrimPrefix(illust.MetaSinglePage.OriginalImageURL, "https://")
		originalImages = append(originalImages, firstPage)
		firstpageLarge := baseURL + strings.TrimPrefix(illust.Images.Large, "https://")
		largeImages = append(largeImages, firstpageLarge)
	}

	for _, page := range illust.MetaPages {
		originalLink := baseURL + strings.TrimPrefix(page.Images.Original, "https://")
		largeLink := baseURL + strings.TrimPrefix(page.Images.Large, "https://")
		largeImages = append(largeImages, largeLink)
		originalImages = append(originalImages, originalLink)
	}

	nsfw := false
	tags := make([]string, 0)
	for _, t := range illust.Tags {
		if t.Name == "R-18" {
			nsfw = true
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
	}

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
