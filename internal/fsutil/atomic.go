package fsutil

import (
	"fmt"
	"io"
	"math/rand"
	"strings"

	"github.com/voilelab/plainshelf/internal/util"
)

const tempExtension = ".tmp"

const tempSuffixLength = 8

const tempDirSuffixLength = 6

const randomAlphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

func tempPath(name string) string {
	return fmt.Sprintf("%s.%s%s", name, RandomString(tempSuffixLength), tempExtension)
}

func TempDirName(base string) string {
	return fmt.Sprintf(".%s-%s%s", base, RandomString(tempDirSuffixLength), tempExtension)
}

func IsTempName(name string) bool {
	return isTempFileName(name) || isStagingDirName(name)
}

func isTempFileName(name string) bool {
	stem, ok := strings.CutSuffix(name, tempExtension)
	if !ok {
		return false
	}
	i := len(stem) - tempSuffixLength - 1
	return i > 0 && stem[i] == '.' && isRandomString(stem[i+1:])
}

func isStagingDirName(name string) bool {
	stem, ok := strings.CutSuffix(name, tempExtension)
	if !ok {
		return false
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

func RandomString(n int) string {
	result := make([]byte, n)
	for i := range result {
		result[i] = randomAlphabet[rand.Intn(len(randomAlphabet))]
	}
	return string(result)
}

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
