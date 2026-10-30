package shelf

import (
	"slices"
	"strings"

	"github.com/voilelab/plainshelf/internal/fsutil"
)

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
