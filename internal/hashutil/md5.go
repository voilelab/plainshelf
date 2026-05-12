package hashutil

import (
	"crypto/md5"
	"encoding/hex"
	"io"

	"github.com/voilelab/plainshelf/internal/util"
)

// MD5Hash computes the MD5 hash and returns it as a hex string.
func MD5Hash(reader io.Reader) (string, error) {
	md5Hash := md5.New()
	if _, err := io.Copy(md5Hash, reader); err != nil {
		return "", util.Errorf("%w", err)
	}

	return hex.EncodeToString(md5Hash.Sum(nil)), nil
}
