package shelf

import (
	"slices"
	"strings"
)

// FolderPath is a book's position in the shelf's folder tree: the ordered path
// segments from books/ down to the book's own directory. It is a shelf concept,
// not part of a book package — a book opened on its own through bookpkg has no
// folder path — so it lives with the shelf, not with the book read/write layer.
type FolderPath []string

func (l FolderPath) String() string {
	return strings.Join(l, "/")
}

func (l FolderPath) Equal(other FolderPath) bool {
	return slices.Equal(l, other)
}

// HasPrefix reports whether l is prefix itself or sits beneath it.
func (l FolderPath) HasPrefix(prefix FolderPath) bool {
	return len(l) >= len(prefix) && slices.Equal(l[:len(prefix)], prefix)
}
