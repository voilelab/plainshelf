package fingerprint

import (
	"bytes"
	"encoding/json/v2"
	"log/slog"
	"maps"
	"strings"
	"sync"
	"time"

	"github.com/voilelab/plainshelf/internal/appcache"
	"github.com/voilelab/plainshelf/internal/fsutil"
	"github.com/voilelab/plainshelf/internal/hashutil"
	"github.com/voilelab/plainshelf/internal/jsonopt"
	"github.com/voilelab/plainshelf/internal/logutil"
	"github.com/voilelab/plainshelf/internal/util"
	"github.com/voilelab/plainshelf/internal/version"
	"github.com/voilelab/plainshelf/shelf/bookpkg"
)

// indexEntry is what one Stat has to match for a source to be answered without
// opening it.
type indexEntry struct {
	Size    int64     `json:"size"`
	ModTime time.Time `json:"mtime"`

	// MD5 is the hash of the content that stat described, which is the key the
	// fingerprint itself is stored under.
	MD5 string `json:"md5"`

	// SeenAt is when these values were FIRST observed, and orders two machines'
	// records of the same source. A run that re-observes the same values carries
	// the original timestamp forward, so an untouched shelf is never rewritten.
	SeenAt time.Time `json:"seen_at,omitzero"`
}

func (e indexEntry) matches(stat *bookpkg.FileStat) bool {
	return stat != nil && e.Size == stat.Size && e.ModTime.Equal(stat.ModTime)
}

func (e indexEntry) describesSame(other indexEntry) bool {
	return e.Size == other.Size && e.ModTime.Equal(other.ModTime) && e.MD5 == other.MD5
}

func (e indexEntry) supersedes(other indexEntry) bool {
	if !e.SeenAt.Equal(other.SeenAt) {
		return e.SeenAt.After(other.SeenAt)
	}
	return e.ModTime.After(other.ModTime)
}

// cacheFile is the on-disk shape of app/fingerprint-cache.json.
type cacheFile struct {
	SchemaVersion int `json:"schema_version"`

	// Generator is for diagnostics only.
	Generator string `json:"generator"`

	Algo    Algo                  `json:"algo"`
	Index   map[string]indexEntry `json:"index"`
	Entries map[string]Entry      `json:"entries"`
}

type RepairHashFunc func(source *bookpkg.Source, contentMD5 string) (bool, error)

type Config struct {
	Store appcache.Store

	Algo Algo

	Logger logutil.Logger

	LiveBooks func() (map[string]struct{}, bool)

	RepairHash RepairHashFunc
}

type Cache struct {
	store      appcache.Store
	algo       Algo
	logger     logutil.Logger
	liveBooks  func() (map[string]struct{}, bool)
	repairHash RepairHashFunc

	mu      sync.Mutex
	index   map[string]indexEntry
	entries map[string]Entry
	stats   Stats

	resolved map[string]struct{}
}

func Open(cfg Config) (*Cache, error) {
	if !cfg.Algo.complete() {
		return nil, util.Errorf("%w", ErrIncompleteAlgo)
	}

	logger := cfg.Logger
	if logger.Logger == nil {
		logger = logutil.Logger{Logger: slog.New(slog.DiscardHandler)}
	}

	cache := &Cache{
		store:      cfg.Store,
		algo:       cfg.Algo,
		logger:     logger,
		liveBooks:  cfg.LiveBooks,
		repairHash: cfg.RepairHash,
		index:      map[string]indexEntry{},
		entries:    map[string]Entry{},
		resolved:   map[string]struct{}{},
	}

	stored, _, err := cache.readFile()
	switch {
	case err != nil:
		cache.logger.Debug("no fingerprint cache to load", "file", cacheFileName, "error", err)
	case stored.Algo != cfg.Algo:
		cache.logger.Debug("discarding a fingerprint cache built with different rules", "file", cacheFileName, "stored", stored.Algo, "wanted", cfg.Algo)
	default:
		cache.index = stored.Index
		cache.entries = stored.Entries
		cache.logger.Debug("loaded the fingerprint cache", "file", cacheFileName, "index", len(stored.Index), "entries", len(stored.Entries))
	}

	return cache, nil
}

