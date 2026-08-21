package shelf

import (
	cryptorand "crypto/rand"
	"encoding/base32"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"path"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/voilelab/plainshelf/internal/fsutil"
	"github.com/voilelab/plainshelf/internal/util"
	"go.rtnl.ai/x/slugify"
)

const MaxTempDirCreationAttempts = 10

// MaxBookIDCreationAttempts bounds the retry loop that draws a fresh random
// book ID when the drawn one is already taken. With the entropy below a single
// retry is already unreachable in practice; the bound only keeps a filesystem
// that answers every probe with "taken" from spinning forever.
const MaxBookIDCreationAttempts = 10

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

// copyTree recursively copies the tree rooted at src onto dst, reproducing every
// file and subdirectory. dst is created if it does not exist. It backs CopyBook:
// a book package copied whole stays self-contained, so the relative asset paths a
// source records need no rewriting.
//
// Whether a child is a directory is decided by Stat, not by the directory
// entry's own type, so that a symlinked directory is descended into and copied
// as a real one - the same way the shelf scanner (childIsDir) treats it. A
// listing reports a symlink as a non-directory, but opening it as a file fails,
// so keying the copy on the entry type would break a package that holds one.
func copyTree(root fsutil.FS, src, dst string) error {
	info, err := root.Stat(src)
	if err != nil {
		return util.Errorf("%w", err)
	}

	if !info.IsDir() {
		return copyFile(root, src, dst)
	}

	if err := root.MkdirAll(dst); err != nil {
		return util.Errorf("%w", err)
	}

	entries, err := root.ReadDir(src)
	if err != nil {
		return util.Errorf("%w", err)
	}

	for _, entry := range entries {
		if err := copyTree(root, path.Join(src, entry.Name()), path.Join(dst, entry.Name())); err != nil {
			return util.Errorf("%w", err)
		}
	}

	return nil
}

// copyFile copies a single regular file from src to dst, creating or truncating
// dst.
func copyFile(root fsutil.FS, src, dst string) error {
	in, err := root.Open(src)
	if err != nil {
		return util.Errorf("%w", err)
	}
	defer in.Close()

	out, err := root.OpenWriter(dst)
	if err != nil {
		return util.Errorf("%w", err)
	}

	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return util.Errorf("%w", err)
	}

	if err := out.Close(); err != nil {
		return util.Errorf("%w", err)
	}

	return nil
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
func fileETag(root fsutil.ReadFS, filePath string) string {
	info, err := root.Stat(filePath)
	if err != nil {
		return ""
	}
	return fmt.Sprintf(`W/"%d-%d"`, info.ModTime().UnixNano(), info.Size())
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

// ErrIgnoredLayerName is the ErrInvalidLayer case where the name is well formed
// but names a directory the scanners skip. It wraps ErrInvalidLayer, so callers
// that only classify layer errors keep matching it, while the API can tell this
// reason apart and explain it: a user filing an existing "@eaDir" under
// PlainShelf is not making a typo, they are hitting a deliberate rule.
var ErrIgnoredLayerName = util.Errorf("%w: hidden or system directory name", ErrInvalidLayer)

// errIgnoredPathSegment marks the isIgnoredDir rejection inside
// validatePathSegment. The segment validator serves layers, source IDs, book
// IDs and asset names, so it stays sentinel-neutral and each caller wraps the
// result in the sentinel of its own domain.
var errIgnoredPathSegment = util.NewError("path segment cannot be a hidden or system directory name (leading dot, @eaDir, #recycle, $RECYCLE.BIN, lost+found)")

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
			if errors.Is(err, errIgnoredPathSegment) {
				return util.Errorf("%w %q: %w", ErrIgnoredLayerName, layer, err)
			}
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
		return errIgnoredPathSegment
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

// bookIDEntropyBytes is how much randomness stands behind a new book ID. Ten
// bytes is 80 bits, which is what makes the ID unique on its own rather than by
// agreement with anyone: a shelf would need on the order of a trillion books
// before two IDs collided with any meaningful probability. That is the property
// this needs, because the collision probe at creation time cannot see a book
// another machine wrote into a shared shelf moments ago, nor one copied in with
// a file manager.
const bookIDEntropyBytes = 10

// bookIDEncoding keeps an ID to lowercase letters and the digits 2-7. The trash
// names a folder after the book ID, so the ID has to survive a case-insensitive
// filesystem and sit in a URL path without escaping; base32 gives both, and
// dropping the padding keeps the ID a single unbroken word.
var bookIDEncoding = base32.NewEncoding("abcdefghijklmnopqrstuvwxyz234567").WithPadding(base32.NoPadding)

// newBookID draws a random book ID.
//
// The ID is opaque. It is generated once when the book is created, persisted in
// book.json, and never recomputed, so renaming the title, moving the book to
// another layer, and restoring it from the trash all leave it alone. Builds
// before this one derived it from the layers and title, which read as if the ID
// could be recomputed from them when it never was, and handed two different
// books the same ID whenever they shared a layer path and title - the case a
// shelf shared between machines produces routinely.
func newBookID() (string, error) {
	buf := make([]byte, bookIDEntropyBytes)
	if _, err := cryptorand.Read(buf); err != nil {
		return "", util.Errorf("%w", err)
	}
	return bookIDEncoding.EncodeToString(buf), nil
}

// validateBookID reports whether a caller-supplied ID is usable as one.
//
// It deliberately says nothing about the shape of the ID beyond path safety.
// Shelves written by older builds carry 8-character hex IDs, some with a "-1"
// de-duplication suffix, and those stay valid forever alongside the random ones
// this build writes; a shelf holds both at once and neither is migrated.
func validateBookID(bookID string) error {
	if err := validatePathSegment(bookID); err != nil {
		return util.Errorf("invalid book id %q: %w", bookID, err)
	}
	return nil
}

func titleToFolderName(title string) string {
	folderName := strings.ReplaceAll(title, " ", "-")
	return slugify.Slugify(folderName) + bookExtension
}
