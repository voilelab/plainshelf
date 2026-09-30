package testutil

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The exported book cache is written as app/book-cache-{writer_id}.json. The
// writer ID is generated on first start, so a test cannot name the file and has
// to find it. The two affixes are spelled here rather than exported from shelf:
// they are part of the on-disk layout a test is entitled to know, and exporting
// them would put a test-only accessor in the package they belong to.
const (
	bookCacheFilePrefix = "book-cache-"
	bookCacheFileSuffix = ".json"
)

// bookCacheExportDeadline bounds the wait below. It matches the one
// shelf/shelf_cache_export_test.go polls with, for the same reason: an export
// that has not landed in five seconds is a broken export, not a slow one.
const (
	bookCacheExportDeadline = 5 * time.Second
	bookCacheExportPoll     = 10 * time.Millisecond
)

// WaitForExportedBookCache polls libRoot's app folder until the exported book
// cache is there and returns its bytes. Decoding is left to the caller, which
// keeps this package clear of a shelf import.
//
// Exporting happens on a timer as well as on demand, so a caller that did not
// force it cannot know when the file appears — polling is what replaces a fixed
// sleep chosen to outlast the interval. Finding a second file is fatal at once
// rather than retried: the writer ID identifies the installation, so two files
// mean a restart orphaned a cache, which is the bug several callers are pinning.
func WaitForExportedBookCache(t *testing.T, libRoot string) []byte {
	t.Helper()

	appDir := filepath.Join(libRoot, "app")
	deadline := time.Now().Add(bookCacheExportDeadline)
	for {
		path, err := findExportedBookCache(t, appDir)
		if err == nil {
			raw, readErr := os.ReadFile(path)
			if readErr != nil {
				t.Fatalf("read exported book cache: %v", readErr)
			}
			return raw
		}
		if time.Now().After(deadline) {
			t.Fatalf("no exported book cache under %s after %s: %v", appDir, bookCacheExportDeadline, err)
		}
		time.Sleep(bookCacheExportPoll)
	}
}

// findExportedBookCache returns the single exported cache in appDir. A missing
// one is reported as an error for the poll above to retry; more than one is
// reported to the test immediately.
func findExportedBookCache(t *testing.T, appDir string) (string, error) {
	t.Helper()

	entries, err := os.ReadDir(appDir)
	if err != nil {
		return "", err
	}

	var found string
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasPrefix(name, bookCacheFilePrefix) || !strings.HasSuffix(name, bookCacheFileSuffix) {
			continue
		}
		if found != "" {
			t.Fatalf("expected exactly one exported book cache under %s, also found %s", appDir, name)
		}
		found = filepath.Join(appDir, name)
	}
	if found == "" {
		return "", os.ErrNotExist
	}
	return found, nil
}