type BookSource struct {
	BookID   string
	SourceID string
}

func (c *Cache) CoverageFor(books []BookSource) Coverage {
	fingerprinted := 0
	for _, book := range books {
		if _, ok := c.Lookup(book.BookID, book.SourceID); ok {
			fingerprinted++
		}
	}

	return Coverage{
		Total:         len(books),
		Fingerprinted: fingerprinted,
		Missing:       len(books) - fingerprinted,
		Algo:          c.algo,
	}
}

func (c *Cache) Stats() Stats {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.stats
}

func (c *Cache) Resolve(book *bookpkg.Book, source *bookpkg.Source, build Builder) (Entry, error) {
	return c.resolve(book, source, build, false)
}

func (c *Cache) Rebuild(book *bookpkg.Book, source *bookpkg.Source, build Builder) (Entry, error) {
	return c.resolve(book, source, build, true)
}

func (c *Cache) resolve(book *bookpkg.Book, source *bookpkg.Source, build Builder, force bool) (Entry, error) {
	if book == nil || source == nil || build == nil {
		return Entry{}, util.NewError("resolving a fingerprint needs a book, a source and a builder")
	}

	key := indexKey(book.ID(), source.ID())

	stat, statErr := source.ContentStat()
	if statErr != nil {
		c.logger.Debug("could not stat a source before fingerprinting it", "book_id", book.ID(), "source_id", source.ID(), "error", statErr)
	} else if !force {
		if entry, ok := c.lookupByStat(key, stat); ok {
			return entry, nil
		}
	}

	readAt := time.Now()
	content, err := source.ReadContent()
	if err != nil {
		return Entry{}, util.Errorf("%w", err)
	}

	md5Hash, err := hashutil.MD5Hash(bytes.NewReader(content))
	if err != nil {
		return Entry{}, util.Errorf("%w", err)
	}

	c.repair(source, md5Hash)

	var entry Entry
	known := false
	if !force {
		entry, known = c.lookupByContent(md5Hash)
	}
	if !known {
		entry, err = build(content)
		if err != nil {
			return Entry{}, util.Errorf("%w", err)
		}

		if entry.Sketch == "" {
			return Entry{}, util.NewError("a fingerprint builder returned no sketch")
		}
	}

	c.record(key, stat, readAt, md5Hash, entry, known)
	return entry, nil
}

func (c *Cache) lookupByStat(key string, stat *bookpkg.FileStat) (Entry, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	indexed, ok := c.index[key]
	if !ok || !indexed.matches(stat) {
		return Entry{}, false
	}

	entry, ok := c.entries[indexed.MD5]
	if !ok {
		return Entry{}, false
	}

	c.stats.StatHits++
	return entry, true
}

func (c *Cache) lookupByContent(md5Hash string) (Entry, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry, ok := c.entries[md5Hash]
	if ok {
		c.stats.ContentHits++
	}
	return entry, ok
}

func (c *Cache) Lookup(bookID, sourceID string) (Entry, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	indexed, ok := c.index[indexKey(bookID, sourceID)]
	if !ok {
		return Entry{}, false
	}
	entry, ok := c.entries[indexed.MD5]
	if !ok {
		return Entry{}, false
	}
	return entry, true
}

func (c *Cache) record(key string, stat *bookpkg.FileStat, readAt time.Time, md5Hash string, entry Entry, known bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !known {
		c.entries[md5Hash] = entry
		c.stats.Built++
	}

	c.resolved[md5Hash] = struct{}{}

	if stat == nil || !stat.ModTime.Before(readAt.Add(-fsutil.RacyWindow)) {
		return
	}

	observed := indexEntry{Size: stat.Size, ModTime: stat.ModTime, MD5: md5Hash, SeenAt: readAt}
	if current, ok := c.index[key]; ok && current.describesSame(observed) {
		observed.SeenAt = current.SeenAt
	}

	c.index[key] = observed
}

