package util

import (
	"errors"
	"io"
	"strings"
	"testing"

	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/encoding/traditionalchinese"
	"golang.org/x/text/encoding/unicode"
)

func TestReEncodeToUTF8ASCII(t *testing.T) {
	const src = "Hello, world!"

	reader, encoding, err := ReEncodeToUTF8(strings.NewReader(src))
	if err != nil {
		t.Fatalf("ReEncodeToUTF8 returned error: %v", err)
	}
	if encoding != "Ascii" && encoding != "ASCII" && encoding != "UTF-8" && encoding != "" {
		t.Fatalf("expected ASCII-compatible encoding, got %q", encoding)
	}

	got, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("failed to read output: %v", err)
	}
	if string(got) != src {
		t.Fatalf("expected output %q, got %q", src, string(got))
	}
}

func TestReEncodeToUTF8GB18030UsesBufferedBytes(t *testing.T) {
	const src = "繁體中文 and 简体中文"

	encoded, err := simplifiedchinese.GB18030.NewEncoder().String(src)
	if err != nil {
		t.Fatalf("failed to encode test input: %v", err)
	}

	reader, encoding, err := ReEncodeToUTF8(strings.NewReader(encoded))
	if err != nil {
		t.Fatalf("ReEncodeToUTF8 returned error: %v", err)
	}
	if encoding != "GB18030" && encoding != "GBK" && encoding != "GB2312" {
		t.Fatalf("expected Chinese encoding, got %q", encoding)
	}

	got, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("failed to read converted output: %v", err)
	}
	if string(got) != src {
		t.Fatalf("expected converted output %q, got %q", src, string(got))
	}
}

func TestReEncodeToUTF8Big5(t *testing.T) {
	// Big5 has no ASCII-incompatible bytes for short strings, so chardet needs a
	// real Traditional-Chinese passage to identify it reliably.
	const src = "第一章 這是一個關於修煉的故事，主角踏上了漫長的旅途，追尋長生不老的秘密。"

	encoded, err := traditionalchinese.Big5.NewEncoder().String(src)
	if err != nil {
		t.Fatalf("failed to encode test input: %v", err)
	}

	reader, encoding, err := ReEncodeToUTF8(strings.NewReader(encoded))
	if err != nil {
		t.Fatalf("ReEncodeToUTF8 returned error: %v", err)
	}
	if encoding != "Big5" {
		t.Fatalf("expected Big5 encoding, got %q", encoding)
	}

	got, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("failed to read converted output: %v", err)
	}
	if string(got) != src {
		t.Fatalf("expected converted output %q, got %q", src, string(got))
	}
}

func TestReEncodeToUTF8UTF16(t *testing.T) {
	const src = "遮天 Big5 舊繁中 test"

	for _, tc := range []struct {
		name    string
		endian  unicode.Endianness
		wantEnc string
	}{
		{"little endian", unicode.LittleEndian, "UTF-16LE"},
		{"big endian", unicode.BigEndian, "UTF-16BE"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			encoded, err := unicode.UTF16(tc.endian, unicode.UseBOM).NewEncoder().String(src)
			if err != nil {
				t.Fatalf("failed to encode test input: %v", err)
			}

			reader, encoding, err := ReEncodeToUTF8(strings.NewReader(encoded))
			if err != nil {
				t.Fatalf("ReEncodeToUTF8 returned error: %v", err)
			}
			if encoding != tc.wantEnc {
				t.Fatalf("expected %s encoding, got %q", tc.wantEnc, encoding)
			}

			got, err := io.ReadAll(reader)
			if err != nil {
				t.Fatalf("failed to read converted output: %v", err)
			}
			if string(got) != src {
				t.Fatalf("expected converted output %q, got %q", src, string(got))
			}
			if strings.HasPrefix(string(got), "\ufeff") {
				t.Fatal("decoded output still carries a leading BOM")
			}
		})
	}
}

func TestReEncodeToUTF8StripsUTF8BOM(t *testing.T) {
	const body = "# 標題\n\n內文"
	const src = "\ufeff" + body

	reader, encoding, err := ReEncodeToUTF8(strings.NewReader(src))
	if err != nil {
		t.Fatalf("ReEncodeToUTF8 returned error: %v", err)
	}
	if encoding != "UTF-8-SIG" {
		t.Fatalf("expected UTF-8-SIG encoding, got %q", encoding)
	}

	got, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("failed to read output: %v", err)
	}
	if string(got) != body {
		t.Fatalf("expected BOM stripped to %q, got %q", body, string(got))
	}
	if strings.HasPrefix(string(got), "\ufeff") {
		t.Fatal("output still starts with a BOM")
	}
}

