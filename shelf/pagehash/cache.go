package pagehash

import (
	"bytes"
	"encoding/json/v2"
	"log/slog"
	"maps"
	"slices"
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

const (
	schemaVersion = 1
	hashName      = "md5"
	cacheFileName = "page-hash-cache.json"
)

var ErrMissingPages = util.NewError("page list names images that are missing")

type pageEntry struct {
	Size    int64     `json:"size"`
	ModTime time.Time `json:"mtime"`
	MD5     string    `json:"md5"`
}

func (e pageEntry) matches(stat *bookpkg.FileStat) bool {
	return !e.ModTime.IsZero() && e.Size == stat.Size && e.ModTime.Equal(stat.ModTime)
}

type sourceEntry struct {
	ListMD5 string               `json:"list_md5"`
	Pages   map[string]pageEntry `json:"pages"`
}

type cacheFile struct {
	SchemaVersion int                    `json:"schema_version"`
	Generator     string                 `json:"generator"`
	Hash          string                 `json:"hash"`
	Sources       map[string]sourceEntry `json:"sources"`
}

type LiveBooksFunc func() (bookIDs map[string]struct{}, complete bool)

type Config struct {
	Store     appcache.Store
	Logger    logutil.Logger
	LiveBooks LiveBooksFunc
}

type Stats struct {
	Pages    int
	StatHits int
	Hashed   int
}

type Cache struct {
	store     appcache.Store
	logger    logutil.Logger
	liveBooks LiveBooksFunc

	mu      sync.Mutex
	sources map[string]sourceEntry
	touched map[string]struct{}
	stats   Stats
}

func Open(cfg Config) *Cache {
	logger := cfg.Logger
	if logger.Logger == nil {
		logger = logutil.Logger{Logger: slog.New(slog.DiscardHandler)}
	}

	cache := &Cache{
		store:     cfg.Store,
		logger:    logger,
		liveBooks: cfg.LiveBooks,
		sources:   map[string]sourceEntry{},
		touched:   map[string]struct{}{},
	}

	stored, _, err := cache.readFile()
	if err != nil {
		cache.logger.Debug("no page hash cache to load", "file", cacheFileName, "error", err)
	} else {
		cache.sources = stored.Sources
	}
	return cache
}

func (c *Cache) Stats() Stats {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.stats
}

func (c *Cache) Recorded(bookID, sourceID, listMD5 string) ([]string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry, ok := c.sources[sourceKey(bookID, sourceID)]
	if !ok || listMD5 == "" || entry.ListMD5 != listMD5 {
		return nil, false
	}
	return distinctHashes(entry.Pages), true
}

func (c *Cache) Resolve(bookID string, source *bookpkg.Source) ([]string, error) {
	list, err := source.Pages()
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	key := sourceKey(bookID, source.ID())
	c.mu.Lock()
	previous := c.sources[key].Pages
	c.mu.Unlock()

	if slices.ContainsFunc(list.Pages, func(page bookpkg.Page) bool { return page.Missing }) {
		c.mu.Lock()
		delete(c.sources, key)
		c.touched[key] = struct{}{}
		c.mu.Unlock()
		return nil, util.Errorf("%w", ErrMissingPages)
	}

	pages := make(map[string]pageEntry, len(list.Pages))
	for _, page := range list.Pages {
		entry, err := c.resolvePage(source, page.Name, previous[page.Name])
		if err != nil {
			return nil, util.Errorf("%w", err)
		}
		pages[page.Name] = entry
	}

	c.mu.Lock()
	c.sources[key] = sourceEntry{ListMD5: source.GetMeta().MD5Hash, Pages: pages}
	c.touched[key] = struct{}{}
	c.mu.Unlock()
	return distinctHashes(pages), nil
}

func (c *Cache) resolvePage(source *bookpkg.Source, name string, previous pageEntry) (pageEntry, error) {
	stat, err := source.AssetStat(name)
	if err != nil {
		return pageEntry{}, util.Errorf("%w", err)
	}

	c.mu.Lock()
	c.stats.Pages++
	if previous.matches(stat) {
		c.stats.StatHits++
		c.mu.Unlock()
		return previous, nil
	}
	c.mu.Unlock()

	readAt := time.Now()
	asset, err := source.OpenAsset(name)
	if err != nil {
		return pageEntry{}, util.Errorf("%w", err)
	}
	md5Hash, hashErr := hashutil.MD5Hash(asset.File)
	closeErr := asset.File.Close()
	if hashErr != nil {
		return pageEntry{}, util.Errorf("%w", hashErr)
	}
	if closeErr != nil {
		return pageEntry{}, util.Errorf("%w", closeErr)
	}

	c.mu.Lock()
	c.stats.Hashed++
	c.mu.Unlock()

	entry := pageEntry{Size: stat.Size, MD5: md5Hash}
	if fsutil.Settled(stat.ModTime, readAt) {
		entry.ModTime = stat.ModTime
	}
	return entry, nil
}

func (c *Cache) Save() error {
	if err := c.store.EnsureWritable(); err != nil {
		return util.Errorf("%w", err)
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	sources := maps.Clone(c.sources)
	stored, raw, readErr := c.readFile()
	if readErr == nil {
		sources = stored.Sources
		for key := range c.touched {
			if entry, ok := c.sources[key]; ok {
				sources[key] = entry
			} else {
				delete(sources, key)
			}
		}
	} else {
		raw = nil
	}

	if c.liveBooks != nil {
		if live, ok := c.liveBooks(); ok {
			pruneSources(sources, live)
		}
	}

	data, err := json.Marshal(cacheFile{
		SchemaVersion: schemaVersion,
		Generator:     "plainshelf/" + version.Version,
		Hash:          hashName,
		Sources:       sources,
	}, jsonopt.DiskCompact())
	if err != nil {
		return util.Errorf("%w", err)
	}

	if !bytes.Equal(data, raw) {
		if err := c.store.WriteFileAtomic(cacheFileName, data); err != nil {
			return util.Errorf("%w", err)
		}
		c.logger.Debug("wrote the page hash cache", "file", cacheFileName, "sources", len(sources))
	}

	c.sources = sources
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
	if stored.SchemaVersion != schemaVersion || stored.Hash != hashName {
		return nil, nil, util.Errorf("page hash cache is schema_version %d with hash %q, this build reads %d with %q",
			stored.SchemaVersion, stored.Hash, schemaVersion, hashName)
	}
	if stored.Sources == nil {
		stored.Sources = map[string]sourceEntry{}
	}
	return &stored, raw, nil
}

func pruneSources(sources map[string]sourceEntry, live map[string]struct{}) {
	for key := range sources {
		bookID, _, _ := strings.Cut(key, "/")
		if _, alive := live[bookID]; !alive {
			delete(sources, key)
		}
	}
}

func distinctHashes(pages map[string]pageEntry) []string {
	hashes := make([]string, 0, len(pages))
	for _, page := range pages {
		hashes = append(hashes, page.MD5)
	}
	slices.Sort(hashes)
	return slices.Compact(hashes)
}

func sourceKey(bookID, sourceID string) string {
	return bookID + "/" + sourceID
}
