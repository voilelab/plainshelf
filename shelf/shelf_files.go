package shelf

import (
	"slices"
	"strings"

	"github.com/voilelab/plainshelf/internal/fsutil"
)

// IsShelfFile reports whether name, a clean slash-separated path relative to
// the shelf root ("." for the root), belongs to the shelf itself: books/,
// trash/, shelf.json and the exported book caches in app/. It is what a file
// export of the shelf may serve; lock files, the scan cache and app/tmp/ are
// runtime state of whoever has the shelf open and stay behind, as do writes
// still in progress (fsutil.IsTempName), so a client never copies a half
// written file.
func IsShelfFile(name string) bool {
	if name == "." {
		return true
	}
	top, rest, nested := strings.Cut(name, "/")
	switch top {
	case booksFolder, trashFolder:
		return !nested || !slices.ContainsFunc(strings.Split(rest, "/"), fsutil.IsTempName)
	case shelfConfigFile:
		return !nested
	case appFolder:
		return !nested || (!strings.Contains(rest, "/") &&
			strings.HasPrefix(rest, bookCacheFilePrefix) &&
			strings.HasSuffix(rest, bookCacheFileSuffix))
	}
	return false
}
