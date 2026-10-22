package logutil

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWithLoggerCloseLeavesFileOpen(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "app.log")
	parent, err := NewLogger(&LogConf{LogFile: LogFileConf{Type: LogFileTypeName, Filename: filename}})
	if err != nil {
		t.Fatalf("NewLogger: %v", err)
	}
	t.Cleanup(func() {
		if err := parent.Close(); err != nil {
			t.Errorf("closing parent logger: %v", err)
		}
	})

	child := parent.With("component", "child")
	parent.Info("before child close")
	if err := child.Close(); err != nil {
		t.Fatalf("closing child logger: %v", err)
	}
	parent.Info("parent after child close")
	child.Info("child after child close")

	data, err := os.ReadFile(filename)
	if err != nil {
		t.Fatalf("reading log file: %v", err)
	}
	for _, message := range []string{"before child close", "parent after child close", "child after child close"} {
		if !strings.Contains(string(data), message) {
			t.Errorf("log file is missing %q: %s", message, data)
		}
	}
}
