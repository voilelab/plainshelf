package shelf

import (
	"slices"
	"strings"
)

type FolderPath []string

func (l FolderPath) String() string {
	return strings.Join(l, "/")
}

func (l FolderPath) Equal(other FolderPath) bool {
	return slices.Equal(l, other)
}

func (l FolderPath) HasPrefix(prefix FolderPath) bool {
	return len(l) >= len(prefix) && slices.Equal(l[:len(prefix)], prefix)
}
