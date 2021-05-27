package pixiv

import (
	"archive/zip"
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"

	"github.com/ericpauley/go-quantize/quantize"
	"github.com/everpcpc/pixiv"
)

type Ugoira struct {
	pixiv.UgoiraMetadataClass
}

func (u *Ugoira) GIF() (io.Reader, error) {
	zip, err := u.download()
	if err != nil {
		return nil, err
	}

	outGIF := &gif.GIF{
		Image: make([]*image.Paletted, len(u.Frames)),
		Delay: make([]int, len(u.Frames)),
	}

	for i, frame := range u.Frames {
		file, err := zip.Open(frame.File)
		if err != nil {
			return nil, err
		}

		img, _, err := image.Decode(file)
		if err != nil {
			return nil, err
		}

		q := quantize.MedianCutQuantizer{}
		palette := q.Quantize(make([]color.Color, 0, 256), img)

		paletted := image.NewPaletted(img.Bounds(), palette)
		draw.Draw(paletted, paletted.Rect, img, paletted.Rect.Min, draw.Src)

		outGIF.Image[i] = paletted
		outGIF.Delay[i] = frame.Delay / 10
	}

	var buf bytes.Buffer
	err = gif.EncodeAll(&buf, outGIF)
	if err != nil {
		return nil, err
	}

	return &buf, nil
}

func (u *Ugoira) Video() (io.Reader, error) {
	return nil, nil
}

func (u *Ugoira) download() (*zip.Reader, error) {
	req, err := http.NewRequest("GET", u.ZipURLs.Medium, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Referer", "https://pixiv.net")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	reader := bytes.NewReader(b)
	return zip.NewReader(reader, reader.Size())
}
