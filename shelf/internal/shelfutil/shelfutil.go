// Package shelfutil holds the low-level primitives that both the shelf package
// and the leaf shelf/bookpkg package depend on: path-segment and format
// validation, weak ETags, and a small random-name helper.
//
// It exists to keep the dependency direction single: bookpkg reads and writes a
// .bookpkg directory without importing shelf, and shelf keeps its scanner,
// trash and cache. Both import this leaf instead of reaching into each other,
// so neither has to pull in the other's file lock, cache or scan tree.
package shelfutil

import (
	"fmt"
	"math/rand"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/voilelab/plainshelf/internal/fsutil"
	"github.com/voilelab/plainshelf/internal/util"
)

// BookFormatText and BookFormatMarkdown are the values a book's or source's
// Format accepts. They decide how the reader renders the text; the bytes on disk
// are the same either way, so switching between them rewrites nothing but the
// metadata. They live here rather than in bookpkg so ValidateBookFormat, which
// both packages call, has a single source of truth for the vocabulary.
const (
	BookFormatText     = "txt"
	BookFormatMarkdown = "md"
)

// maxPathSegmentLength bounds a single path segment (layer name, source ID,
// book ID or asset name). 255 bytes is the common per-component filesystem
// limit, so a segment that passes here is one every target filesystem can hold.
const maxPathSegmentLength = 255

// RandomString returns n random alphanumeric characters. It names temporary
// directories and rescan tokens, where only collision resistance matters, not
// cryptographic strength.
func RandomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	result := make([]byte, n)
	for i := range result {
		result[i] = letters[rand.Intn(len(letters))]
	}
	return string(result)
}

// FileETag returns a weak ETag derived from a file's mtime and size, or an
// empty string when the file cannot be stat'd. Every stored file the read path
// serves with caching headers derives its validator here, so covers and source
// assets cannot drift apart on what counts as "changed".
func FileETag(root fsutil.ReadFS, filePath string) string {
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
// cache. Keys are lower case; IsIgnoredDir folds the name before looking it up,
// because a share exported over SMB may spell "$RECYCLE.BIN" either way.
var ignoredDirNames = map[string]bool{
	"@eadir":       true, // Synology index and thumbnail sidecar
	"#recycle":     true, // Synology network recycle bin
	"$recycle.bin": true, // Windows recycle bin, visible over SMB
	"lost+found":   true, // ext filesystem recovery directory
}

// IsIgnoredDir reports whether a directory name under books/ must be skipped by
// the shelf scanners. The leading-dot rule covers the open-ended set of hidden
// helper directories (.git, .stfolder, .dropbox.cache, .Spotlight-V100,
// .fseventsd, .TemporaryItems) in one condition; ignoredDirNames lists the known
// system directories that carry no leading dot.
func IsIgnoredDir(name string) bool {
	if strings.HasPrefix(name, ".") {
		return true
	}
	return ignoredDirNames[strings.ToLower(name)]
}

// ErrIgnoredPathSegment marks the IsIgnoredDir rejection inside
// ValidatePathSegment. The segment validator serves layers, source IDs, book
// IDs and asset names, so it stays sentinel-neutral and each caller wraps the
// result in the sentinel of its own domain.
var ErrIgnoredPathSegment = util.NewError("path segment cannot be a hidden or system directory name (leading dot, @eaDir, #recycle, $RECYCLE.BIN, lost+found)")

var bcp47Regex = regexp.MustCompile(`^[a-zA-Z]{2,3}(-[a-zA-Z0-9]{2,8})*$`)

// ValidateBCP47 reports whether lang is empty or a well-formed BCP 47 tag.
func ValidateBCP47(lang string) bool {
	lang = strings.TrimSpace(lang)
	if lang == "" {
		return true
	}

	return bcp47Regex.MatchString(lang)
}

// ValidateBookFormat reports whether a Format value is one this build writes.
// Empty stays valid: books created through the API rather than an import carry
// no format at all, and the reader already treats that as plain text.
func ValidateBookFormat(format string) bool {
	switch format {
	case "", BookFormatText, BookFormatMarkdown:
		return true
	default:
		return false
	}
}

// ValidateSourceID reports whether sourceID is usable as a source folder name.
func ValidateSourceID(sourceID string) error {
	if err := ValidatePathSegment(sourceID); err != nil {
		return util.Errorf("invalid source id %q: %w", sourceID, err)
	}
	return nil
}

// ValidatePathSegment reports whether segment is a single, safe path component:
// non-empty, not "." or "..", not a scanner-ignored directory name, free of
// path separators, valid UTF-8, and within the per-component length limit.
func ValidatePathSegment(segment string) error {
	if segment == "" {
		return util.NewError("path segment cannot be empty")
	}
	if segment == "." || segment == ".." {
		return util.NewError("path segment cannot be . or ..")
	}
	if IsIgnoredDir(segment) {
		// Creating one would succeed on disk and then vanish: the scanners skip
		// exactly these names, so the layer would never be listed again.
		return ErrIgnoredPathSegment
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
