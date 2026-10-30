package scancache

import (
	"io/fs"
	"slices"
	"time"
)

const schemaVersion = 2

const FileName = "scan-cache.json"

// DirChild is one child of a scanned directory the walk may descend into, or a
// file the walk was asked to keep (Config.KeepFiles). Every other plain file is
// dropped: the walk never descends into it, and adding one still bumps the
// parent's mtime.
type DirChild struct {
	Name string `json:"name"`

	// File marks a kept regular file. Its content may change without the
	// parent's mtime moving, so the listing says only that it exists.
	File bool `json:"file,omitzero"`

	// Symlink marks a child whose type has to be resolved with a Stat, because
	// its directory entry describes the link rather than its target. See
	// ChildIsDir.
	Symlink bool `json:"symlink,omitzero"`
}

// dirSnapshot is one directory as a walk found it. An identical ModTime on the
// next walk means no direct child was added, removed or renamed since.
type dirSnapshot struct {
	ModTime  time.Time  `json:"mtime"`
	Children []DirChild `json:"children"`
}

// scanCacheFile is the on-disk shape of app/scan-cache.json. Dirs maps a path
// relative to the shelf root ("books/Fiction") to the entries found there.
type scanCacheFile struct {
	SchemaVersion int                    `json:"schema_version"`
	Generator     string                 `json:"generator"`
	Dirs          map[string]dirSnapshot `json:"dirs"`
}

type Stats struct {
	Dirs       int
	ReadDirs   int
	ReusedDirs int

	Duration time.Duration
}

func dirChildren(entries []fs.DirEntry, keep []string) []DirChild {
	children := make([]DirChild, 0, len(entries))
	for _, entry := range entries {
		symlink := entry.Type()&fs.ModeSymlink != 0
		if !symlink && !entry.IsDir() {
			if entry.Type().IsRegular() && slices.Contains(keep, entry.Name()) {
				children = append(children, DirChild{Name: entry.Name(), File: true})
			}
			continue
		}
		children = append(children, DirChild{Name: entry.Name(), Symlink: symlink})
	}
	return children
}
