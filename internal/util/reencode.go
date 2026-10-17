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

// encodingSniffLimit bounds how much of the input is buffered for detection.
// CJK and UTF-8 probes settle within a few KiB, so the bound mostly matters for
// a long ASCII lead-in (front matter, a licence header) before the first
// non-ASCII byte: 1 MiB tolerates that while staying a small fraction of the
// 100 MB body limit. An encoding that first appears after it is not detected.
const encodingSniffLimit = 1 << 20

// encodingSniffChunk is the unit fed to chardet. Reads are filled to it, so the
// first chunk always holds a whole BOM.
const encodingSniffChunk = 32 << 10

// ReEncodeToUTF8 detects src's encoding from at most encodingSniffLimit bytes
// and returns a reader that streams the whole input as UTF-8. Only that prefix
// is held in memory; the rest is read from src as the result is read.
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
		// chardet detected a UTF-8 BOM. Strip it: a leading U+FEFF has no textual
		// meaning, and left in place it breaks Markdown title parsing (the opening
		// "# Title" is no longer at the start of the line).
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

// detectEncoding feeds src to chardet chunk by chunk until it settles, src
// ends, or encodingSniffLimit is reached, and returns the bytes it consumed.
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
		// Feed reports whether it still needs data.
		if !detector.Feed(chunk[:n]) || eof {
			break
		}
	}
	return prefix, detector.GetResult(), nil
}
