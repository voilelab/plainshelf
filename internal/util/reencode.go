package util

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	"github.com/wlynxg/chardet"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/encoding/traditionalchinese"
	"golang.org/x/text/encoding/unicode"
)

// UnsupportedEncodingError reports that chardet identified the upload's encoding
// but ReEncodeToUTF8 has no decoder for it. It is user-actionable — the file
// itself is the problem, not the server — so handlers map it to 400 rather than
// 500 and surface the detected encoding name to the client.
type UnsupportedEncodingError struct {
	Encoding string
}

func (e *UnsupportedEncodingError) Error() string {
	return fmt.Sprintf("unsupported encoding: `%s`", e.Encoding)
}

func ReEncodeToUTF8(src io.Reader) (io.Reader, string, error) {
	bs, err := io.ReadAll(src)
	if err != nil {
		return nil, "", Errorf("%w", err)
	}

	res := chardet.Detect(bs)
	switch res.Encoding {
	case "", "Ascii", "ASCII", "UTF-8":
		return strings.NewReader(string(bs)), res.Encoding, nil
	case "UTF-8-SIG":
		// chardet detected a UTF-8 BOM. Strip it: a leading U+FEFF has no textual
		// meaning, and left in place it breaks Markdown title parsing (the opening
		// "# Title" is no longer at the start of the line).
		return strings.NewReader(strings.TrimPrefix(string(bs), "\ufeff")), res.Encoding, nil
	case "GB18030", "GBK", "GB2312":
		return simplifiedchinese.GB18030.NewDecoder().Reader(bytes.NewReader(bs)), res.Encoding, nil
	case "Big5":
		return traditionalchinese.Big5.NewDecoder().Reader(bytes.NewReader(bs)), res.Encoding, nil
	case "UTF-16", "UTF-16LE":
		// UseBOM consumes a leading BOM and lets it override the endianness, so a
		// BOM'd file decodes cleanly without a stray U+FEFF; the little-endian
		// default covers the BOM-less case (Windows Notepad "Unicode" is LE).
		dec := unicode.UTF16(unicode.LittleEndian, unicode.UseBOM)
		return dec.NewDecoder().Reader(bytes.NewReader(bs)), res.Encoding, nil
	case "UTF-16BE":
		dec := unicode.UTF16(unicode.BigEndian, unicode.UseBOM)
		return dec.NewDecoder().Reader(bytes.NewReader(bs)), res.Encoding, nil
	default:
		return nil, "", &UnsupportedEncodingError{Encoding: res.Encoding}
	}
}
