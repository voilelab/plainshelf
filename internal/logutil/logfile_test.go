package logutil

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNewLogFileRejectsRotateWithoutDir(t *testing.T) {
	for name, dir := range map[string]string{
		"empty":      "",
		"whitespace": "  ",
	} {
		t.Run(name, func(t *testing.T) {
			lf, err := NewLogFile(LogFileConf{Type: LogFileTypeNameRotate, Dir: dir})
			if err == nil {
				lf.Close()
				t.Fatal("NewLogFile: expected an error for filename_rotate without dir")
			}
			// The operator has to be able to tell which field is wrong.
			if !strings.Contains(err.Error(), "dir") {
				t.Fatalf("NewLogFile error = %q, want it to name dir", err)
			}

			// The same error has to reach startup, which builds its loggers
			// through NewLogger and aborts on its error.
			if _, err := NewLogger(&LogConf{LogFile: LogFileConf{Type: LogFileTypeNameRotate, Dir: dir}}); err == nil {
				t.Fatal("NewLogger: expected an error for filename_rotate without dir")
			}
		})
	}
}

func TestNewLogFileRotateWithDirWritesAndLists(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "logs")
	conf := LogFileConf{Type: LogFileTypeNameRotate, Dir: dir, Prefix: "app"}

	logger, err := NewLogger(&LogConf{LogFile: conf})
	if err != nil {
		t.Fatalf("NewLogger: %v", err)
	}
	logger.Info("hello")
	if err := logger.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// The directory is created on demand and the file carries today's stamp.
	want := "app-" + time.Now().Format(logDateLayout) + ".log"
	if _, err := os.Stat(filepath.Join(dir, want)); err != nil {
		t.Fatalf("stat %s: %v", want, err)
	}

	entries, err := ListLogFilesForSources([]SourceConf{{Name: "app", LogFile: conf}})
	if err != nil {
		t.Fatalf("ListLogFilesForSources: %v", err)
	}
	if len(entries) != 1 || entries[0].Filename != want {
		t.Fatalf("entries = %+v, want one %s", entries, want)
	}
}

// TestNewLogFileLeavesOtherTypesAlone pins the reverse condition: only
// filename_rotate gained a guard.
func TestNewLogFileLeavesOtherTypesAlone(t *testing.T) {
	for _, typ := range []LogFileType{LogFileTypeDefault, LogFileTypeStderr, LogFileTypeStdout, LogFileTypeNone} {
		lf, err := NewLogFile(LogFileConf{Type: typ})
		if err != nil {
			t.Fatalf("NewLogFile(%q): %v", typ, err)
		}
		if err := lf.Close(); err != nil {
			t.Fatalf("Close(%q): %v", typ, err)
		}
	}

	// filename with an empty filename still lists nothing rather than failing.
	entries, err := listLogFilesForSource("app", LogFileConf{Type: LogFileTypeName})
	if err != nil {
		t.Fatalf("listLogFilesForSource: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("entries = %+v, want none", entries)
	}
}
