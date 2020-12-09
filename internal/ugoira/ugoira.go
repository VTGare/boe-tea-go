package ugoira

import (
	"bytes"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/VTGare/pixiv"
	"github.com/valyala/fasthttp"
)

var (
	client = http.DefaultClient
)

type Ugoira struct {
	ID       string
	File     *os.File
	Error    bool   `json:"error"`
	Message  string `json:"message"`
	Metadata *pixiv.UgoiraMetadataClass
}

func (a *App) NewUgoira(id string) (*Ugoira, error) {
	intID, err := strconv.Atoi(id)
	if err != nil {
		return nil, err
	}

	metadata, err := a.app.UgoiraMetadata(uint64(intID))
	if err != nil {
		return nil, err
	}

	return &Ugoira{id, nil, false, "", &metadata.UgoiraMetadataUgoiraMetadata}, nil
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
	return float64(len(u.Metadata.Frames)) / float64(u.FPS())
}

func (u *Ugoira) FPS() int {
	var (
		ms = 0.0
	)

	for _, frame := range u.Metadata.Frames {
		ms += float64(frame.Delay)
	}

	l := len(u.Metadata.Frames)
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
		return nil, fmt.Errorf("fasthttpGet(): Status Code %v", resp.StatusCode())
	}
	return resp.Body(), nil
}

func downloadZIP(ugoira *Ugoira) (*os.File, error) {
	body, err := fasthttpGet(ugoira.Metadata.ZipURLs.Medium, ugoira.ID)
	if err != nil {
		return nil, fmt.Errorf("downloadZIP(): %v", err)
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
