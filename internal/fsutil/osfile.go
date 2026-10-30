package fsutil

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/voilelab/plainshelf/internal/util"
)

func ReadTextFile(path string) (string, error) {
	bs, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", util.Errorf("%w", err)
	}
	return string(bs), nil
}

func WriteTextFileAtomic(path, tempPattern, text string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return util.Errorf("%w", err)
	}

	tmp, err := os.CreateTemp(dir, tempPattern)
	if err != nil {
		return util.Errorf("%w", err)
	}
	tmpPath := tmp.Name()

	if _, err := tmp.WriteString(text); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return util.Errorf("%w", err)
	}
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return util.Errorf("%w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return util.Errorf("%w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return util.Errorf("%w", err)
	}

	return nil
}
