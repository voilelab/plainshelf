package shelf

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"path"
	"time"

	"github.com/voilelab/plainshelf/internal/fsutil"
	"github.com/voilelab/plainshelf/internal/util"
	"github.com/voilelab/plainshelf/internal/version"
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

// scanCacheSchemaVersion is the snapshot format this build writes. As with the
// exported book cache there is no migration path and none is needed: the file
// is derived from the shelf and can always be thrown away.
const scanCacheSchemaVersion = 1

// scanCacheFileName is the snapshot under app/. Unlike the exported book cache
// it is not named per installation: it describes the filesystem rather than the
// machine that read it, so two installations sharing a shelf write the same
// answer, and every entry is re-validated against the real mtime before it is
// used. A foreign snapshot can therefore only ever cost a cache miss.
const scanCacheFileName = "scan-cache.json"

// scanCacheRacyWindow is how recently a directory may have been modified before
// the walk refuses to remember it.
//
// Timestamps are coarse: ext3 and HFS+ store whole seconds, and a FAT-backed
// share reached over SMB stores two. A directory modified again inside the same
// tick keeps the mtime the walk just recorded, and would then look unchanged
// forever - a book added moments after the scan would never appear. Recording
// only directories whose mtime is already older than this window closes that
// race: a modification that could still share the recorded timestamp leaves the
// directory out of the snapshot, so the next scan reads it normally. This is
// the same "racily clean" rule Git applies to its index.
const scanCacheRacyWindow = 2 * time.Second

// dirChild is one child of a scanned directory that the walk may descend into.
//
// Plain files are dropped rather than recorded: the walk only ever descends
// into directories, so a snapshot that keeps them would grow with content it
// never consults. Adding a file still changes the parent's mtime and still
// costs the directory its cached entry, which is correct and cheap.
type dirChild struct {
	Name string `json:"name"`

	// Symlink marks a child whose directory entry describes the link rather
	// than what it points at, so its type has to be resolved with a Stat. See
	// childIsDir.
	Symlink bool `json:"symlink,omitempty"`
}

