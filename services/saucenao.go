package services

import (
	"encoding/json"
	"log"
	"net/url"
	"os"

	"github.com/valyala/fasthttp"
)

type SauceNaoResult struct {
	Header  *TopHeader `json:"header"`
	Results *[]Sauce   `json:"results"`
}

type TopHeader struct {
	ResultsReturned int `json:"results_returned"`
}

type Sauce struct {
	Header *SauceHeader `json:"header"`
	Data   *SauceData   `json:"data"`
}

type SauceHeader struct {
	Similarity string `json:"similarity"`
	Thumbnail  string `json:"thumbnail"`
}

type SauceData struct {
	URLs       []string    `json:"ext_urls"`
	Title      string      `json:"title"`
	MemberName string      `json:"member_name"`
	Creator    interface{} `json:"creator"`
	Author     string      `json:"author"`
	Source     string      `json:"source"`
}

var (
	base_url = "https://saucenao.com/search.php?db=8191&output_type=2&api_key="
)

func init() {
	apiKey := os.Getenv("SAUCENAO_API")
	if apiKey == "" {
		log.Fatalln("SAUCENAO_API env does not exist")
	}
	base_url += apiKey
}

func SearchByURL(image string) (*SauceNaoResult, error) {
	image = url.QueryEscape(image)
	uri := base_url + "&url=" + image

	body, err := fasthttpGet(uri)
	if err != nil {
		return nil, err
	}

	var res SauceNaoResult
	err = json.Unmarshal(body, &res)
	if err != nil {
		return nil, err
	}

	return &res, nil
}

func fasthttpGet(uri string) ([]byte, error) {
	req := fasthttp.AcquireRequest()
	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseRequest(req)
	defer fasthttp.ReleaseResponse(resp)

	req.SetRequestURI(uri)
	req.Header.SetMethod("GET")
	err := fasthttp.Do(req, resp)
	if err != nil {
		return nil, err
	}
	return resp.Body(), nil
}
