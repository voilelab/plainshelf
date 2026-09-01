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
	"slices"
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

// DirIgnoreReasonHidden is the reason a hidden directory carries. The
// leading-dot rule is not a name, so it is not part of any list: a directory
// whose name starts with a dot is never a folder, on every shelf, and that is
// the one part of the rules configuration does not reach.
const DirIgnoreReasonHidden = "hidden directories are never folders"

// IgnoredDir is one directory name the shelf scanners skip, and why.
//
// The reason is carried rather than left in a comment because it is what the
// user is told: the log line when a shelf loads its rules, and the message
// refusing to create a folder that would vanish from the next listing. A shelf
// may name its own directories in shelf.json, and only it knows why one of
// those is there.
type IgnoredDir struct {
	// Name is the directory name as it was written. Matching folds case,
	// because a share exported over SMB may spell "$RECYCLE.BIN" either way.
	Name string

	// Reason is a short phrase completing "skipped because ...". It may be
	// empty for a name a user listed without explaining.
	Reason string
}

// DefaultIgnoredDirs are the directory names filesystems, NAS firmware and sync
// clients create inside a shelf. They are never folders the user made, so a
// shelf that says nothing skips exactly these: on Synology every directory
// carries its own "@eaDir", which would otherwise double the folder tree and
// travel into the exported book cache.
//
// They are defaults, not a floor. A shelf.json that lists its own directories
// replaces this list, which is what lets a shelf on storage this list has never
// heard of describe itself - and equally what lets one drop a name it needs.
// See the load path in shelf/shelf_config.go, which says so in the log.
func DefaultIgnoredDirs() []IgnoredDir {
	return []IgnoredDir{
		{Name: "@eaDir", Reason: "Synology index and thumbnail sidecar"},
		{Name: "#recycle", Reason: "Synology network recycle bin"},
		{Name: "$RECYCLE.BIN", Reason: "Windows recycle bin, visible over SMB"},
		{Name: "lost+found", Reason: "ext filesystem recovery directory"},
	}
}

// IgnoreRules is which directory names one shelf's scanners skip. The zero
// value carries no names, so it skips hidden directories and nothing else; use
// NewIgnoreRules(DefaultIgnoredDirs()) for a shelf that has said nothing.
type IgnoreRules struct {
	// byName is keyed by the folded name. It is never written after
	// NewIgnoreRules returns, so an IgnoreRules can be copied and read from
	// several goroutines.
	byName map[string]IgnoredDir
}

// NewIgnoreRules builds the rules for a shelf that skips these directories.
// Names are matched without regard to case, and an entry with an empty name is
// dropped; validating the rest is the caller's job, because only the caller
// knows where to report a bad entry.
func NewIgnoreRules(dirs []IgnoredDir) IgnoreRules {
	byName := make(map[string]IgnoredDir, len(dirs))
	for _, dir := range dirs {
		if dir.Name == "" {
			continue
		}
		byName[strings.ToLower(dir.Name)] = dir
	}
	if len(byName) == 0 {
		return IgnoreRules{}
	}
	return IgnoreRules{byName: byName}
}

// MatchIgnoredDir reports whether a directory name is skipped, and why. The
// reason is what a caller shows the user; an empty reason means the name is
// skipped and nobody said why.
func (r IgnoreRules) MatchIgnoredDir(name string) (reason string, ignored bool) {
	if strings.HasPrefix(name, ".") {
		return DirIgnoreReasonHidden, true
	}
	dir, ok := r.byName[strings.ToLower(name)]
	if !ok {
		return "", false
	}
	return dir.Reason, true
}

// IsIgnoredDir reports whether a directory name is skipped, for the scanners,
// which have no use for the reason.
func (r IgnoreRules) IsIgnoredDir(name string) bool {
	_, ignored := r.MatchIgnoredDir(name)
	return ignored
}

// Names returns the configured names as written, sorted, for logging. Hidden
// directories are not among them: they are a rule, not a list.
func (r IgnoreRules) Names() []string {
	names := make([]string, 0, len(r.byName))
	for _, dir := range r.byName {
		names = append(names, dir.Name)
	}
	slices.Sort(names)
	return names
}

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

// ValidatePathSegment reports whether segment is a single, safe path component
// for something the shelf writes - a source ID, a book ID, an asset name:
// structurally usable and not hidden.
func ValidatePathSegment(segment string) error {
	if err := ValidateFolderSegment(segment); err != nil {
		return err
	}
	if strings.HasPrefix(segment, ".") {
		// A hidden name would keep what the shelf wrote out of the user's own
		// file manager, and out of any tool that walks the shelf.
		return util.NewError("path segment cannot start with a dot")
	}
	return nil
}

// ValidateFolderSegment reports whether segment is structurally a single, safe
// path component: non-empty, not "." or "..", free of path separators, valid
// UTF-8, and within the per-component length limit.
//
// It stops short of the hidden-name rule above, which is why folders use it: a
// hidden directory is not a folder either, but that is a scanner rule, and
// whoever holds the shelf's IgnoreRules can say so with the reason attached
// rather than reporting a malformed segment.
func ValidateFolderSegment(segment string) error {
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
