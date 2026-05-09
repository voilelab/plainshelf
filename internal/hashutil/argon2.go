package hashutil

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/voilelab/plainshelf/internal/util"
	"golang.org/x/crypto/argon2"
)

const (
	ArgonTimeCost   uint32 = 1
	ArgonMemoryKiB  uint32 = 64 * 1024
	ArgonThreads    uint8  = 4
	ArgonKeyLength  uint32 = 32
	ArgonSaltLength int    = 16
)

func NewContentHash(content []byte) (string, error) {
	salt := make([]byte, ArgonSaltLength)
	_, err := rand.Read(salt)
	if err != nil {
		return "", util.Errorf("%w", err)
	}

	key := argon2.IDKey(content, salt, ArgonTimeCost, ArgonMemoryKiB, ArgonThreads, ArgonKeyLength)

	encodedSalt := base64.RawStdEncoding.EncodeToString(salt)
	encodedKey := base64.RawStdEncoding.EncodeToString(key)

	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s", argon2.Version, ArgonMemoryKiB, ArgonTimeCost, ArgonThreads, encodedSalt, encodedKey), nil
}

func parseContentHash(encoded string) (salt []byte, key []byte, timeCost uint32, memoryKiB uint32, threads uint8, err error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 {
		return nil, nil, 0, 0, 0, util.Errorf("invalid argon2 hash format")
	}
	if parts[1] != "argon2id" {
		return nil, nil, 0, 0, 0, util.Errorf("unsupported hash algorithm")
	}

	var version int
	_, err = fmt.Sscanf(parts[2], "v=%d", &version)
	if err != nil {
		return nil, nil, 0, 0, 0, util.Errorf("invalid argon2 version: %w", err)
	}
	if version != argon2.Version {
		return nil, nil, 0, 0, 0, util.Errorf("unsupported argon2 version")
	}

	var parsedThreads uint32
	_, err = fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memoryKiB, &timeCost, &parsedThreads)
	if err != nil {
		return nil, nil, 0, 0, 0, util.Errorf("invalid argon2 parameters: %w", err)
	}
	if parsedThreads > 255 {
		return nil, nil, 0, 0, 0, util.Errorf("invalid argon2 thread value")
	}
	threads = uint8(parsedThreads)

	salt, err = base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return nil, nil, 0, 0, 0, util.Errorf("invalid argon2 salt: %w", err)
	}

	key, err = base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return nil, nil, 0, 0, 0, util.Errorf("invalid argon2 hash: %w", err)
	}

	return salt, key, timeCost, memoryKiB, threads, nil
}

func VerifyContentHash(content []byte, encoded string) (bool, error) {
	salt, expectedKey, timeCost, memoryKiB, threads, err := parseContentHash(encoded)
	if err != nil {
		return false, err
	}

	actualKey := argon2.IDKey(content, salt, timeCost, memoryKiB, threads, uint32(len(expectedKey)))
	return subtle.ConstantTimeCompare(actualKey, expectedKey) == 1, nil
}