// dirSnapshot is one directory as a walk found it.
type dirSnapshot struct {
	// ModTime is the directory's mtime at the moment its children were read.
	// It is the whole validity condition: an identical mtime on the next walk
	// means no direct child was added, removed or renamed since.
	ModTime time.Time `json:"mtime"`

	Children []dirChild `json:"children"`
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

// scanStats records what one walk of the shelf tree cost. Kept so that the
// effect of this cache is observable - in the debug log after every full scan,
// and in the tests that measure it - rather than only inferable from wall time.
type scanStats struct {
	// Dirs is how many directories the walk descended into, ReadDirs how many
	// of those it had to list, and ReusedDirs how many it answered from the
	// snapshot. ReadDirs + ReusedDirs can be lower than Dirs when a listing
	// fails or the cache is off.
	Dirs       int
	ReadDirs   int
	ReusedDirs int

	Duration time.Duration
}

// scanDirCache is one walk's view of the snapshot: the entries the previous
// walk left behind, and the ones this walk is recording for the next.
type scanDirCache struct {
	shelf     *Shelf
	enabled   bool
	startedAt time.Time

	prev map[string]dirSnapshot
	next map[string]dirSnapshot

	stats scanStats
}

func (s *Shelf) newScanDirCache() *scanDirCache {
	c := &scanDirCache{
		shelf:     s,
		enabled:   s.scanCacheEnabled,
		startedAt: time.Now(),
	}

	if !c.enabled {
		return c
	}

	s.bookCache.RLock()
	c.prev = s.bookCache.dirs
	s.bookCache.RUnlock()

	c.next = make(map[string]dirSnapshot, len(c.prev))
	return c
}

// readDir returns the children of pth that the walk may descend into, listing
// the directory only when its mtime shows the listing could have changed. It
// also reports whether that listing is proven unchanged, which is what the
// walk passes down as its children's trusted flag.
//
// trusted says the caller established that pth is still the same directory the
// snapshot describes. A modification time identifies a directory's content,
// not the directory: move one away and rename another into its place and the
// path now holds something else entirely, with an mtime of its own that may
// well equal the recorded one - coarse-timestamp filesystems (the ones this
// cache is most useful on) and timestamp-preserving copies both produce that.
// Matching the mtime alone would then serve the old directory's children
// forever and hide every book under the new one.
//
// What rules that out is the parent. Renaming a directory into place changes
// its new parent's mtime, so a parent whose own listing is proven unchanged
// proves that none of its children were swapped, and only then may a child's
// recorded mtime be believed. A parent that had to be relisted distrusts every
// child, which cascades down that subtree for the same reason - the cost is
// one scan at the old price for the part of the tree that actually changed.
// The walk's root is trusted by definition; it has no parent under the shelf.
func (c *scanDirCache) readDir(pth string, trusted bool) ([]dirChild, bool, error) {
	c.stats.Dirs++

	if !c.enabled {
		entries, err := c.shelf.dbRoot.ReadDir(pth)
		if err != nil {
			return nil, false, util.Errorf("%w", err)
		}
		c.stats.ReadDirs++
		return dirChildren(entries), false, nil
	}

	// Stat before the listing, never after. Read the other way round, a
	// directory modified between the two calls would be recorded with the newer
	// mtime and the older content, and the next walk would trust it. This order
	// can only pair a newer listing with an older mtime, which merely costs the
	// next walk a re-read.
	var modTime time.Time
	if info, err := c.shelf.dbRoot.Stat(pth); err == nil {
		modTime = info.ModTime()

		if prev, ok := c.prev[pth]; trusted && ok && prev.ModTime.Equal(modTime) {
			c.next[pth] = prev
			c.stats.ReusedDirs++
			return prev.Children, true, nil
		}
	}

	readAt := time.Now()
	entries, err := c.shelf.dbRoot.ReadDir(pth)
	if err != nil {
		return nil, false, util.Errorf("%w", err)
	}
	c.stats.ReadDirs++

	// Recorded even when the listing was only re-read because the parent
	// changed: these are the children this directory really holds at this
	// mtime, which is what the next walk needs.
	children := dirChildren(entries)
	if !modTime.IsZero() && modTime.Before(readAt.Add(-scanCacheRacyWindow)) {
		c.next[pth] = dirSnapshot{ModTime: modTime, Children: children}
	}
	return children, false, nil
}

// install publishes what this walk learned. A walk that was stopped early is
// dropped instead: its snapshot describes only the part of the tree it reached,
// and installing it would evict entries for directories it never visited.
func (c *scanDirCache) install(complete bool) scanStats {
	c.stats.Duration = time.Since(c.startedAt)

	c.shelf.bookCache.Lock()
	defer c.shelf.bookCache.Unlock()

	if c.enabled && complete {
		c.shelf.bookCache.dirs = c.next
	}
	c.shelf.bookCache.lastScanStats = c.stats

	return c.stats
}

// dirChildren keeps the entries a shelf walk can descend into: directories, and
// symlinks that may turn out to be directories.
func dirChildren(entries []fs.DirEntry) []dirChild {
	children := make([]dirChild, 0, len(entries))
	for _, entry := range entries {
		symlink := entry.Type()&fs.ModeSymlink != 0
		if !symlink && !entry.IsDir() {
			continue
		}
		children = append(children, dirChild{Name: entry.Name(), Symlink: symlink})
	}
	return children
}

// childIsDir reports whether pth is a directory, paying for a Stat call only
// when the directory entry could not answer on its own.
//
// A listing already reports each entry's type, so a walk does not need to stat
// every child it visits; on a network shelf that removes roughly half the round
// trips of a full scan. The exception is a symlink: the readdir type byte
// describes the link itself, so a link pointing at a directory reports false.
// Those fall back to Stat, which resolves the link exactly as the walk always
// did - including when the entry came from the scan cache, because a symlink's
// target can change without the directory holding it being modified.
//
// child is nil at the root of a walk, which has no directory entry of its own.
func childIsDir(root fsutil.ReadFS, pth string, child *dirChild) (bool, error) {
	if child != nil && !child.Symlink {
		return true, nil
	}

	info, err := root.Stat(pth)
	if err != nil {
		return false, util.Errorf("%w", err)
	}

	return info.IsDir(), nil
}

// lastScanStats reports what the most recent walk of the shelf tree cost.
func (s *Shelf) lastScanStats() scanStats {
	s.bookCache.RLock()
	defer s.bookCache.RUnlock()
	return s.bookCache.lastScanStats
}

// loadScanCache seeds the in-memory snapshot from app/scan-cache.json so that
// the very first scan of a process - the one the user waits for at startup -
// is already cheap.
//
// Every failure here is a cache miss, never an error: the file is absent on a
// fresh shelf, may have been written by a build that is not this one, and is
// rebuilt for free by the walk that is about to run anyway.
func (s *Shelf) loadScanCache() {
	if !s.scanCacheEnabled {
		return
	}

	filePath := path.Join(appFolder, scanCacheFileName)

	file, err := s.dbRoot.Open(filePath)
	if err != nil {
		s.Debug("no directory scan cache to load", "path", filePath, "error", err)
		return
	}
	defer file.Close()

	var snapshot scanCacheFile
	if err := json.NewDecoder(file).Decode(&snapshot); err != nil {
		s.Debug("ignoring an unreadable directory scan cache", "path", filePath, "error", err)
		return
	}
	if snapshot.SchemaVersion != scanCacheSchemaVersion || snapshot.Dirs == nil {
		s.Debug("ignoring a directory scan cache this build does not read", "path", filePath, "schema_version", snapshot.SchemaVersion)
		return
	}

	digest, err := scanCacheDigest(snapshot.Dirs)
	if err != nil {
		s.Debug("ignoring a directory scan cache that could not be fingerprinted", "path", filePath, "error", err)
		return
	}

	s.bookCache.Lock()
	s.bookCache.dirs = snapshot.Dirs
	s.bookCache.scanCacheDigest = digest
	s.bookCache.Unlock()

	s.Debug("loaded the directory scan cache", "path", filePath, "dirs", len(snapshot.Dirs))
}

// saveScanCache writes the snapshot when it differs from the one on disk.
//
// It is offered after the initial scan and again at Close, not after every full
// scan. Two reasons: a rescan requested from the UI is documented to write
// nothing to the shelf, and a shelf whose folders are not changing produces the
// same snapshot every time, which the digest check turns into no write at all.
func (s *Shelf) saveScanCache() error {
	if !s.scanCacheEnabled {
		return nil
	}

	// Not an error worth reporting on a read-only shelf: the snapshot is
	// rebuildable runtime state that this shelf was never going to keep, and
	// both call sites would only log a warning at every startup and shutdown.
	if s.readOnly {
		return nil
	}

	root, err := s.writeRoot()
	if err != nil {
		return util.Errorf("%w", err)
	}

	s.bookCache.RLock()
	dirs := s.bookCache.dirs
	lastDigest := s.bookCache.scanCacheDigest
	s.bookCache.RUnlock()

	if len(dirs) == 0 {
		return nil
	}

	digest, err := scanCacheDigest(dirs)
	if err != nil {
		return util.Errorf("%w", err)
	}
	if digest == lastDigest {
		return nil
	}

	data, err := json.Marshal(scanCacheFile{
		SchemaVersion: scanCacheSchemaVersion,
		Generator:     "plainshelf/" + version.Version,
		Dirs:          dirs,
	})
	if err != nil {
		return util.Errorf("%w", err)
	}

	// Atomic, like every other file under app/: a sync client copying the shelf
	// away must never pick up half a snapshot.
	filePath := path.Join(appFolder, scanCacheFileName)
	if err := fsutil.WriteFileAtomic(root, filePath, data); err != nil {
		return util.Errorf("%w", err)
	}

	s.bookCache.Lock()
	s.bookCache.scanCacheDigest = digest
	s.bookCache.Unlock()

	s.Debug("wrote the directory scan cache", "path", filePath, "dirs", len(dirs))
	return nil
}

// scanCacheDigest fingerprints the snapshot so an unchanged shelf does not
// rewrite the file. encoding/json sorts map keys, so the result is stable.
func scanCacheDigest(dirs map[string]dirSnapshot) (string, error) {
	data, err := json.Marshal(dirs)
	if err != nil {
		return "", util.Errorf("%w", err)
	}

	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}
