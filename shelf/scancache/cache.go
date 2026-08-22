package scancache

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/voilelab/plainshelf/internal/appcache"
	"github.com/voilelab/plainshelf/internal/fsutil"
	"github.com/voilelab/plainshelf/internal/logutil"
	"github.com/voilelab/plainshelf/internal/util"
	"github.com/voilelab/plainshelf/internal/version"
)

// Config is everything the cache needs from the shelf that owns it. Only Store
// is required; the rest default to "do nothing", which is what a read-only or
// standalone reader wants.
type Config struct {
	// Store reads and writes the cache's file under app/.
	Store appcache.Store

	// Enabled turns the cache on. When false the walk lists every directory the
	// long way and nothing is ever persisted - the escape hatch for a mount
	// whose directory mtimes cannot be trusted.
	Enabled bool

	// Logger receives the cache's diagnostics; pass the shelf's. Optional: the
	// zero value has no slog.Logger behind it and is defaulted by Open to one
	// that discards everything, so a caller that supplies only Store still gets
	// a working cache rather than a panic on the first log line.
	Logger logutil.Logger
}

// Cache is one process's directory scan snapshot for a shelf: the entries the
// last complete walk left behind, kept in memory across walks and persisted
// under app/. It holds no *Shelf; everything it needs arrives through Config.
//
// Open it once per shelf, take a Walk for each traversal, and Save when the
// shelf is closed. Its own mutex guards the snapshot, which is deliberately not
// the book cache's lock: publishing the snapshot and publishing the book cache
// are two separate critical sections with no invariant between them.
type Cache struct {
	store   appcache.Store
	enabled bool
	logger  logutil.Logger

	mu sync.RWMutex

	// dirs is the snapshot the last complete walk left behind, and digest
	// fingerprints the copy of it that is on disk so an unchanged shelf is never
	// rewritten.
	dirs   map[string]dirSnapshot
	digest string

	// lastStats is what the most recent walk cost, kept so the effect of the
	// snapshot is observable rather than only inferable.
	lastStats Stats
}

// Open builds the cache and seeds its in-memory snapshot from app/scan-cache.json
// so that the very first scan of a process - the one the user waits for at
// startup - is already cheap.
//
// Every failure while loading is a cache miss, never an error: the file is
// absent on a fresh shelf, may have been written by a build that is not this
// one, and is rebuilt for free by the walk that is about to run anyway. Open
// therefore never fails.
func Open(cfg Config) *Cache {
	// Logger is optional (see Config): a caller that supplies only Store gets a
	// discarding logger rather than a nil slog.Logger that would panic on the
	// first Debug below.
	logger := cfg.Logger
	if logger.Logger == nil {
		logger = logutil.Logger{Logger: slog.New(slog.DiscardHandler)}
	}

	c := &Cache{
		store:   cfg.Store,
		enabled: cfg.Enabled,
		logger:  logger,
	}
	c.load()
	return c
}

// load reads the snapshot from disk. Called once by Open, before the first walk:
// the snapshot from the previous run is what makes that walk cheap. Every
// failure is a cache miss the walk rebuilds, so nothing here is fatal.
func (c *Cache) load() {
	if !c.enabled {
		return
	}

	data, err := c.store.ReadFile(FileName)
	if err != nil {
		c.logger.Debug("no directory scan cache to load", "file", FileName, "error", err)
		return
	}

	var snapshot scanCacheFile
	if err := json.Unmarshal(data, &snapshot); err != nil {
		c.logger.Debug("ignoring an unreadable directory scan cache", "file", FileName, "error", err)
		return
	}
	if snapshot.SchemaVersion != schemaVersion || snapshot.Dirs == nil {
		c.logger.Debug("ignoring a directory scan cache this build does not read", "file", FileName, "schema_version", snapshot.SchemaVersion)
		return
	}

	digest, err := scanCacheDigest(snapshot.Dirs)
	if err != nil {
		c.logger.Debug("ignoring a directory scan cache that could not be fingerprinted", "file", FileName, "error", err)
		return
	}

	c.dirs = snapshot.Dirs
	c.digest = digest

	c.logger.Debug("loaded the directory scan cache", "file", FileName, "dirs", len(snapshot.Dirs))
}

// LastStats reports what the most recent walk of the shelf tree cost.
func (c *Cache) LastStats() Stats {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.lastStats
}

// Walk is one traversal's view of the snapshot: the entries the previous walk
// left behind, and the ones this walk is recording for the next. Take one with
// NewWalk, drive it with ReadDir, and publish what it learned with Install.
type Walk struct {
	cache     *Cache
	enabled   bool
	startedAt time.Time

	prev map[string]dirSnapshot
	next map[string]dirSnapshot

	stats Stats
}

// NewWalk starts a fresh traversal, reading the snapshot the last complete walk
// left behind so this one can reuse the directories that have not changed.
func (c *Cache) NewWalk() *Walk {
	w := &Walk{
		cache:     c,
		enabled:   c.enabled,
		startedAt: time.Now(),
	}

	if !c.enabled {
		return w
	}

	c.mu.RLock()
	w.prev = c.dirs
	c.mu.RUnlock()

	w.next = make(map[string]dirSnapshot, len(w.prev))
	return w
}

