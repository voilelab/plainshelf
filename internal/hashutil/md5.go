package hashutil

import (
	"crypto/md5"
	"io"
)

func MD5Hash(reader io.Reader) (string, error) {
	md5Hash := md5.New()
	if _, err := io.Copy(md5Hash, reader); err != nil {
		return "", err
	}

	return string(md5Hash.Sum(nil)), nil
}
