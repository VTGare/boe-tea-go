package services

import (
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/everpcpc/pixiv"
)

var (
	baseURL = "https://api.kotori.love/pixiv/image/"
	app     *pixiv.AppPixivAPI
)

type PixivPost struct {
	Author         string
	Title          string
	Likes          int
	Tags           []string
	LargeImages    []string
	OriginalImages []string
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
		log.Fatal(err)
	}
	app = pixiv.NewApp()
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
	illust, err := getIllust(pid)
	if err != nil {
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

	tags := make([]string, 0)
	for _, t := range illust.Tags {
		tags = append(tags, t.Name)
	}

	return &PixivPost{
		Author:         illust.User.Name,
		Title:          illust.Title,
		Tags:           tags,
		LargeImages:    largeImages,
		OriginalImages: originalImages,
		Likes:          illust.TotalBookmarks,
	}, nil
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
