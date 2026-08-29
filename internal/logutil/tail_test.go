package logutil

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// openTailFixture writes content to a temp file and opens it for reading.
func openTailFixture(t *testing.T, content string) *os.File {
	t.Helper()

	path := filepath.Join(t.TempDir(), "app.log")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	fp, err := os.Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = fp.Close() })
	return fp
}

// readAll returns what a caller would receive after SeekTail.
func readAll(t *testing.T, fp *os.File) string {
	t.Helper()

	bs, err := io.ReadAll(fp)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	return string(bs)
}

func TestSeekTailReturnsWholeFileWhenItFits(t *testing.T) {
	const content = "line 1\nline 2\n"

	for _, tc := range []struct {
		name     string
		maxBytes int64
	}{
		{name: "larger limit", maxBytes: 1024},
		{name: "exact limit", maxBytes: int64(len(content))},
		{name: "unlimited", maxBytes: 0},
		{name: "negative is unlimited", maxBytes: -1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fp := openTailFixture(t, content)

			start, err := SeekTail(fp, tc.maxBytes)
			if err != nil {
				t.Fatalf("SeekTail: %v", err)
			}
			if start != 0 {
				t.Fatalf("start = %d, want 0", start)
			}
			if got := readAll(t, fp); got != content {
				t.Fatalf("content = %q, want the whole file", got)
			}
		})
	}
}

// TestSeekTailAlignsToALineStart pins the property the log viewer depends on:
// the response never opens on the middle of a line.
func TestSeekTailAlignsToALineStart(t *testing.T) {
	fp := openTailFixture(t, "aaaa\nbbbb\ncccc\n")

	// 12 bytes back from the end lands inside "bbbb", so the tail starts at
	// "bbbb" rather than at "bb".
	start, err := SeekTail(fp, 12)
	if err != nil {
		t.Fatalf("SeekTail: %v", err)
	}
	if start != 5 {
		t.Fatalf("start = %d, want 5", start)
	}
	if got := readAll(t, fp); got != "bbbb\ncccc\n" {
		t.Fatalf("content = %q, want whole trailing lines", got)
	}
}

// TestSeekTailKeepsAnOverlongLine covers a line longer than the requested
// tail: aligning past its break would answer with an empty body, so the
// unaligned tail is returned instead.
func TestSeekTailKeepsAnOverlongLine(t *testing.T) {
	line := strings.Repeat("x", int(maxLineScanBytes)*2)
	fp := openTailFixture(t, line+"\n")

	start, err := SeekTail(fp, 1024)
	if err != nil {
		t.Fatalf("SeekTail: %v", err)
	}
	if start != int64(len(line))+1-1024 {
		t.Fatalf("start = %d, want an unaligned tail", start)
	}
	if got := readAll(t, fp); len(got) != 1024 {
		t.Fatalf("content length = %d, want 1024", len(got))
	}
}

func TestSeekTailOnEmptyFile(t *testing.T) {
	fp := openTailFixture(t, "")

	start, err := SeekTail(fp, 1024)
	if err != nil {
		t.Fatalf("SeekTail: %v", err)
	}
	if start != 0 {
		t.Fatalf("start = %d, want 0", start)
	}
	if got := readAll(t, fp); got != "" {
		t.Fatalf("content = %q, want empty", got)
	}
}
