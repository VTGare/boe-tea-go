package ugoira

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"net/http"
	"os"
	"time"

	"github.com/valyala/fasthttp"
)

var (
	client = http.DefaultClient
)

type PixivUgoira struct {
	ID      string
	Error   bool       `json:"error"`
	Message string     `json:"message"`
	Body    UgoiraBody `json:"body"`
}

type UgoiraBody struct {
	Source         string        `json:"src"`
	OriginalSource string        `json:"originalSrc"`
	MIME           string        `json:"mime_type"`
	Frames         []UgoiraFrame `json:"frames"`
}

type UgoiraFrame struct {
	File  string `json:"file"`
	Delay int    `json:"delay"`
}

func (u *PixivUgoira) Duration() float64 {
	return float64(len(u.Body.Frames)) / float64(u.FPS())
}

func (u *PixivUgoira) FPS() int {
	var (
		ms = 0.0
	)

	for _, frame := range u.Body.Frames {
		ms += float64(frame.Delay)
	}

	l := len(u.Body.Frames)
	fps := 1.0 / ((ms / float64(l)) / 1000.0)
	return int(math.Floor(fps + 0.5))
}

func getUgoira(id string) (*PixivUgoira, error) {
	resp, err := http.Get("https://www.pixiv.net/ajax/illust/" + id + "/ugoira_meta")
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()
	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("failed to get ugoira. status %v", resp.Status)
	}

	var res PixivUgoira
	err = json.Unmarshal(data, &res)
	if err != nil {
		return nil, err
	}
	res.ID = id

	return &res, nil
}

func fasthttpGet(uri, id string) ([]byte, error) {
	req := fasthttp.AcquireRequest()
	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseRequest(req)
	defer fasthttp.ReleaseResponse(resp)

	req.SetRequestURI(uri)
	req.Header.SetMethod("GET")
	req.Header.SetUserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:78.0) Gecko/20100101 Firefox/78.0")
	req.Header.SetReferer("https://www.pixiv.net/en/artworks/" + id)

	err := fasthttp.Do(req, resp)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("status code %v", resp.StatusCode())
	}
	return resp.Body(), nil
}

func downloadZIP(ugoira *PixivUgoira) (*os.File, error) {
	body, err := fasthttpGet(ugoira.Body.OriginalSource, ugoira.ID)
	if err != nil {
		return nil, err
	}

	file, err := os.Create(fmt.Sprintf("temp_%v_%v.zip", time.Now().Format("15-04-05"), ugoira.ID))
	if err != nil {
		return nil, err
	}

	_, err = io.Copy(file, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	return file, nil
}
