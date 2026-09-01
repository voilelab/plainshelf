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
// is required; Logger defaults to one that discards.
type Config struct {
	// Store reads and writes the cache's file under app/.
	Store appcache.Store

	// Enabled turns the cache on. When false the walk lists every directory and
	// nothing is persisted - the escape hatch for a mount whose mtimes cannot be
	// trusted.
	Enabled bool

	// Logger receives diagnostics; pass the shelf's. Optional.
	Logger logutil.Logger
}

// Cache is one process's directory scan snapshot for a shelf: the entries the
// last complete walk left behind, kept in memory across walks and persisted
// under app/. Its own mutex guards the snapshot - deliberately not the book
// cache's lock, since publishing the two is unrelated.
type Cache struct {
	store   appcache.Store
	enabled bool
	logger  logutil.Logger

	mu sync.RWMutex

	// dirs is the last complete walk's snapshot; digest fingerprints the copy on
	// disk so an unchanged shelf is never rewritten. lastStats is what the most
	// recent walk cost.
	dirs      map[string]dirSnapshot
	digest    string
	lastStats Stats
}

// Open builds the cache and loads the previous run's snapshot, so the first walk
// - the one the user waits for at startup - is already cheap. Every load failure
// is a cache miss the walk rebuilds, so Open never fails.
func Open(cfg Config) *Cache {
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

// LastStats reports what the most recent walk cost.
func (c *Cache) LastStats() Stats {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.lastStats
}

// Walk is one traversal's view of the snapshot: what the previous walk left
// behind, and what this one records for the next. Take one with NewWalk, drive
// it with ReadDir, and publish it with Install.
type Walk struct {
	cache     *Cache
	enabled   bool
	verify    bool
	startedAt time.Time

	prev map[string]dirSnapshot
	next map[string]dirSnapshot

	stats      Stats
	mismatches []Mismatch
}

// NewWalk starts a traversal, reading the last complete walk's snapshot so this
// one can reuse the directories that have not changed.
func (c *Cache) NewWalk() *Walk {
	return c.newWalk(false)
}

// NewVerifyingWalk starts a traversal that lists every directory for real, even
// the ones the snapshot says it could skip, and records the ones the snapshot
// describes wrongly. Read them back with Mismatches.
//
// It costs a full ReadDir per directory - the whole cost the cache exists to
// avoid - so it belongs only on a walk a user asked for. Nothing else changes:
// the trust chain is evaluated exactly as an ordinary walk evaluates it, and a
// disabled cache has no snapshot to check, so the walk is then indistinguishable
// from an ordinary one.
func (c *Cache) NewVerifyingWalk() *Walk {
	return c.newWalk(true)
}

func (c *Cache) newWalk(verify bool) *Walk {
	w := &Walk{
		cache:     c,
		enabled:   c.enabled,
		verify:    verify,
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

// Mismatches reports the directories a verifying walk found the snapshot
// describing wrongly, in the order the walk reached them. Always empty for an
// ordinary walk and for a disabled cache.
func (w *Walk) Mismatches() []Mismatch {
	return w.mismatches
}

// ReadDir returns the children of pth the walk may descend into, listing pth
// through root only when its mtime shows the listing could have changed. It also
// reports whether that listing is proven unchanged - the trusted flag the caller
// passes down to those children.
//
// trusted says the caller established pth is still the directory the snapshot
// describes. An mtime identifies content, not the directory: swap one directory
// for another carrying the same mtime and matching the mtime alone would serve
// the old children forever. The parent rules that out - renaming a directory
// into place changes the parent's mtime, so only a parent whose listing was
// reused may let a child's recorded mtime be believed; a relisted parent
// distrusts its whole subtree. The walk's root is trusted by definition.
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

	// Stat before the listing, never after: reading the other way could pair a
	// newer mtime with older content and have the next walk trust it. This order
	// only ever costs a needless re-read.
	var modTime time.Time
	var reusable *dirSnapshot
	if info, err := root.Stat(pth); err == nil {
		modTime = info.ModTime()

		if prev, ok := w.prev[pth]; trusted && ok && prev.ModTime.Equal(modTime) {
			if !w.verify {
				w.next[pth] = prev
				w.stats.ReusedDirs++
				return prev.Children, true, nil
			}
			reusable = &prev
		}
	}

	readAt := time.Now()
	entries, err := root.ReadDir(pth)
	if err != nil {
		return nil, false, util.Errorf("%w", err)
	}
	w.stats.ReadDirs++

	children := dirChildren(entries)
	// Only remember a directory whose mtime is settled; see fsutil.RacyWindow.
	if !modTime.IsZero() && modTime.Before(readAt.Add(-fsutil.RacyWindow)) {
		w.next[pth] = dirSnapshot{ModTime: modTime, Children: children}
	}

	if reusable == nil {
		return children, false, nil
	}

	// A verifying walk reached a directory an ordinary walk would have skipped.
	// The listing it read replaces the snapshot's above rather than sitting
	// beside it: this walk knows what the directory holds, and recording the
	// listing it just proved wrong would be storing a known lie. That is not the
	// automatic repair this diagnosis stays out of - scan_cache is untouched and
	// nothing else is invalidated - it is only this walk declining to write down
	// something it can see is false.
	w.stats.CheckedDirs++

	// Stat again before believing the difference. A working filesystem may have
	// changed this directory between the Stat above and the listing - a sync
	// client finishing its copy while the user presses the button, which is
	// exactly when it is most likely - and the listing would then disagree with a
	// snapshot that was accurate when it was read. Blaming the mount for that
	// would send a user to turn off a setting that is doing its job. A
	// modification time that has moved is the mount reporting the change, so
	// there is nothing to diagnose; one that has not moved is the fault itself.
	// A directory that has gone is not evidence of anything either.
	if after, err := root.Stat(pth); err != nil || !after.ModTime().Equal(modTime) {
		return children, true, nil
	}

	if missing, stale := diffChildren(reusable.Children, children); len(missing) > 0 || len(stale) > 0 {
		w.mismatches = append(w.mismatches, Mismatch{Dir: pth, Missing: missing, Stale: stale})
	}

	// Trusted for the same reason the reuse above is: this directory's mtime is
	// the one the snapshot recorded and its parent's listing was itself trusted,
	// so the children below may still have their recorded mtimes believed - and
	// checking only the top of the tree would miss the directory the new book
	// was actually dropped into.
	return children, true, nil
}

// Install publishes what the walk learned. A walk stopped early (complete=false)
// is dropped: its snapshot covers only the tree it reached, and installing it
// would evict directories it never visited. The stats are recorded either way.
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

// Save writes the snapshot when it differs from the one on disk. Offered after
// the initial scan and at Close, not after every walk: the digest check turns an
// unchanged shelf into no write at all.
func (c *Cache) Save() error {
	if !c.enabled {
		return nil
	}

	// A read-only shelf silently no-ops: the snapshot is rebuildable state it was
	// never going to keep, so this is not worth a warning at every startup.
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

	// Atomic, like every file under app/: a sync client must never copy away half
	// a snapshot.
	if err := c.store.WriteFileAtomic(FileName, data); err != nil {
		return util.Errorf("%w", err)
	}

	c.mu.Lock()
	c.digest = digest
	c.mu.Unlock()

	c.logger.Debug("wrote the directory scan cache", "file", FileName, "dirs", len(dirs))
	return nil
}

// ChildIsDir reports whether pth is a directory, paying for a Stat through root
// only when the directory entry could not answer. A listing already reports each
// entry's type, but a symlink's type describes the link, so a link to a
// directory falls back to Stat. child is nil at the root of a walk.
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
