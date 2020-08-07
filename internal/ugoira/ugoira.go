package ugoira

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/valyala/fasthttp"
)

var (
	client = http.DefaultClient
)

type Ugoira struct {
	ID      string
	File    *os.File
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

func NewUgoira(id string) (*Ugoira, error) {
	uri := fmt.Sprintf("https://www.pixiv.net/ajax/illust/%v/ugoira_meta", id)
	resp, err := fasthttpGet(uri, id)
	if err != nil {
		return nil, err
	}

	var res Ugoira
	err = json.Unmarshal(resp, &res)
	if err != nil {
		return nil, err
	}
	res.ID = id

	return &res, nil
}

func (u *Ugoira) toWebm() error {
	zip, err := downloadZIP(u)
	if err != nil {
		return err
	}

	folder := strings.TrimSuffix(zip.Name(), ".zip")
	_, err = unzip(zip.Name(), folder)
	if err != nil {
		return err
	}

	webm, err := makeWebm(folder, u)
	if err != nil {
		return err
	}
	os.RemoveAll(folder)
	zip.Close()
	os.Remove(zip.Name())

	file, err := os.Open(webm)
	if err != nil {
		return err
	}

	u.File = file
	return nil
}

func (u *Ugoira) Duration() float64 {
	return float64(len(u.Body.Frames)) / float64(u.FPS())
}

func (u *Ugoira) FPS() int {
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

func downloadZIP(ugoira *Ugoira) (*os.File, error) {
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