func TestReEncodeToUTF8UnsupportedEncoding(t *testing.T) {
	const src = "これは日本語のテキストです。文字化けのテスト。"

	encoded, err := japanese.ShiftJIS.NewEncoder().String(src)
	if err != nil {
		t.Fatalf("failed to encode test input: %v", err)
	}

	_, _, err = ReEncodeToUTF8(strings.NewReader(encoded))
	if err == nil {
		t.Fatal("expected an error for an unsupported encoding")
	}

	unsupported, ok := errors.AsType[*UnsupportedEncodingError](err)
	if !ok {
		t.Fatalf("expected *UnsupportedEncodingError, got %T: %v", err, err)
	}
	if unsupported.Encoding == "" {
		t.Fatal("unsupported encoding error carries no encoding name")
	}
	if !strings.Contains(unsupported.Error(), unsupported.Encoding) {
		t.Fatalf("error message %q does not name the encoding %q", unsupported.Error(), unsupported.Encoding)
	}
}

type countingReader struct {
	r    io.Reader
	read int
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.read += n
	return n, err
}

func TestReEncodeToUTF8ReadsOnlyThePrefixUpFront(t *testing.T) {
	for _, tc := range []struct {
		name string
		line string
	}{
		{"ASCII sniffing stops within the prefix and leaves the rest unread", "plain ascii line\n"},
		{"UTF-8 sniffing stops within the prefix and leaves the rest unread", "一行文字 line of text\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			src := strings.Repeat(tc.line, 8*encodingSniffLimit/len(tc.line))
			counted := &countingReader{r: strings.NewReader(src)}

			reader, _, err := ReEncodeToUTF8(counted)
			if err != nil {
				t.Fatalf("ReEncodeToUTF8 returned error: %v", err)
			}
			if counted.read > encodingSniffLimit {
				t.Fatalf("read %d bytes before the result was read, want at most %d", counted.read, encodingSniffLimit)
			}

			got, err := io.ReadAll(reader)
			if err != nil {
				t.Fatalf("failed to read output: %v", err)
			}
			if string(got) != src {
				t.Fatalf("output differs from input: %d bytes, want %d", len(got), len(src))
			}
		})
	}
}

func TestReEncodeToUTF8Big5BeyondThePrefix(t *testing.T) {
	src := strings.Repeat("第一章 這是一個關於修煉的故事，主角踏上了漫長的旅途。\n", 2*encodingSniffLimit/64)

	encoded, err := traditionalchinese.Big5.NewEncoder().String(src)
	if err != nil {
		t.Fatalf("failed to encode test input: %v", err)
	}

	reader, encoding, err := ReEncodeToUTF8(strings.NewReader(encoded))
	if err != nil {
		t.Fatalf("ReEncodeToUTF8 returned error: %v", err)
	}
	if encoding != "Big5" {
		t.Fatalf("expected Big5 encoding, got %q", encoding)
	}

	got, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("failed to read converted output: %v", err)
	}
	if string(got) != src {
		t.Fatal("converted output differs from the source text")
	}
}

func TestReEncodeToUTF8EncodingPastThePrefixPassesThrough(t *testing.T) {
	lateBig5, err := traditionalchinese.Big5.NewEncoder().String("第一章 這是一個關於修煉的故事，主角踏上了漫長的旅途。")
	if err != nil {
		t.Fatalf("failed to encode test input: %v", err)
	}
	src := strings.Repeat("a", encodingSniffLimit) + lateBig5

	reader, encoding, err := ReEncodeToUTF8(strings.NewReader(src))
	if err != nil {
		t.Fatalf("ReEncodeToUTF8 returned error: %v", err)
	}
	if encoding != "Ascii" {
		t.Fatalf("expected the prefix to read as Ascii, got %q", encoding)
	}

	got, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("failed to read output: %v", err)
	}
	if string(got) != src {
		t.Fatal("expected the input passed through byte for byte")
	}

	reader, encoding, err = ReEncodeToUTF8(strings.NewReader("aaaa" + lateBig5))
	if err != nil || encoding != "Big5" {
		t.Fatalf("expected Big5 inside the prefix, got %q, %v", encoding, err)
	}
	if got, _ := io.ReadAll(reader); !strings.HasSuffix(string(got), "漫長的旅途。") {
		t.Fatalf("expected decoded Big5, got %q", got)
	}
}
