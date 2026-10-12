package bookpkg

import (
	"os"
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

// shiftModTime sets the time outright rather than sleeping until the second
// ticks over: on a filesystem with one-second timestamp granularity a fresh
// write can otherwise land in the same second as the cached stat.
func shiftModTime(t *testing.T, filePath string, offset time.Duration) {
	t.Helper()

	at := time.Now().Add(offset)
	if err := os.Chtimes(filePath, at, at); err != nil {
		t.Fatalf("Failed to shift mod time of %s: %v", filePath, err)
	}
}
