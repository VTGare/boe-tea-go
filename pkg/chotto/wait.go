package chotto

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"time"

	"github.com/valyala/fasthttp"
)

var (
	//ErrRateLimited ...
	ErrRateLimited = errors.New("trace.moe is rate limited, try again later")
	waitURL        = "https://trace.moe/api/search?url="
)

type Result struct {
	Limit     int        `json:"limit"`
	LimitTTL  int        `json:"limit_ttl"`
	Quota     int        `json:"quota"`
	QuotaTTL  int        `json:"quota_ttl"`
	Documents []Document `json:"docs"`
}

type Document struct {
	AnilistID    int         `json:"anilist_id"`
	MalID        int         `json:"mal_id"`
	Anime        string      `json:"anime"`
	Episode      interface{} `json:"episode"`
	From         float64     `json:"from"`
	To           float64     `json:"to"`
	At           float64     `json:"at"`
	Similarity   float64     `json:"similarity"`
	Title        string      `json:"title"`
	TitleNative  string      `json:"title_native"`
	TitleChinese string      `json:"title_chinese"`
	TitleEnglish string      `json:"title_english"`
	TitleRomaji  string      `json:"title_romaji"`
	Synonyms     []string    `json:"synonyms"`
	IsAdult      bool        `json:"is_adult"`
}

func ErrOutOfQuota(quotaTTL int) error {
	return fmt.Errorf("boe tea has ran out of today's trace.moe quota. It'll reset in %v", secondsToReadable(quotaTTL))
}

func secondsToReadable(sec int) string {
	t := time.Second * time.Duration(sec)
	return t.String()
}

func SearchWait(image string) (*Result, error) {
	image = url.QueryEscape(image)
	uri := waitURL + image

	resp, err := get(uri)
	defer fasthttp.ReleaseResponse(resp)
	if err != nil {
		return nil, err
	}

	switch resp.StatusCode() {
	case 400:
		return nil, fmt.Errorf("search image is empty")
	case 403:
		return nil, fmt.Errorf("invalid token")
	case 429:
		return nil, fmt.Errorf("requesting too fast")
	case 500:
		return nil, fmt.Errorf("something went wrong in the trace.moe backend")
	}

	var res Result
	err = json.Unmarshal(resp.Body(), &res)
	if err != nil {
		return nil, err
	}

	return &res, nil
}

func get(uri string) (*fasthttp.Response, error) {
	req := fasthttp.AcquireRequest()
	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseRequest(req)

	req.SetRequestURI(uri)
	req.Header.SetMethod("GET")
	err := fasthttp.Do(req, resp)
	if err != nil {
		return nil, err
	}
	return resp, nil
}
