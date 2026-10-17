package fsutil

import (
	"fmt"
	"io"
	"math/rand"
	"strings"

	"github.com/voilelab/plainshelf/internal/util"
)

// tempSuffixLength is the number of random characters added to a temp file name.
const tempSuffixLength = 8

// tempDirSuffixLength is the number of random characters added to a staging
// directory name.
const tempDirSuffixLength = 6

// randomAlphabet is what RandomString draws from.
const randomAlphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

// tempPath builds a unique temp path next to name.
//
// The name keeps a trailing ".tmp" on purpose: file sync software (Dropbox,
// Syncthing, iCloud) is commonly configured to ignore that suffix, so an
// in-progress write is not synced before it is complete. The random middle
// segment keeps two concurrent writers to the same destination from sharing a
// temp file and corrupting each other's data.
func tempPath(name string) string {
	return fmt.Sprintf("%s.%s.tmp", name, RandomString(tempSuffixLength))
}

// TempDirName names a hidden staging directory that is renamed to base once
// its contents are complete.
func TempDirName(base string) string {
	return fmt.Sprintf(".%s-%s.tmp", base, RandomString(tempDirSuffixLength))
}

// IsTempName reports whether name, a single path element, is an in-progress
// write: a temp file from tempPath ("<name>.<8>.tmp") or a staging directory
// from TempDirName (".<base>-<6>.tmp"). It matches the whole form, not just the
// ".tmp" extension, so a user's "notes.tmp" is not one.
func IsTempName(name string) bool {
	stem, ok := strings.CutSuffix(name, ".tmp")
	if !ok {
		return false
	}
	if i := len(stem) - tempSuffixLength - 1; i > 0 && stem[i] == '.' && isRandomString(stem[i+1:]) {
		return true
	}
	i := len(stem) - tempDirSuffixLength - 1
	return i > 1 && stem[0] == '.' && stem[i] == '-' && isRandomString(stem[i+1:])
}

func isRandomString(s string) bool {
	for _, c := range []byte(s) {
		if strings.IndexByte(randomAlphabet, c) < 0 {
			return false
		}
	}
	return true
}

// RandomString returns n characters drawn from an alphanumeric alphabet. It
// names temp files, temp directories and rescan tokens, where only collision
// resistance matters, not cryptographic strength.
func RandomString(n int) string {
	result := make([]byte, n)
	for i := range result {
		result[i] = randomAlphabet[rand.Intn(len(randomAlphabet))]
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