// ReadDir returns the children of pth that the walk may descend into, listing
// the directory through root only when its mtime shows the listing could have
// changed. It also reports whether that listing is proven unchanged - the
// trusted flag the walk passes down to those children.
//
// trusted says the caller established that pth is still the same directory the
// snapshot describes. An mtime identifies a directory's content, not the
// directory: move one away and rename another into its place and the path holds
// something else entirely, with an mtime of its own that may well equal the
// recorded one - coarse-timestamp filesystems (the ones this cache is most
// useful on) and timestamp-preserving copies both produce that. Matching the
// mtime alone would then serve the old directory's children forever and hide
// every book under the new one.
//
// The parent rules that out. Renaming a directory into place changes its new
// parent's mtime, so a parent whose own listing is proven unchanged proves none
// of its children were swapped, and only then may a child's recorded mtime be
// believed. A parent that had to be relisted distrusts every child, cascading
// down that subtree for the same reason - the cost is one scan at the old price
// for the part of the tree that actually changed. The walk's root is trusted by
// definition; it has no parent under the shelf.
func (w *Walk) ReadDir(root fsutil.ReadFS, pth string, trusted bool) ([]DirChild, bool, error) {
	w.stats.Dirs++

	if !w.enabled {
		entries, err := root.ReadDir(pth)
		if err != nil {
			return nil, false, util.Errorf("%w", err)
		}
		w.stats.ReadDirs++
		return dirChildren(entries), false, nil
	}

	// Stat before the listing, never after. Read the other way round, a
	// directory modified between the two calls would be recorded with the newer
	// mtime and the older content, and the next walk would trust it. This order
	// can only pair a newer listing with an older mtime, which merely costs the
	// next walk a re-read.
	var modTime time.Time
	if info, err := root.Stat(pth); err == nil {
		modTime = info.ModTime()

		if prev, ok := w.prev[pth]; trusted && ok && prev.ModTime.Equal(modTime) {
			w.next[pth] = prev
			w.stats.ReusedDirs++
			return prev.Children, true, nil
		}
	}

	readAt := time.Now()
	entries, err := root.ReadDir(pth)
	if err != nil {
		return nil, false, util.Errorf("%w", err)
	}
	w.stats.ReadDirs++

	// Recorded even when the listing was only re-read because the parent
	// changed: these are the children this directory really holds at this
	// mtime, which is what the next walk needs.
	children := dirChildren(entries)
	// Only remember directories whose mtime is settled; see fsutil.RacyWindow.
	if !modTime.IsZero() && modTime.Before(readAt.Add(-fsutil.RacyWindow)) {
		w.next[pth] = dirSnapshot{ModTime: modTime, Children: children}
	}
	return children, false, nil
}

// Install publishes what this walk learned into the cache. A walk that was
// stopped early is dropped instead: its snapshot describes only the part of the
// tree it reached, and installing it would evict entries for directories it
// never visited. The walk's stats are recorded whether or not the snapshot is.
func (w *Walk) Install(complete bool) Stats {
	w.stats.Duration = time.Since(w.startedAt)

	c := w.cache
	c.mu.Lock()
	defer c.mu.Unlock()

	if w.enabled && complete {
		c.dirs = w.next
	}
	c.lastStats = w.stats

	return w.stats
}

// Save writes the snapshot when it differs from the one on disk.
//
// It is offered after the initial scan and again at Close, not after every full
// scan. Two reasons: a rescan requested from the UI is documented to write
// nothing to the shelf, and a shelf whose folders are not changing produces the
// same snapshot every time, which the digest check turns into no write at all.
func (c *Cache) Save() error {
	if !c.enabled {
		return nil
	}

	// A read-only shelf is not an error worth reporting: the snapshot is
	// rebuildable runtime state that this shelf was never going to keep, and
	// both call sites would only log a warning at every startup and shutdown.
	if err := c.store.EnsureWritable(); err != nil {
		if errors.Is(err, fsutil.ErrReadOnly) {
			return nil
		}
		return util.Errorf("%w", err)
	}

	c.mu.RLock()
	dirs := c.dirs
	lastDigest := c.digest
	c.mu.RUnlock()

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
		SchemaVersion: schemaVersion,
		Generator:     "plainshelf/" + version.Version,
		Dirs:          dirs,
	})
	if err != nil {
		return util.Errorf("%w", err)
	}

	// Atomic, like every other file under app/: a sync client copying the shelf
	// away must never pick up half a snapshot.
	if err := c.store.WriteFileAtomic(FileName, data); err != nil {
		return util.Errorf("%w", err)
	}

	c.mu.Lock()
	c.digest = digest
	c.mu.Unlock()

	c.logger.Debug("wrote the directory scan cache", "file", FileName, "dirs", len(dirs))
	return nil
}

// ChildIsDir reports whether pth is a directory, paying for a Stat call through
// root only when the directory entry could not answer on its own.
//
// A listing already reports each entry's type, so a walk need not stat every
// child it visits; on a network shelf that removes roughly half a full scan's
// round trips. The exception is a symlink: the readdir type byte describes the
// link itself, so a link pointing at a directory reports false. Those fall back
// to Stat, which resolves the link exactly as the walk always did - including
// when the entry came from the scan cache, because a symlink's target can change
// without the directory holding it being modified.
//
// child is nil at the root of a walk, which has no directory entry of its own.
func ChildIsDir(root fsutil.ReadFS, pth string, child *DirChild) (bool, error) {
	if child != nil && !child.Symlink {
		return true, nil
	}

	info, err := root.Stat(pth)
	if err != nil {
		return false, util.Errorf("%w", err)
	}

	return info.IsDir(), nil
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
