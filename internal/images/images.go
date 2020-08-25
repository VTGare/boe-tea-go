package images

import (
	"bytes"
	"image"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"net/http"

	"github.com/disintegration/gift"
	_ "golang.org/x/image/webp"
)

func Deepfry(original image.Image) *bytes.Buffer {
	g := gift.New(
		gift.UnsharpMask(1, 5, 0),
		gift.Brightness(5),
		gift.Saturation(100),
		gift.Contrast(80),
	)

	deepfried := image.NewRGBA(original.Bounds())
	g.Draw(deepfried, original)

	var buf bytes.Buffer
	jpeg.Encode(&buf, deepfried, &jpeg.Options{
		Quality: 10,
	})
	return &buf
}

func Jpegify(original image.Image, quality int) *bytes.Buffer {
	var buf bytes.Buffer
	jpeg.Encode(&buf, original, &jpeg.Options{
		Quality: quality,
	})
	return &buf
}

func DownloadImage(url string) (image.Image, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}

	img, _, err := image.Decode(resp.Body)
	if err != nil {
		return nil, err
	}

	return img, nil
}
