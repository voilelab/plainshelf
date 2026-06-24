package shelf

import (
	"crypto/md5"
	"fmt"
	"math/rand"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/voilelab/plainshelf/internal/fsutil"
	"github.com/voilelab/plainshelf/internal/util"
	"go.rtnl.ai/x/slugify"
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

func validateLayers(layers Layers) error {
	for _, layer := range layers {
		if err := validatePathSegment(layer); err != nil {
			return util.Errorf("invalid layer name %q: %w", layer, err)
		}
		if strings.Contains(layer, bookExtension) {
			return util.Errorf("invalid layer name %q: must not contain %q", layer, bookExtension)
		}
	}
	return nil
}

func validateSourceID(sourceID string) error {
	if err := validatePathSegment(sourceID); err != nil {
		return util.Errorf("invalid source id %q: %w", sourceID, err)
	}
	return nil
}

func validatePathSegment(segment string) error {
	if segment == "" {
		return util.NewError("path segment cannot be empty")
	}
	if segment == "." || segment == ".." {
		return util.NewError("path segment cannot be . or ..")
	}
	if strings.ContainsAny(segment, `/\`) {
		return util.NewError("path segment cannot contain path separators")
	}
	if !utf8.ValidString(segment) {
		return util.NewError("path segment must be valid UTF-8")
	}
	if len(segment) > maxPathSegmentLength {
		return util.Errorf("path segment exceeds %d bytes", maxPathSegmentLength)
	}
	return nil
}

// seedBookID derives an initial book ID from the layers and title. This is only
// a seed for the first ID candidate: once a book is created the ID is persisted
// in book.json and never recomputed, so renaming the title or moving the book to
// another layer does NOT change its ID (callers still de-duplicate on collision).
func seedBookID(layers Layers, title string) string {
	cont := strings.Join(layers, "-") + "-" + title
	md5Hash := md5.Sum([]byte(cont))
	hash := fmt.Sprintf("%x", md5Hash)
	return hash[:8] // Use the first 8 characters of the hash as the book ID
}

func titleToFolderName(title string) string {
	// Replace spaces with dashes and remove special characters for folder naming
	folderName := strings.ReplaceAll(title, " ", "-")
	return slugify.Slugify(folderName) + bookExtension
}
