package util

import (
	"io"
	"strings"
	"testing"

	"golang.org/x/text/encoding/simplifiedchinese"
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
