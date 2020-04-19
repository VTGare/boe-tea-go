package services

import (
	"log"
	"os"
	"strconv"

	"github.com/everpcpc/pixiv"
)

var (
	baseURL = "https://pixiv.cat/"
	app     *pixiv.AppPixivAPI
)

func init() {
	pixivEmail := os.Getenv("PIXIV_EMAIL")
	if pixivEmail == "" {
		log.Fatalln("PIXIV_EMAIL env does not exist")
	}

	pixivPassword := os.Getenv("PIXIV_PASSWORD")
	if pixivEmail == "" {
		log.Fatalln("PIXIV_PASSWORD env does not exist")
	}

	_, err := pixiv.Login(pixivEmail, pixivPassword)
	if err != nil {
		log.Fatal(err)
	}
	app = pixiv.NewApp()
}

//GetPixivImages perfoms a Pixiv API call and returns an array of high-resolution image URLs
func GetPixivImages(id string) ([]string, error) {
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

	extension := getExtension(illust)

	if illust.PageCount > 1 {
		for i := 1; i <= illust.PageCount; i++ {
			images = append(images, baseURL+id+"-"+strconv.Itoa(i)+"."+extension)
		}
	} else {
		images = append(images, baseURL+id+"."+extension)
	}

	return images, nil
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
