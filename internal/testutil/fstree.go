package testutil

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// TreeSnapshot fingerprints every path under root, so a test can assert that
// a refused mutation left the tree exactly as it was.
func TreeSnapshot(t *testing.T, root string) map[string]string {
	t.Helper()

	snapshot := map[string]string{}
	err := filepath.WalkDir(root, func(pth string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, pth)
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		snapshot[rel] = fmt.Sprintf("dir=%t size=%d mtime=%d", entry.IsDir(), info.Size(), info.ModTime().UnixNano())
		return nil
	})
	if err != nil {
		t.Fatalf("walk %q: %v", root, err)
	}
	return snapshot
}

// SnapshotDiff reports what changed between two TreeSnapshot results, or ""
// when nothing did.
func SnapshotDiff(before, after map[string]string) string {
	var lines []string
	for pth, state := range after {
		previous, existed := before[pth]
		switch {
		case !existed:
			lines = append(lines, fmt.Sprintf("+ %s (%s)", pth, state))
		case previous != state:
			lines = append(lines, fmt.Sprintf("~ %s (%s -> %s)", pth, previous, state))
		}
	}
	for pth := range before {
		if _, ok := after[pth]; !ok {
			lines = append(lines, fmt.Sprintf("- %s", pth))
		}
	}
	slices.Sort(lines)
	return strings.Join(lines, "\n")
}

// AssertNoTempFiles fails the test if any *.tmp file survived in dir.
func AssertNoTempFiles(t *testing.T, dir string) {
	t.Helper()

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir(%q): %v", dir, err)
	}
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".tmp") {
			t.Errorf("temp file left behind in %s: %s", dir, entry.Name())
		}
	}
}
