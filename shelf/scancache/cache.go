package scancache

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json/v2"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/voilelab/plainshelf/internal/appcache"
	"github.com/voilelab/plainshelf/internal/fsutil"
	"github.com/voilelab/plainshelf/internal/jsonopt"
	"github.com/voilelab/plainshelf/internal/logutil"
	"github.com/voilelab/plainshelf/internal/util"
	"github.com/voilelab/plainshelf/internal/version"
)

type Config struct {
	Store appcache.Store

	Enabled bool

	Logger logutil.Logger

	KeepFiles []string
}

type Cache struct {
	store     appcache.Store
	enabled   bool
	logger    logutil.Logger
	keepFiles []string

	mu sync.RWMutex

	dirs      map[string]dirSnapshot
	digest    string
	lastStats Stats
}

func Open(cfg Config) *Cache {
	logger := cfg.Logger
	if logger.Logger == nil {
		logger = logutil.Logger{Logger: slog.New(slog.DiscardHandler)}
	}

	c := &Cache{
		store:     cfg.Store,
		enabled:   cfg.Enabled,
		logger:    logger,
		keepFiles: cfg.KeepFiles,
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

func (c *Cache) LastStats() Stats {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.lastStats
}

type Walk struct {
	cache     *Cache
	enabled   bool
	startedAt time.Time

	prev map[string]dirSnapshot
	next map[string]dirSnapshot

	stats Stats
}

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

func (w *Walk) ReadDir(root fsutil.ReadFS, pth string, trusted bool) ([]DirChild, bool, error) {
	w.stats.Dirs++

	if !w.enabled {
		entries, err := root.ReadDir(pth)
		if err != nil {
			return nil, false, util.Errorf("%w", err)
		}
		w.stats.ReadDirs++
		return dirChildren(entries, w.cache.keepFiles), false, nil
	}

	// Stat before the listing, never after: reading the other way could pair a
	// newer mtime with older content and have the next walk trust it. This order
	// only ever costs a needless re-read.
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

	children := dirChildren(entries, w.cache.keepFiles)
	if !modTime.IsZero() && modTime.Before(readAt.Add(-fsutil.RacyWindow)) {
		w.next[pth] = dirSnapshot{ModTime: modTime, Children: children}
	}
	return children, false, nil
}

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

func (c *Cache) Save() error {
	if !c.enabled {
		return nil
	}

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
	}, jsonopt.DiskCompact())
	if err != nil {
		return util.Errorf("%w", err)
	}

	if err := c.store.WriteFileAtomic(FileName, data); err != nil {
		return util.Errorf("%w", err)
	}

	c.mu.Lock()
	c.digest = digest
	c.mu.Unlock()

	c.logger.Debug("wrote the directory scan cache", "file", FileName, "dirs", len(dirs))
	return nil
}

func ChildIsDir(root fsutil.ReadFS, pth string, child *DirChild) (bool, error) {
	if child != nil && child.File {
		return false, nil
	}
	if child != nil && !child.Symlink {
		return true, nil
	}

	info, err := root.Stat(pth)
	if err != nil {
		return false, util.Errorf("%w", err)
	}

	return info.IsDir(), nil
}

func scanCacheDigest(dirs map[string]dirSnapshot) (string, error) {
	data, err := json.Marshal(dirs, jsonopt.DiskCompact())
	if err != nil {
		return "", util.Errorf("%w", err)
	}

	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}
