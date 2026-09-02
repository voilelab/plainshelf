package shelf

import "strings"

// FolderPath is a book's position in the shelf's folder tree: the ordered path
// segments from books/ down to the book's own directory. It is a shelf concept,
// not part of a book package — a book opened on its own through bookpkg has no
// folder path — so it lives with the shelf, not with the book read/write layer.
type FolderPath []string

func (l FolderPath) String() string {
	return strings.Join(l, "/")
}

func (l FolderPath) Equal(other FolderPath) bool {
	if len(l) != len(other) {
		return false
	}
	for i := range l {
		if l[i] != other[i] {
			return false
		}
	}
	return true
}
