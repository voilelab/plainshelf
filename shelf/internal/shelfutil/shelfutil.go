package shelfutil

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/voilelab/plainshelf/internal/fsutil"
	"github.com/voilelab/plainshelf/internal/util"
)

const (
	BookFormatText     = "txt"
	BookFormatMarkdown = "md"
)

// 255 bytes is the common per-component filesystem limit, so a segment that
// passes here is one every target filesystem can hold.
const maxPathSegmentLength = 255

func FileETag(root fsutil.ReadFS, filePath string) string {
	info, err := root.Stat(filePath)
	if err != nil {
		return ""
	}
	return fmt.Sprintf(`W/"%d-%d"`, info.ModTime().UnixNano(), info.Size())
}

const DirIgnoreReasonHidden = "hidden directories are never folders"

type IgnoredDir struct {
	// Name is the directory name as it was written. Matching folds case,
	// because a share exported over SMB may spell "$RECYCLE.BIN" either way.
	Name string

	Reason string
}

func DefaultIgnoredDirs() []IgnoredDir {
	return []IgnoredDir{
		{Name: "@eaDir", Reason: "Synology index and thumbnail sidecar"},
		{Name: "#recycle", Reason: "Synology network recycle bin"},
		{Name: "$RECYCLE.BIN", Reason: "Windows recycle bin, visible over SMB"},
		{Name: "lost+found", Reason: "ext filesystem recovery directory"},
	}
}

type IgnoreRules struct {
	byName map[string]IgnoredDir
}

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

func (r IgnoreRules) Names() []string {
	names := make([]string, 0, len(r.byName))
	for _, dir := range r.byName {
		names = append(names, dir.Name)
	}
	slices.Sort(names)
	return names
}

type NSFWFolder struct {
	Path string

	Reason string
}

type NSFWRules struct {
	byPath map[string]NSFWFolder
}

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

func (r NSFWRules) HasFolders() bool { return len(r.byPath) > 0 }

func (r NSFWRules) IsNSFWFolder(folders []string) bool {
	_, ok := r.Match(folders)
	return ok
}

func (r NSFWRules) Paths() []string {
	paths := make([]string, 0, len(r.byPath))
	for _, folder := range r.byPath {
		paths = append(paths, folder.Path)
	}
	slices.Sort(paths)
	return paths
}

func ValidateNSFWFolderPath(folderPath string) error {
	_, err := normalizeFolderPath(folderPath)
	return err
}

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

// Neither half alone is enough. Lowercasing misses Greek final sigma - "Σ"
// lowercases to "σ" while "ς" lowercases to itself - so a folder rule is
// silently missed. Simple case folding deliberately leaves "İ" alone, so a
// Turkish folder would stop matching its lowercase spelling. Doing both only
// ever merges spellings Unicode already calls the same letter, and merging is
// the conservative direction for this rule.
func foldSegment(segment string) string {
	return strings.Map(func(r rune) rune { return minFold(unicode.ToLower(r)) }, segment)
}

func minFold(r rune) rune {
	smallest := r
	for folded := unicode.SimpleFold(r); folded != r; folded = unicode.SimpleFold(folded) {
		smallest = min(smallest, folded)
	}
	return smallest
}

var bcp47Regex = regexp.MustCompile(`^[a-zA-Z]{2,3}(-[a-zA-Z0-9]{2,8})*$`)

func ValidateBCP47(lang string) bool {
	lang = strings.TrimSpace(lang)
	if lang == "" {
		return true
	}

	return bcp47Regex.MatchString(lang)
}

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

func ValidatePathSegment(segment string) error {
	if err := ValidateFolderSegment(segment); err != nil {
		return err
	}
	if strings.HasPrefix(segment, ".") {
		return util.NewError("path segment cannot start with a dot")
	}
	return nil
}

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
