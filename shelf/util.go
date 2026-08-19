package shelf

import (
	"crypto/md5"
	"fmt"
	"io/fs"
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

// fileETag returns a weak ETag derived from a file's mtime and size, or an
// empty string when the file cannot be stat'd. Every stored file the read path
// serves with caching headers derives its validator here, so covers and source
// assets cannot drift apart on what counts as "changed".
func fileETag(root fsutil.FS, filePath string) string {
	info, err := root.Stat(filePath)
	if err != nil {
		return ""
	}
	return fmt.Sprintf(`W/"%d-%d"`, info.ModTime().UnixNano(), info.Size())
}

// entryIsDir reports whether pth is a directory, paying for a Stat call only
// when the directory entry cannot answer on its own.
//
// ReadDir already reports each entry's type, so a walk does not need to stat
// every child it visits; on a network shelf that removes roughly half the round
// trips of a full scan. The exception is a symlink: DirEntry.IsDir comes from
// the readdir type byte, which describes the link itself, so a link pointing at
// a directory reports false. Those fall back to Stat, which resolves the link
// exactly as the walk always did.
//
// entry is nil at the root of a walk, which has no directory entry of its own.
func entryIsDir(root fsutil.FS, pth string, entry fs.DirEntry) (bool, error) {
	if entry != nil && entry.Type()&fs.ModeSymlink == 0 {
		return entry.IsDir(), nil
	}

	info, err := root.Stat(pth)
	if err != nil {
		return false, util.Errorf("%w", err)
	}

	return info.IsDir(), nil
}

// ignoredDirNames are directory names that filesystems, NAS firmware and sync
// clients create inside a shelf. They are never layers the user made, so both
// scanners skip them: on Synology every directory carries its own "@eaDir",
// which would otherwise double the layer tree and travel into the exported book
// cache. Keys are lower case; isIgnoredDir folds the name before looking it up,
// because a share exported over SMB may spell "$RECYCLE.BIN" either way.
var ignoredDirNames = map[string]bool{
	"@eadir":       true, // Synology index and thumbnail sidecar
	"#recycle":     true, // Synology network recycle bin
	"$recycle.bin": true, // Windows recycle bin, visible over SMB
	"lost+found":   true, // ext filesystem recovery directory
}

// isIgnoredDir reports whether a directory name under books/ must be skipped by
// the shelf scanners. The leading-dot rule covers the open-ended set of hidden
// helper directories (.git, .stfolder, .dropbox.cache, .Spotlight-V100,
// .fseventsd, .TemporaryItems) in one condition; ignoredDirNames lists the known
// system directories that carry no leading dot.
func isIgnoredDir(name string) bool {
	if strings.HasPrefix(name, ".") {
		return true
	}
	return ignoredDirNames[strings.ToLower(name)]
}

// ErrInvalidLayer is returned when a layer name is not a usable path segment.
// Every operation that accepts caller-supplied layers checks them before
// touching the filesystem, so callers can treat it as a request error.
var ErrInvalidLayer = util.NewError("invalid layer name")

var bcp47Regex = regexp.MustCompile(`^[a-zA-Z]{2,3}(-[a-zA-Z0-9]{2,8})*$`)

func validateBCP47(lang string) bool {
	lang = strings.TrimSpace(lang)
	if lang == "" {
		return true
	}

	return bcp47Regex.MatchString(lang)
}

// validateBookFormat reports whether a BookMeta.Format value is one this build
// writes. Empty stays valid: books created through the API rather than an
// import carry no format at all, and the reader already treats that as plain
// text.
func validateBookFormat(format string) bool {
	switch format {
	case "", BookFormatText, BookFormatMarkdown:
		return true
	default:
		return false
	}
}

func validateLayers(layers Layers) error {
	for _, layer := range layers {
		if err := validatePathSegment(layer); err != nil {
			return util.Errorf("%w %q: %w", ErrInvalidLayer, layer, err)
		}
		if strings.Contains(layer, bookExtension) {
			return util.Errorf("%w %q: must not contain %q", ErrInvalidLayer, layer, bookExtension)
		}
	}
	return nil
}

// ValidateLayers reports whether every layer path segment is safe to use.
// API handlers use this before scheduling background work so malformed batch
// requests fail synchronously rather than becoming failed worker tasks.
func ValidateLayers(layers Layers) error {
	return validateLayers(layers)
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
	if isIgnoredDir(segment) {
		// Creating one would succeed on disk and then vanish: the scanners skip
		// exactly these names, so the layer would never be listed again.
		return util.NewError("path segment cannot be a hidden or system directory name (leading dot, @eaDir, #recycle, $RECYCLE.BIN, lost+found)")
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
