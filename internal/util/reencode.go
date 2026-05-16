package util

import (
	"bytes"
	"io"
	"strings"

	"github.com/wlynxg/chardet"
	"golang.org/x/text/encoding/simplifiedchinese"
)

func ReEncodeToUTF8(src io.Reader) (io.Reader, string, error) {
	bs, err := io.ReadAll(src)
	if err != nil {
		return nil, "", Errorf("%w", err)
	}

	res := chardet.Detect(bs)
	if res.Confidence < 0.5 {
		return nil, "", Errorf("failed to detect encoding with sufficient confidence")
	}

	switch res.Encoding {
	case "UTF-8", "UTF-8-SIG":
		return strings.NewReader(string(bs)), res.Encoding, nil
	case "GB18030", "GBK", "GB2312":
		return simplifiedchinese.GB18030.NewDecoder().Reader(bytes.NewReader(bs)), res.Encoding, nil
	default:
		return nil, "", Errorf("unsupported encoding: `%s`", res.Encoding)
	}
}
