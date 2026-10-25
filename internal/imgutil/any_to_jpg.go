package imgutil

import (
	_ "image/gif"
	_ "image/png"

	"bytes"
	"errors"
	"image"
	"image/jpeg"

	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/tiff"
	_ "golang.org/x/image/webp"
)

const MaxPixels = 40_000_000

var ErrTooManyPixels = errors.New("image exceeds the pixel limit")

func AnyToJPG(bs []byte) ([]byte, error) {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(bs))
	if err != nil {
		return nil, err
	}
	if int64(cfg.Width)*int64(cfg.Height) > MaxPixels {
		return nil, ErrTooManyPixels
	}

	img, format, err := image.Decode(bytes.NewReader(bs))
	if err != nil {
		return nil, err
	}

	if format == "jpeg" {
		return bs, nil
	}

	var buf bytes.Buffer
	err = jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90})
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
