package bookpkg

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/voilelab/plainshelf/internal/logutil"
)

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

// shiftModTime moves a file's modification time by offset from now. Staleness is
// detected from the meta file's stat (see getFileStat), and on a filesystem with
// one-second timestamp granularity a fresh write can land in the same second as
// the cached stat and go unnoticed. Setting the time outright makes that
// deterministic instead of sleeping until the second ticks over.
func shiftModTime(t *testing.T, filePath string, offset time.Duration) {
	t.Helper()

	at := time.Now().Add(offset)
	if err := os.Chtimes(filePath, at, at); err != nil {
		t.Fatalf("Failed to shift mod time of %s: %v", filePath, err)
	}
}

// treeSnapshot records every entry under root by relative path, so a test can
// assert that a refused mutation left the package on disk exactly as it was.
func treeSnapshot(t *testing.T, root string) map[string]string {
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

// snapshotDiff describes what changed between two treeSnapshot results, or "" if
// nothing did.
func snapshotDiff(before, after map[string]string) string {
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
