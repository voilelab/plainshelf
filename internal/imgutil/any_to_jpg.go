package imgutil

import (
	_ "image/gif"
	_ "image/png"

	"bytes"
	"image"
	"image/jpeg"

	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/tiff"
	_ "golang.org/x/image/webp"
)

// AnyToJPG converts image bytes of any supported format to JPEG format.
// If the input is already in JPEG format, it returns the original bytes.
func AnyToJPG(bs []byte) ([]byte, error) {
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
