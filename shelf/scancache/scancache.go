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
	"slices"
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

	// CheckedDirs counts the directories a verifying walk listed for real even
	// though the snapshot said the listing could be reused. It is zero for every
	// ordinary walk, which is what makes "the diagnosis ran" distinguishable
	// from "the diagnosis found nothing".
	CheckedDirs int

	Duration time.Duration
}

// Mismatch reports one directory whose snapshot no longer describes what the
// directory holds, even though its modification time says nothing has changed.
//
// That combination is not a stale cache the next walk will correct: the walk
// decides whether to relist from the modification time alone, so a directory
// that reports an unchanged time while its children change is one the cache can
// never recover from on its own. It is the shape of the cloud storage gateways
// that do not touch a directory's time when a child is added, and the reason a
// book copied onto such a mount is never found.
type Mismatch struct {
	// Dir is the path relative to the shelf root, as the snapshot keys it.
	Dir string

	// Missing names the children the directory holds and the snapshot does not -
	// the books and folders a scan reusing this snapshot would never see.
	Missing []string

	// Stale names the children the snapshot holds and the directory no longer
	// does. Same cause, opposite symptom: a book deleted from outside that goes
	// on being listed.
	Stale []string
}

// diffChildren reports how snapshot differs from actual: what actual holds and
// snapshot does not, and the reverse. Both are nil when the two agree, so a
// consistent directory produces no finding at all.
func diffChildren(snapshot, actual []DirChild) (missing, stale []string) {
	inSnapshot := make(map[string]bool, len(snapshot))
	for _, child := range snapshot {
		inSnapshot[child.Name] = true
	}

	inActual := make(map[string]bool, len(actual))
	for _, child := range actual {
		inActual[child.Name] = true
		if !inSnapshot[child.Name] {
			missing = append(missing, child.Name)
		}
	}

	for _, child := range snapshot {
		if !inActual[child.Name] {
			stale = append(stale, child.Name)
		}
	}

	slices.Sort(missing)
	slices.Sort(stale)
	return missing, stale
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
