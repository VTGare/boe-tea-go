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
	Author string
	Title  string
	Likes  int
	Tags   []string
	Images []string
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
		images = make([]string, 0)
	)

	pid, err := strconv.ParseUint(id, 10, 0)
	if err != nil {
		return nil, err
	}
	illust, err := getIllust(pid)
	if err != nil {
		return nil, err
	}

	//extension := getExtension(illust)

	firstpage := baseURL + strings.TrimPrefix(illust.MetaSinglePage.OriginalImageURL, "https://")
	images = append(images, firstpage)
	for _, page := range illust.MetaPages {
		link := baseURL + strings.TrimPrefix(page.Images.Original, "https://")
		images = append(images, link)
	}

	/*if illust.PageCount > 1 {
		for i := 1; i <= illust.PageCount; i++ {
			images = append(images, baseURL+id+"-"+strconv.Itoa(i)+"."+extension)
		}
	} else {
		images = append(images, baseURL+id+"."+extension)
	}*/

	tags := make([]string, 0)
	for _, t := range illust.Tags {
		tags = append(tags, t.Name)
	}

	return &PixivPost{
		Author: illust.User.Name,
		Title:  illust.Title,
		Tags:   tags,
		Images: images,
		Likes:  illust.TotalBookmarks,
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
