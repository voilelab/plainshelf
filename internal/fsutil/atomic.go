package fsutil

import (
	"fmt"
	"io"
	"math/rand"

	"github.com/voilelab/plainshelf/internal/util"
)

// tempSuffixLength is the number of random characters added to a temp file name.
const tempSuffixLength = 8

// tempPath builds a unique temp path next to name.
//
// The name keeps a trailing ".tmp" on purpose: file sync software (Dropbox,
// Syncthing, iCloud) is commonly configured to ignore that suffix, so an
// in-progress write is not synced before it is complete. The random middle
// segment keeps two concurrent writers to the same destination from sharing a
// temp file and corrupting each other's data.
func tempPath(name string) string {
	return fmt.Sprintf("%s.%s.tmp", name, randomSuffix(tempSuffixLength))
}

func randomSuffix(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	result := make([]byte, n)
	for i := range result {
		result[i] = letters[rand.Intn(len(letters))]
	}
	return string(result)
}

// WriteFileAtomic writes data to name so that readers never observe a partially
// written file: the data goes to a unique temp file first and is then renamed
// over the destination. On any failure the temp file is removed and the
// destination is left untouched.
func WriteFileAtomic(fsys FS, name string, data []byte) error {
	tmpName := tempPath(name)

	if err := fsys.WriteFile(tmpName, data); err != nil {
		_ = fsys.Remove(tmpName)
		return util.Errorf("%w", err)
	}

	if err := fsys.Rename(tmpName, name); err != nil {
		_ = fsys.Remove(tmpName)
		return util.Errorf("%w", err)
	}

	return nil
}

// WriteAtomic is the streaming form of WriteFileAtomic, for content that should
// not be buffered in memory. It carries the same guarantees.
func WriteAtomic(fsys FS, name string, r io.Reader) error {
	tmpName := tempPath(name)

	dest, err := fsys.OpenWriter(tmpName)
	if err != nil {
		_ = fsys.Remove(tmpName)
		return util.Errorf("%w", err)
	}

	_, copyErr := io.Copy(dest, r)
	closeErr := dest.Close()
	if copyErr != nil {
		_ = fsys.Remove(tmpName)
		return util.Errorf("%w", copyErr)
	}
	if closeErr != nil {
		_ = fsys.Remove(tmpName)
		return util.Errorf("%w", closeErr)
	}

	if err := fsys.Rename(tmpName, name); err != nil {
		_ = fsys.Remove(tmpName)
		return util.Errorf("%w", err)
	}

	return nil
}
