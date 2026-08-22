package shelf

import "strings"

// Layers is a book's position in the shelf's folder tree: the ordered path
// segments from books/ down to the book's own directory. It is a shelf concept,
// not part of a book package — a book opened on its own through bookpkg has no
// layers — so it lives with the shelf, not with the book read/write layer.
type Layers []string

func (l Layers) String() string {
	return strings.Join(l, "/")
}

func (l Layers) Equal(other Layers) bool {
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

func NewLayersFromString(s string) Layers {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, "/")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}
