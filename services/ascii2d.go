package services

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/gocolly/colly/v2"
	log "github.com/sirupsen/logrus"
)

type SauceA2D struct {
	Thumbnail string
	Name      string
	Author    string
	AuthorURL string
	URL       string
	From      string
}

type detailBox struct {
	names []string
	links []string
	from  string
}

type resultA2D struct {
	thumnails   []string
	detailBoxes []*detailBox
}

var (
	client      = &http.Client{}
	baseASCII2D = "https://www.ascii2d.net/imagesearch/search/"
)

func GetSauceA2D(uri string) ([]SauceA2D, error) {
	log.Infoln("Getting ASCII2D source")
	page, err := getASCII2DPage(uri)
	if err != nil {
		return nil, err
	}

	return resultToSauce(page), nil
}

func getASCII2DPage(uri string) (*resultA2D, error) {
	c := colly.NewCollector()
	res := &resultA2D{
		thumnails:   make([]string, 0),
		detailBoxes: make([]*detailBox, 0),
	}

	c.OnHTML(".image-box", func(e *colly.HTMLElement) {
		res.thumnails = append(res.thumnails, e.ChildAttr("img", "src"))
	})

	c.OnHTML(".detail-box", func(e *colly.HTMLElement) {
		b := &detailBox{}
		b.from = e.ChildText("small")
		b.names = e.ChildTexts("a")
		b.links = e.ChildAttrs("a", "href")

		res.detailBoxes = append(res.detailBoxes, b)
	})

	err := c.Request("POST",
		baseASCII2D, strings.NewReader("uri="+url.QueryEscape(uri)),
		nil,
		http.Header{"Content-Type": []string{"application/x-www-form-urlencoded"}},
	)

	if err != nil {
		return nil, err
	}

	c.Wait()
	return res, nil
}

func resultToSauce(res *resultA2D) []SauceA2D {
	sauces := make([]SauceA2D, 0)

	if len(res.thumnails) == 0 || res == nil {
		return sauces
	}

	for ind, thumbnail := range res.thumnails {
		sauce := SauceA2D{}
		sauce.Thumbnail = "https://ascii2d.net" + thumbnail
		sauce.From = res.detailBoxes[ind].from
		if len(res.detailBoxes[ind].names) == 0 {
			continue
		}
		sauce.Name = res.detailBoxes[ind].names[0]

		if len(res.detailBoxes[ind].links) == 0 {
			continue
		}
		sauce.URL = res.detailBoxes[ind].links[0]

		if len(res.detailBoxes[ind].names) != 2 {
			continue
		}
		sauce.Author = res.detailBoxes[ind].names[1]

		if len(res.detailBoxes[ind].links) != 2 {
			continue
		}
		sauce.AuthorURL = res.detailBoxes[ind].links[1]

		sauces = append(sauces, sauce)
	}

	return sauces
}