func (c *Cache) repair(source *bookpkg.Source, md5Hash string) {
	if c.repairHash == nil {
		return
	}

	repaired, err := c.repairHash(source, md5Hash)
	switch {
	case err != nil:
		c.logger.Debug("could not repair a stale md5_hash", "source", source.FolderPath(), "error", err)
	case repaired:
		c.logger.Warn("repaired an md5_hash that disagreed with the source content", "source", source.FolderPath())

		c.mu.Lock()
		c.stats.Repaired++
		c.mu.Unlock()
	}
}

func (c *Cache) Save() error {
	if err := c.store.EnsureWritable(); err != nil {
		return util.Errorf("%w", err)
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	index := maps.Clone(c.index)
	entries := maps.Clone(c.entries)

	stored, raw, readErr := c.readFile()
	if readErr == nil && stored.Algo == c.algo {
		mergeIndex(index, stored.Index)
		mergeEntries(entries, stored.Entries)
	} else {
		raw = nil
	}

	if c.liveBooks != nil {
		if live, ok := c.liveBooks(); ok {
			pruneCache(index, entries, live, c.resolved)
		}
	}

	data, err := json.Marshal(cacheFile{
		SchemaVersion: schemaVersion,
		Generator:     "plainshelf/" + version.Version,
		Algo:          c.algo,
		Index:         index,
		Entries:       entries,
	}, jsonopt.DiskCompact())
	if err != nil {
		return util.Errorf("%w", err)
	}

	if !bytes.Equal(data, raw) {
		if err := c.store.WriteFileAtomic(cacheFileName, data); err != nil {
			return util.Errorf("%w", err)
		}
		c.logger.Debug("wrote the fingerprint cache", "file", cacheFileName, "index", len(index), "entries", len(entries))
	}

	c.index = index
	c.entries = entries
	return nil
}

func (c *Cache) readFile() (*cacheFile, []byte, error) {
	raw, err := c.store.ReadFile(cacheFileName)
	if err != nil {
		return nil, nil, util.Errorf("%w", err)
	}

	var stored cacheFile
	if err := json.Unmarshal(raw, &stored); err != nil {
		return nil, nil, util.Errorf("%w", err)
	}
	if stored.SchemaVersion != schemaVersion {
		return nil, nil, util.Errorf("fingerprint cache is schema_version %d, this build reads %d", stored.SchemaVersion, schemaVersion)
	}
	if stored.Index == nil {
		stored.Index = map[string]indexEntry{}
	}
	if stored.Entries == nil {
		stored.Entries = map[string]Entry{}
	}

	return &stored, raw, nil
}

func mergeEntries(entries, other map[string]Entry) {
	for md5Hash, entry := range other {
		if _, ok := entries[md5Hash]; !ok {
			entries[md5Hash] = entry
		}
	}
}

func mergeIndex(index, other map[string]indexEntry) {
	for key, candidate := range other {
		if current, ok := index[key]; ok && !candidate.supersedes(current) {
			continue
		}
		index[key] = candidate
	}
}

func pruneCache(index map[string]indexEntry, entries map[string]Entry, live, keep map[string]struct{}) {
	referenced := make(map[string]struct{}, len(index)+len(keep))
	maps.Copy(referenced, keep)
	for key, record := range index {
		bookID, _, ok := splitIndexKey(key)
		if !ok {
			delete(index, key)
			continue
		}
		if _, alive := live[bookID]; !alive {
			delete(index, key)
			continue
		}
		referenced[record.MD5] = struct{}{}
	}

	for md5Hash := range entries {
		if _, ok := referenced[md5Hash]; !ok {
			delete(entries, md5Hash)
		}
	}
}

func indexKey(bookID, sourceID string) string {
	return bookID + "/" + sourceID
}

func splitIndexKey(key string) (bookID, sourceID string, ok bool) {
	bookID, sourceID, ok = strings.Cut(key, "/")
	if !ok || bookID == "" || sourceID == "" {
		return "", "", false
	}
	return bookID, sourceID, true
}
