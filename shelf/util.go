package shelf

import (
	"fmt"
	"math/rand"
	"regexp"
	"strings"
	"time"

	"github.com/voilelab/plainshelf/internal/fsutil"
	"github.com/voilelab/plainshelf/internal/logutil"
	"github.com/voilelab/plainshelf/internal/util"
)

const MaxTempDirCreationAttempts = 10

func createTempDir(root fsutil.FS, prefix string) (string, error) {
	for range MaxTempDirCreationAttempts {
		tmpDirName := fmt.Sprintf("%s-%s-%s", prefix, time.Now().Format("20060102-150405"), randomString(6))
		err := root.Mkdir(tmpDirName)
		if err == nil {
			return tmpDirName, nil
		}
	}

	return "", util.NewError("failed to create temp directory after multiple attempts")
}

func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	result := make([]byte, n)
	for i := range result {
		result[i] = letters[rand.Intn(len(letters))]
	}
	return string(result)
}

var bcp47Regex = regexp.MustCompile(`^[a-zA-Z]{2,3}(-[a-zA-Z0-9]{2,8})*$`)

func validateBCP47(lang string) bool {
	lang = strings.TrimSpace(lang)
	if lang == "" {
		return true
	}

	return bcp47Regex.MatchString(lang)
}

func newLoggerForTest() logutil.Logger {
	logger, _ := logutil.NewLogger(&logutil.LogConf{
		Format:    "json",
		Level:     "debug",
		LogFile:   logutil.LogFileConf{Type: logutil.LogFileTypeDefault},
		AddSource: false,
	})
	return *logger
}
