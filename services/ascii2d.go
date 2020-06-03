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
		log.Warnln(err)
		return nil, err
	}

	return resultToSauce(page), nil
}

func getASCII2DPage(uri string) (*resultA2D, error) {
	c := colly.NewCollector()
	c.Async = false

	res := &resultA2D{
		thumnails:   make([]string, 0),
		detailBoxes: make([]*detailBox, 0),
	}

	c.OnResponse(func(f *colly.Response) {
		log.Infof("ascii2d response. Status code: %v. Headers: %v. Proxy URL: %v", f.StatusCode, f.Headers, f.Request.ProxyURL)
	})

	c.OnHTML(".image-box", func(e *colly.HTMLElement) {
		res.thumnails = append(res.thumnails, e.ChildAttr("img", "src"))
	})

	c.OnHTML(".detail-box", func(e *colly.HTMLElement) {
		b := &detailBox{}
		b.from = e.ChildText("small")
		b.names = e.ChildTexts("a")
		b.links = e.ChildAttrs("a", "href")

		log.Infoln("Detail box", b)
		res.detailBoxes = append(res.detailBoxes, b)
	})

	log.Infoln("ASCII2D POST request. URI = ", url.QueryEscape(uri))
	err := c.Request("POST",
		baseASCII2D, strings.NewReader("uri="+url.QueryEscape(uri)),
		nil,
		http.Header{"Content-Type": []string{"application/x-www-form-urlencoded"}},
	)

	if err != nil {
		return nil, err
	}

	log.Infoln("ASCII2D result: ", res)
	return res, nil
}

func resultToSauce(res *resultA2D) []SauceA2D {
	sauces := make([]SauceA2D, 0)

	if len(res.thumnails) == 0 || res == nil {
		return sauces
	}

	for ind, box := range res.detailBoxes {
		sauce := SauceA2D{}
		sauce.Thumbnail = "https://ascii2d.net" + res.thumnails[ind]
		sauce.From = box.from
		if len(box.names) >= 2 && len(box.links) >= 2 {
			sauce.Name = box.names[0]
			sauce.URL = box.links[0]
			sauce.Author = box.names[1]
			sauce.AuthorURL = box.links[1]
		}
		sauces = append(sauces, sauce)
	}

	log.Infoln("ASCII2D sauces", sauces)
	return sauces
}
