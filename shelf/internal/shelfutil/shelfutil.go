// Package shelfutil holds the primitives both shelf and shelf/bookpkg depend
// on, so that neither has to import the other and pull in its file lock, cache
// or scan tree.
package shelfutil

import (
	"fmt"
	"math/rand"
	"regexp"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/voilelab/plainshelf/internal/fsutil"
	"github.com/voilelab/plainshelf/internal/util"
)

// The formats a book's or source's Format accepts. They live here rather than
// in bookpkg so ValidateBookFormat, which both packages call, has one source of
// truth for the vocabulary.
const (
	BookFormatText     = "txt"
	BookFormatMarkdown = "md"
)

// 255 bytes is the common per-component filesystem limit, so a segment that
// passes here is one every target filesystem can hold.
const maxPathSegmentLength = 255

// RandomString names temporary directories and rescan tokens, where only
// collision resistance matters, not cryptographic strength.
func RandomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	result := make([]byte, n)
	for i := range result {
		result[i] = letters[rand.Intn(len(letters))]
	}
	return string(result)
}

// FileETag returns an empty string when the file cannot be stat'd. Every file
// the read path serves with caching headers derives its validator here, so
// covers and source assets cannot drift apart on what counts as "changed".
func FileETag(root fsutil.ReadFS, filePath string) string {
	info, err := root.Stat(filePath)
	if err != nil {
		return ""
	}
	return fmt.Sprintf(`W/"%d-%d"`, info.ModTime().UnixNano(), info.Size())
}

// The leading-dot rule is not a name, so it is not part of any list: it holds
// on every shelf, and is the one part of the rules configuration cannot reach.
const DirIgnoreReasonHidden = "hidden directories are never folders"

// IgnoredDir is one directory name the shelf scanners skip, and why.
//
// The reason is data rather than a comment because it is what the user is told:
// the log line when a shelf loads its rules, and the refusal to create a folder
// that would vanish from the next listing. A shelf may name its own directories
// in shelf.json, and only it knows why one of those is there.
type IgnoredDir struct {
	// Name is the directory name as it was written. Matching folds case,
	// because a share exported over SMB may spell "$RECYCLE.BIN" either way.
	Name string

	// Reason completes "skipped because ...", and may be empty for a name a
	// user listed without explaining.
	Reason string
}

// DefaultIgnoredDirs are the directories filesystems, NAS firmware and sync
// clients create inside a shelf: on Synology every directory carries its own
// "@eaDir", which would otherwise double the folder tree and travel into the
// exported book cache.
//
// They are defaults, not a floor. A shelf.json that lists its own directories
// replaces this list, which is what lets a shelf on unfamiliar storage describe
// itself - and equally what lets one drop a name it needs.
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

// NewIgnoreRules matches names without regard to case and drops an entry with
// an empty name; validating the rest is the caller's job, because only the
// caller knows where to report a bad entry.
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

// MatchIgnoredDir returns the reason a caller shows the user; an empty reason
// means the name is skipped and nobody said why.
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

// NSFWFolder is a path rather than a name, unlike IgnoredDir: a directory a
// filesystem creates carries the same name at every level and is skipped
// wherever it appears, while "the folder I keep adult books in" is one place in
// one tree.
type NSFWFolder struct {
	// Path is written "/"-separated - "Fiction/Adult". Matching folds case (see
	// foldSegment) and ignores leading, trailing and repeated separators, so a
	// path copied out of a file manager still matches.
	Path string

	// Reason completes "marked because ...", and may be empty.
	//
	// It is shown to the user: an editor that cannot offer to clear a
	// folder-borne mark says where the mark came from instead, and the note the
	// person wrote in shelf.json is the better answer than the path alone.
	Reason string
}

// NSFWRules has no built-in list, because only the user knows what their own
// folders hold: the zero value marks nothing.
//
// A rule marks its folder and every folder below it. The prefixes are
// normalized once, when the shelf is opened, because a listing asks this
// question once per book and re-parsing shelf.json a thousand times is the cost
// this exists to avoid.
type NSFWRules struct {
	// byPath is keyed by the normalized, case-folded path (see foldSegment). It
	// is never written after NewNSFWRules returns, so an NSFWRules can be copied
	// and read from several goroutines.
	byPath map[string]NSFWFolder
}

// NewNSFWRules matches paths without regard to case and drops an entry whose
// path is unusable; reporting a bad entry is the caller's job - see
// ValidateNSFWFolderPath.
func NewNSFWRules(folders []NSFWFolder) NSFWRules {
	byPath := make(map[string]NSFWFolder, len(folders))
	for _, folder := range folders {
		key, err := normalizeFolderPath(folder.Path)
		if err != nil {
			continue
		}
		byPath[key] = folder
	}
	if len(byPath) == 0 {
		return NSFWRules{}
	}
	return NSFWRules{byPath: byPath}
}

