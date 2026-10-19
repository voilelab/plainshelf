// Package scancache stores a shelf's directory scan snapshot under app/, so a
// full walk can replace ReadDir with a single Stat for every directory that has
// not changed since the previous walk. Everything arrives through Config, which
// is why it holds no *Shelf.
//
// A directory's mtime changes when a direct child is added, removed or renamed,
// but says nothing about subdirectories, so per-book staleness is still the
// per-book stat in Book.IsStale. The snapshot is disposable runtime state: a
// missing, unreadable or too-new file is a cache miss, never an error.
package scancache

import (
	"io/fs"
	"slices"
	"time"
)

// schemaVersion is the snapshot format this build writes. There is no migration
// path: the file is derived from the shelf and can always be thrown away.
//
// v2 records the kept files (Config.KeepFiles); a v1 snapshot never listed them,
// so reusing one would hide a folder.json until its directory changed.
const schemaVersion = 2

// FileName describes the filesystem, not the machine, so installations sharing
// a shelf write the same answer and every entry is re-validated against the
// real mtime before use.
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

// Stats records what one walk cost, so the effect of the cache is observable.
// ReadDirs + ReusedDirs can be below Dirs when a listing fails or the cache is
// off.
type Stats struct {
	Dirs       int
	ReadDirs   int
	ReusedDirs int

	Duration time.Duration
}

// dirChildren keeps directories, symlinks that may turn out to be one, and the
// regular files named in keep.
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
