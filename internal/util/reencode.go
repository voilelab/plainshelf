package util

import (
	"bytes"
	"errors"
	"fmt"
	"io"

	"github.com/wlynxg/chardet"
	"github.com/wlynxg/chardet/consts"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/encoding/traditionalchinese"
	"golang.org/x/text/encoding/unicode"
)

type UnsupportedEncodingError struct {
	Encoding string
}

func (e *UnsupportedEncodingError) Error() string {
	return fmt.Sprintf("unsupported encoding: `%s`", e.Encoding)
}

const encodingSniffLimit = 1 << 20

const encodingSniffChunk = 32 << 10

func ReEncodeToUTF8(src io.Reader) (io.Reader, string, error) {
	prefix, res, err := detectEncoding(src)
	if err != nil {
		return nil, "", Errorf("%w", err)
	}

	rest := func(head []byte) io.Reader {
		return io.MultiReader(bytes.NewReader(head), src)
	}
	switch res.Encoding {
	case "", "Ascii", "ASCII", "UTF-8":
		return rest(prefix), res.Encoding, nil
	case "UTF-8-SIG":
		return rest(bytes.TrimPrefix(prefix, []byte("\ufeff"))), res.Encoding, nil
	case "GB18030", "GBK", "GB2312":
		return simplifiedchinese.GB18030.NewDecoder().Reader(rest(prefix)), res.Encoding, nil
	case "Big5":
		return traditionalchinese.Big5.NewDecoder().Reader(rest(prefix)), res.Encoding, nil
	case "UTF-16", "UTF-16LE":
		// UseBOM strips a leading BOM and picks endianness from it; LittleEndian is
		// only the fallback. Targets BOM'd UTF-16 (Notepad "Unicode") — BOM-less
		// UTF-16 CJK is detected as "" and handled as UTF-8, which is out of scope.
		dec := unicode.UTF16(unicode.LittleEndian, unicode.UseBOM)
		return dec.NewDecoder().Reader(rest(prefix)), res.Encoding, nil
	case "UTF-16BE":
		dec := unicode.UTF16(unicode.BigEndian, unicode.UseBOM)
		return dec.NewDecoder().Reader(rest(prefix)), res.Encoding, nil
	default:
		return nil, "", &UnsupportedEncodingError{Encoding: res.Encoding}
	}
}

func detectEncoding(src io.Reader) ([]byte, chardet.Result, error) {
	detector := chardet.NewUniversalDetector(consts.UnknownLangFilter)
	var prefix []byte
	chunk := make([]byte, encodingSniffChunk)
	for len(prefix) < encodingSniffLimit {
		n, err := io.ReadFull(src, chunk)
		prefix = append(prefix, chunk[:n]...)
		eof := errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF)
		if err != nil && !eof {
			return nil, chardet.Result{}, err
		}
		needsMore := detector.Feed(chunk[:n])
		if !needsMore || eof {
			break
		}
	}
	return prefix, detector.GetResult(), nil
}
