package readingprogress

import (
	"os"
	"path/filepath"

	"github.com/voilelab/plainshelf/internal/util"
)

// SharedDataDir returns the desktop app's data directory, <UserConfigDir>/PlainShelf.
// It must stay identical to the dataRoot the desktop app derives in
// desktop/app.go (startServer): the standalone reader writes progress into the
// same directory so the desktop app can read and project it.
func SharedDataDir() (string, error) {
	root, err := os.UserConfigDir()
	if err != nil {
		return "", util.Errorf("%w", err)
	}
	return filepath.Join(root, "PlainShelf"), nil
}

// DefaultProgressPath returns the shared reading_progress.json path used by both
// the desktop app and the standalone reader. It must match the desktop app's
// readingProgressPath (desktop/app.go).
func DefaultProgressPath() (string, error) {
	dir, err := SharedDataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "reading_progress.json"), nil
}
