package util

import (
	"io"

	"github.com/wlynxg/chardet"
	"golang.org/x/text/encoding/simplifiedchinese"
)

func ReEncodeToUTF8(src io.ReadSeeker) (io.Reader, string, error) {
	buf := make([]byte, 1024)
	n, err := src.Read(buf)
	if err != nil && err != io.EOF {
		return nil, "", Errorf("%w", err)
	}
	buf = buf[:n]

	src.Seek(0, io.SeekStart)

	res := chardet.Detect(buf)
	if res.Confidence < 0.5 {
		return nil, "", Errorf("failed to detect encoding with sufficient confidence")
	}

	switch res.Encoding {
	case "UTF-8", "UTF-8-SIG":
		return src, res.Encoding, nil
	case "GB18030", "GBK", "GB2312":
		return simplifiedchinese.GB18030.NewDecoder().Reader(src), res.Encoding, nil
	default:
		return nil, "", Errorf("unsupported encoding: `%s`", res.Encoding)
	}
}
