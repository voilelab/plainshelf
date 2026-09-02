// Package scancache stores a shelf's directory scan snapshot under app/, so a
// full walk can replace ReadDir with a single Stat for every directory that has
// not changed since the previous walk.
//
// It is independent of the shelf that owns it: everything arrives through
// Config, and the walk reaches the filesystem through the root the shelf hands
// each ReadDir call. That is why it holds no *Shelf.
//
// A directory's mtime changes when a direct child is added, removed or renamed,
// so a remembered mtime tells the next walk whether the listing can be reused.
// It says nothing about subdirectories, so per-book staleness is still the
// per-book stat in Book.IsStale. The snapshot is disposable runtime state: a
// missing, unreadable or too-new file is a cache miss, never an error.
package scancache

import (
	"io/fs"
	"time"
)

// schemaVersion is the snapshot format this build writes. There is no migration
// path: the file is derived from the shelf and can always be thrown away.
const schemaVersion = 1

// FileName is the snapshot under app/. It describes the filesystem, not the
// machine, so installations sharing a shelf write the same answer and every
// entry is re-validated against the real mtime before use. Exported so the shelf
// that owns the file can locate it.
const FileName = "scan-cache.json"

// DirChild is one child of a scanned directory the walk may descend into. Plain
// files are dropped: the walk never descends into them, and adding one still
// bumps the parent's mtime.
type DirChild struct {
	Name string `json:"name"`

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

// dirChildren keeps the entries a walk can descend into: directories, and
// symlinks that may turn out to be directories.
func dirChildren(entries []fs.DirEntry) []DirChild {
	children := make([]DirChild, 0, len(entries))
	for _, entry := range entries {
		symlink := entry.Type()&fs.ModeSymlink != 0
		if !symlink && !entry.IsDir() {
			continue
		}
		children = append(children, DirChild{Name: entry.Name(), Symlink: symlink})
	}
	return children
}
