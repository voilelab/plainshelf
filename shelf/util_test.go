package shelf

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/voilelab/plainshelf/internal/fsutil"
	"github.com/voilelab/plainshelf/internal/logutil"
)

// newTestShelf creates a shelf and waits for the initial cache scan to complete.
// It registers a cleanup function to close the shelf when the test ends.
func newTestShelf(t *testing.T, conf *ShelfConf) *Shelf {
	t.Helper()
	s, err := NewShelf(conf)
	if err != nil {
		t.Fatalf("NewShelf: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	if err := s.WaitReady(context.Background()); err != nil {
		t.Fatalf("WaitReady: %v", err)
	}
	return s
}

// shiftModTime moves a file's modification time by offset from now. Staleness
// is detected from the meta file's stat (see getFileStat), and on a filesystem
// with one-second timestamp granularity a fresh write can land in the same
// second as the cached stat and go unnoticed. Setting the time outright makes
// that deterministic instead of sleeping until the second ticks over: push the
// file forward after writing it directly, or back before a write that has to go
// through the API.
func shiftModTime(t *testing.T, filePath string, offset time.Duration) {
	t.Helper()

	at := time.Now().Add(offset)
	if err := os.Chtimes(filePath, at, at); err != nil {
		t.Fatalf("Failed to shift mod time of %s: %v", filePath, err)
	}
}

func TestCreateTempDir(t *testing.T) {
	tmpDir := t.TempDir()

	root, err := os.OpenRoot(tmpDir)
	if err != nil {
		t.Fatalf("Failed to open root: %v", err)
	}
	defer root.Close()

	err = root.MkdirAll("test/tmp", 0755)
	if err != nil {
		t.Fatalf("Failed to create test/tmp directory: %v", err)
	}

	tmpName, err := createTempDir(fsutil.NewRootFS(root), "test/tmp/a")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	if tmpName == "" {
		t.Fatalf("Temp dir name is empty")
	}

	// Check if the directory was actually created
	_, err = root.Open(tmpName)
	if err != nil {
		t.Fatalf("Failed to open created temp dir: %v", err)
	}
}

func TestValidateBCP47(t *testing.T) {
	tests := []struct {
		lang     string
		expected bool
	}{
		{"en", true},
		{"zh-Hant", true},
		{"fr-CA", true},
		{"es-419", true},
		{"invalid-lang", false},
		{"123", false},
	}

	for _, test := range tests {
		result := validateBCP47(test.lang)
		if result != test.expected {
			t.Errorf("validateBCP47(%q) = %v; expected %v", test.lang, result, test.expected)
		}
	}
}

func newLoggerForTest() logutil.Logger {
	logger, err := logutil.NewLogger(&logutil.LogConf{
		Format:    "json",
		Level:     "debug",
		LogFile:   logutil.LogFileConf{Type: logutil.LogFileTypeDefault},
		AddSource: false,
	})

	if err != nil {
		panic("Failed to create logger for test: " + err.Error())
	}

	return *logger
}
