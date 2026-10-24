package apitest

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/voilelab/plainshelf/server"
)

func WriteLogFile(t *testing.T, path, content string) {
	t.Helper()

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile %s: %v", filepath.Base(path), err)
	}
}

func LogEntryByFilename(t *testing.T, env *Env, filename string) server.LogFileEntry {
	t.Helper()

	for _, entry := range GetJSON[[]server.LogFileEntry](t, env, LogURL()) {
		if entry.Filename == filename {
			return entry
		}
	}
	t.Fatalf("log list is missing the seeded file %s", filename)
	return server.LogFileEntry{}
}
