// Package scancache stores the directory scan snapshot of a shelf under app/,
// so a full walk can replace a ReadDir with a single Stat for every directory
// that has not changed since the previous walk.
//
// The package is deliberately independent of the shelf that owns it. It knows
// how to remember a directory's mtime and children, when a recorded listing may
// be believed, and how to persist the snapshot; it does not know how a shelf is
// scanned, how a book package is recognized, or when a scan is ready.
// Everything it needs from the outside arrives through Config: a Store for its
// file under app/, whether the cache is enabled, and a logger. That is why the
// cache no longer holds a *Shelf.
package scancache

import (
	"io/fs"
	"time"
)

/*
The directory scan cache makes a full scan cheaper without making it less
frequent.

A directory's modification time changes whenever one of its DIRECT children is
added, removed or renamed. So a walk that remembers each directory's mtime
alongside the children it found there can, on the next walk, replace ReadDir
with a single Stat for every directory that has not changed. On a shelf where
most folders sit untouched between scans - the normal case - the cost of a full
scan drops from one ReadDir per directory to one Stat per directory plus a
ReadDir for the few that actually changed. That is "polling, but cheaper", not
"stop polling", so an SMB or cloud mount benefits exactly like a local disk.

What it deliberately does NOT do is decide whether a book changed. A directory's
mtime says nothing about what happened inside its subdirectories, so everything
under a {book}.bookpkg folder - book.json above all - is still checked by the
per-book stat in Book.IsStale. This cache only answers "which entries does this
directory hold".

The snapshot is runtime state, not shelf data: it lives under app/, is rewritten
whole, and every reader treats a missing, unreadable or too-new file as a plain
cache miss and walks the shelf the long way.
*/

// schemaVersion is the snapshot format this build writes. As with the exported
// book cache there is no migration path and none is needed: the file is derived
// from the shelf and can always be thrown away.
const schemaVersion = 1

// FileName is the snapshot under app/. Unlike the exported book cache it is not
// named per installation: it describes the filesystem rather than the machine
// that read it, so two installations sharing a shelf write the same answer, and
// every entry is re-validated against the real mtime before it is used. A
// foreign snapshot can therefore only ever cost a cache miss.
//
// It is exported so a shelf that owns the file can locate it - the facade builds
// its Store under app/ and its integration tests find it there.
const FileName = "scan-cache.json"

// DirChild is one child of a scanned directory that the walk may descend into.
//
// Plain files are dropped rather than recorded: the walk only ever descends
// into directories, so a snapshot that keeps them would grow with content it
// never consults. Adding a file still changes the parent's mtime and still
// costs the directory its cached entry, which is correct and cheap.
type DirChild struct {
	Name string `json:"name"`

	// Symlink marks a child whose directory entry describes the link rather
	// than what it points at, so its type has to be resolved with a Stat. See
	// ChildIsDir.
	Symlink bool `json:"symlink,omitempty"`
}

// dirSnapshot is one directory as a walk found it.
type dirSnapshot struct {
	// ModTime is the directory's mtime at the moment its children were read.
	// It is the whole validity condition: an identical mtime on the next walk
	// means no direct child was added, removed or renamed since.
	ModTime time.Time `json:"mtime"`

	Children []DirChild `json:"children"`
}

// scanCacheFile is the on-disk shape of app/scan-cache.json.
type scanCacheFile struct {
	SchemaVersion int `json:"schema_version"`

	// Generator records the build that wrote the file, for diagnostics only.
	Generator string `json:"generator"`

	// Dirs maps a directory path relative to the shelf root ("books/Fiction")
	// to the entries the walk found there.
	Dirs map[string]dirSnapshot `json:"dirs"`
}

// Stats records what one walk of the shelf tree cost. Kept so that the effect
// of this cache is observable - in the debug log after every full scan, and in
// the tests that measure it - rather than only inferable from wall time.
type Stats struct {
	// Dirs is how many directories the walk descended into, ReadDirs how many
	// of those it had to list, and ReusedDirs how many it answered from the
	// snapshot. ReadDirs + ReusedDirs can be lower than Dirs when a listing
	// fails or the cache is off.
	Dirs       int
	ReadDirs   int
	ReusedDirs int

	Duration time.Duration
}

// dirChildren keeps the entries a shelf walk can descend into: directories, and
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
