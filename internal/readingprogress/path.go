package readingprogress

import (
	"os"
	"path/filepath"

	"github.com/voilelab/plainshelf/internal/util"
)

func SharedDataDir() (string, error) {
	root, err := os.UserConfigDir()
	if err != nil {
		return "", util.Errorf("%w", err)
	}
	return filepath.Join(root, "PlainShelf"), nil
}

func DefaultProgressPath() (string, error) {
	dir, err := SharedDataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "reading_progress.json"), nil
}
