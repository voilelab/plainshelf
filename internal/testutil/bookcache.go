package testutil

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const (
	bookCacheFilePrefix = "book-cache-"
	bookCacheFileSuffix = ".json"
)

const (
	bookCacheExportDeadline = 5 * time.Second
	bookCacheExportPoll     = 10 * time.Millisecond
)

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