// Match returns the listed rule that marks this path, walking down from the
// root so the shallowest one wins: that is the rule that would still mark the
// folder if every deeper entry were removed, and it is the one to name when
// telling a user where a mark came from.
func (r NSFWRules) Match(folders []string) (NSFWFolder, bool) {
	if len(r.byPath) == 0 {
		return NSFWFolder{}, false
	}

	var key strings.Builder
	for _, segment := range folders {
		if key.Len() > 0 {
			key.WriteByte('/')
		}
		key.WriteString(foldSegment(segment))
		if folder, ok := r.byPath[key.String()]; ok {
			return folder, true
		}
	}
	return NSFWFolder{}, false
}

// IsNSFWFolder asks whether any prefix of the path is listed, since a rule
// marks its own folder and everything below it.
func (r NSFWRules) IsNSFWFolder(folders []string) bool {
	_, ok := r.Match(folders)
	return ok
}

// Paths returns the configured paths as written, sorted, for logging.
func (r NSFWRules) Paths() []string {
	paths := make([]string, 0, len(r.byPath))
	for _, folder := range r.byPath {
		paths = append(paths, folder.Path)
	}
	slices.Sort(paths)
	return paths
}

// ValidateNSFWFolderPath accepts exactly what NewNSFWRules keeps, so a caller
// can say what was wrong with an entry before the rules silently drop it.
func ValidateNSFWFolderPath(folderPath string) error {
	_, err := normalizeFolderPath(folderPath)
	return err
}

// normalizeFolderPath turns a written folder path into the folded key the rules
// are stored under. Empty segments are dropped, so a leading slash, a trailing
// slash and a doubled separator all name the same folder; a path with no
// segment at all is refused, because "" would mark the whole shelf and is far
// more likely to be an empty field than a decision.
func normalizeFolderPath(folderPath string) (string, error) {
	segments := strings.Split(folderPath, "/")
	folded := make([]string, 0, len(segments))
	for _, segment := range segments {
		if segment == "" {
			continue
		}
		if err := ValidateFolderSegment(segment); err != nil {
			return "", util.Errorf("invalid folder path %q: %w", folderPath, err)
		}
		folded = append(folded, foldSegment(segment))
	}
	if len(folded) == 0 {
		return "", util.Errorf("folder path %q names no folder", folderPath)
	}
	return strings.Join(folded, "/"), nil
}

// foldSegment makes one path segment a map key that two spellings differing
// only in case share.
//
// Neither half alone is enough. Lowercasing misses Greek final sigma - "Σ"
// lowercases to "σ" while "ς" lowercases to itself - so a folder rule is
// silently missed. Simple case folding deliberately leaves "İ" alone, so a
// Turkish folder would stop matching its lowercase spelling. Doing both only
// ever merges spellings Unicode already calls the same letter, and merging is
// the conservative direction for this rule.
//
// IgnoreRules still keys on the lowercase name alone: that is shipped behavior
// deciding which directories a shelf skips, so changing it is its own change.
func foldSegment(segment string) string {
	return strings.Map(func(r rune) rune { return minFold(unicode.ToLower(r)) }, segment)
}

// minFold gives every rune in one folding orbit the same representative.
func minFold(r rune) rune {
	smallest := r
	for folded := unicode.SimpleFold(r); folded != r; folded = unicode.SimpleFold(folded) {
		if folded < smallest {
			smallest = folded
		}
	}
	return smallest
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

// ValidateBookFormat keeps empty valid: books created through the API rather
// than an import carry no format, and the reader treats that as plain text.
func ValidateBookFormat(format string) bool {
	switch format {
	case "", BookFormatText, BookFormatMarkdown:
		return true
	default:
		return false
	}
}

func ValidateSourceID(sourceID string) error {
	if err := ValidatePathSegment(sourceID); err != nil {
		return util.Errorf("invalid source id %q: %w", sourceID, err)
	}
	return nil
}

// ValidatePathSegment covers something the shelf writes - a source ID, a book
// ID, an asset name: structurally usable and not hidden.
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

// ValidateFolderSegment stops short of the hidden-name rule above, which is why
// folders use it: a hidden directory is not a folder either, but that is a
// scanner rule, and whoever holds the shelf's IgnoreRules can say so with the
// reason attached rather than reporting a malformed segment.
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
