package services

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/gocolly/colly/v2"
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

	for ind, thumbnail := range res.thumnails[1:] {
		sauce := SauceA2D{}
		sauce.Thumbnail = "https://ascii2d.net" + thumbnail
		sauce.From = res.detailBoxes[ind+1].from
		sauce.Name = res.detailBoxes[ind+1].names[0]
		sauce.URL = res.detailBoxes[ind+1].links[0]
		sauce.Author = res.detailBoxes[ind+1].names[1]
		sauce.AuthorURL = res.detailBoxes[ind+1].links[1]
		sauces = append(sauces, sauce)
	}

	return sauces
}
