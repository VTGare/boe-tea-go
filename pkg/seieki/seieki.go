package seieki

import (
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/valyala/fasthttp"
)

type Seieki struct {
	Mask int

	baseURL string
	key     string
}

func NewSeieki(key string) *Seieki {
	baseURL := "https://saucenao.com/search.php?output_type=2"
	if key != "" {
		baseURL += "&api_key=" + key
	}

	return &Seieki{Mask: 8191, baseURL: baseURL, key: key}
}

func (s *Seieki) requestURL(uri string) string {
	return fmt.Sprintf("%v&db_mask=%v&url=%v", s.baseURL, s.Mask, url.QueryEscape(uri))
}

func (s *Seieki) Sauce(uri string) (*Result, error) {
	resp, err := get(s.requestURL(uri))
	defer fasthttp.ReleaseResponse(resp)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("expected 200, got %v", resp.StatusCode())
	}

	var res Result
	err = json.Unmarshal(resp.Body(), &res)
	if err != nil {
		return nil, err
	}
	res.filter()
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
