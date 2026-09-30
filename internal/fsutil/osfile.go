package fsutil

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/voilelab/plainshelf/internal/util"
)

// ReadTextFile returns the file's contents, or an empty string when it does not
// exist. Callers that distinguish "not configured" from "not written yet" check
// the path themselves; this only reports real read failures.
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

// WriteTextFileAtomic replaces the file at an OS path through a temp file and a
// rename, so an interrupted write cannot leave a half-written document behind.
// The parent directory is created if it is missing, and the file is written
// 0600 because the documents stored this way are one person's own device state.
// tempPattern names the temp file for os.CreateTemp. On any failure the temp
// file is removed and the destination is left untouched.
//
// This is the OS-path counterpart to WriteFileAtomic, which works over an FS
// rooted at a shelf and carries no permission or directory-creation promise.
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
