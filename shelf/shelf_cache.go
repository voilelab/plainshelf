package shelf

import (
	"maps"
	"slices"
	"sort"
	"sync"
	"time"

	"github.com/voilelab/plainshelf/internal/util"
)

type bookIDCacheEntry struct {
	layers Layers
	path   string
	book   *Book
}

type bookCache struct {
	sync.RWMutex
	cache map[string]*bookIDCacheEntry

	// layers is every layer directory the shelf holds, sorted, with the root as
	// an empty Layers. Kept beside the book cache because both come out of the
	// same walk, and kept as its own list because an empty layer holds no book
	// and could not be rebuilt from cache above — the same reason
	// BookCacheFile.Layers exists.
	layers []Layers

	treeDirty    bool
	lastFullScan time.Time

	scanInterval      time.Duration
	lastBookCheck     time.Time
	bookCheckInterval time.Duration
	refreshing        bool

	// lastScanStart is when the walk behind the current cache began. See
	// scanToBookCache for why the start and not the end.
	lastScanStart time.Time

	// Exported cache state; see shelf_cache_export.go. There is deliberately no
	// "export is dirty" flag: books are edited in place through *Book, which the
	// cache holds a pointer to, so a flag would have to be set from every
	// mutating method and any one that was missed would silently stop exporting.
	// lastExportDigest fingerprints what was written instead, which cannot be
	// forgotten because it is computed from the content itself.
	lastExport       time.Time
	lastExportDigest string
	exporting        bool
}

func newBookCache(scanInterval, bookCheckInterval time.Duration) *bookCache {
	return &bookCache{
		cache: make(map[string]*bookIDCacheEntry),

		scanInterval:      scanInterval,
		bookCheckInterval: bookCheckInterval,
	}
}

func (s *Shelf) markBookCacheTreeDirty() {
	s.bookCache.Lock()
	s.bookCache.treeDirty = true
	s.bookCache.Unlock()
}

func (s *Shelf) refreshBookCacheIfNeeded(force bool) error {
	s.bookCache.RLock()
	treeDirty := s.bookCache.treeDirty
	lastFullScan := s.bookCache.lastFullScan
	scanInterval := s.bookCache.scanInterval
	s.bookCache.RUnlock()

	if !force && !treeDirty && time.Since(lastFullScan) < scanInterval {
		s.onlyRefreshBooksInCache()
		return nil
	}

	err := s.scanToBookCache()
	if err != nil {
		return util.Errorf("%w", err)
	}
	return nil
}

// scanToBookCache scans the book folders and updates the book cache with the current state of the books.
func (s *Shelf) scanToBookCache() error {
	cache := make(map[string]*bookIDCacheEntry)

	// Recorded before the walk, not after. A book read early in the walk can be
	// edited before the walk ends, so only the start time is a moment the whole
	// result is known to be current as of. The exported cache publishes this as
	// its Timestamp and readers use it to decide what they must re-read.
	scanStart := time.Now()

	var layers []Layers

	err := s.iterateShelfTree(func(ls Layers) bool {
		layers = append(layers, ls)
		return true
	}, func(b *Book) bool {
		cache[b.ID()] = &bookIDCacheEntry{
			layers: b.Layers(),
			path:   b.FolderPath(),
			book:   b,
		}
		return true
	})
	if err != nil {
		return util.Errorf("%w", err)
	}

	sortLayers(layers)

	s.bookCache.Lock()
	s.bookCache.cache = cache
	s.bookCache.layers = layers
	s.bookCache.treeDirty = false
	s.bookCache.lastFullScan = time.Now()
	s.bookCache.lastScanStart = scanStart
	s.bookCache.Unlock()

	return nil
}

func (s *Shelf) onlyRefreshBooksInCache() {
	// Snapshot the current entries under a brief read lock so that concurrent
	// list operations are not blocked during the filesystem I/O below.
	s.bookCache.RLock()
	snapshot := maps.Clone(s.bookCache.cache)
	s.bookCache.RUnlock()

	// Perform all stat and open calls outside any cache lock. The caller holds
	// s.shelfLock.RLock() (shared flock), which prevents mutations from modifying the
	// cache concurrently, so the snapshot remains consistent with the filesystem.
	//
	// updated: bookID → refreshed entry (nil = delete from cache)
	updated := make(map[string]*bookIDCacheEntry)

	for bookID, cacheEntry := range snapshot {
		if !cacheEntry.book.IsStale() {
			continue
		}

		book, err := openBook(s.dbRoot, s.Logger, cacheEntry.path)
		if err != nil {
			s.Warn("Failed to refresh book cache entry, skipping", "bookID", cacheEntry.book.ID(), "error", err)
			updated[bookID] = nil
			continue
		}

		if book.ID() != cacheEntry.book.ID() {
			s.Warn("Book ID mismatch when refreshing book cache entry, skipping", "expectedBookID", cacheEntry.book.ID(), "actualBookID", book.ID())
			updated[bookID] = nil
			continue
		}

		book.setLayers(cacheEntry.layers)
		updated[bookID] = &bookIDCacheEntry{
			layers: cacheEntry.layers,
			path:   cacheEntry.path,
			book:   book,
		}
	}

	// Apply only the changed entries under a brief write lock.
	s.bookCache.Lock()
	for bookID, entry := range updated {
		if entry != nil {
			s.bookCache.cache[bookID] = entry
		} else {
			delete(s.bookCache.cache, bookID)
		}
	}
	s.bookCache.lastBookCheck = time.Now()
	s.bookCache.Unlock()
}

// scheduleBookCacheRefreshIfNeeded triggers a refresh based on the current cache state.
//
// Full scans (when the tree is dirty or the scan interval has elapsed) run synchronously so
// that callers immediately see structural changes such as moved or renamed layers.
//
// Per-book staleness checks (within the scan interval) run in a background goroutine so
// that list operations are never blocked by N filesystem stat calls — the main performance
// concern on SMB mounts. The check is rate-limited by bookCheckInterval.
func (s *Shelf) scheduleBookCacheRefreshIfNeeded() {
	// Deferred so it runs on every return path below: whichever tier of refresh
	// this call decides on, the exported cache is offered the result.
	defer s.scheduleBookCacheExportIfNeeded()

	s.bookCache.RLock()
	treeDirty := s.bookCache.treeDirty
	lastFullScan := s.bookCache.lastFullScan
	lastBookCheck := s.bookCache.lastBookCheck
	refreshing := s.bookCache.refreshing
	scanInterval := s.bookCache.scanInterval
	s.bookCache.RUnlock()

	// Full scan: tree is structurally dirty or scan interval elapsed. Keep synchronous so
	// mutations are immediately visible to callers.
	if treeDirty || time.Since(lastFullScan) >= scanInterval {
		_ = s.refreshBookCacheIfNeeded(false)
		return
	}

	// Per-book check: async so callers are not blocked on N stat calls.
	if refreshing || time.Since(lastBookCheck) < s.bookCache.bookCheckInterval {
		return
	}

	s.bookCache.Lock()
	if s.bookCache.refreshing {
		s.bookCache.Unlock()
		return
	}
	s.bookCache.refreshing = true
	s.bookCache.Unlock()

	go func() {
		defer func() {
			s.bookCache.Lock()
			s.bookCache.refreshing = false
			s.bookCache.Unlock()
		}()

		if err := s.shelfLock.RLock(); err != nil {
			s.Warn("background book check skipped: failed to acquire lock", "error", err)
			return
		}
		defer s.shelfLock.Unlock()

		s.onlyRefreshBooksInCache()
	}()
}

func (s *Shelf) listBooksFromCache() []*Book {
	s.bookCache.RLock()
	defer s.bookCache.RUnlock()

	var books []*Book
	for _, cacheEntry := range s.bookCache.cache {
		books = append(books, cacheEntry.book)
	}

	sort.Slice(books, func(i, j int) bool {
		return books[i].ID() < books[j].ID()
	})

	return books
}

// listLayersFromCache returns the cached layer list. The copy is deliberate:
// the caller marshals it into a response while the next scan may already be
// replacing the cached slice.
func (s *Shelf) listLayersFromCache() []Layers {
	s.bookCache.RLock()
	defer s.bookCache.RUnlock()

	layers := make([]Layers, len(s.bookCache.layers))
	for i, layer := range s.bookCache.layers {
		layers[i] = slices.Clone(layer)
	}
	return layers
}

// addLayersToBookCache records a layer and every ancestor it needed, so that a
// directory this process just created shows up in the next listing instead of
// waiting for the scan interval. Every place that MkdirAll's a layer path calls
// it; a layer created outside PlainShelf is still found by the next scan.
func (s *Shelf) addLayersToBookCache(layer Layers) {
	s.bookCache.Lock()
	defer s.bookCache.Unlock()

	added := false
	for i := 0; i <= len(layer); i++ {
		prefix := layer[:i]
		if slices.ContainsFunc(s.bookCache.layers, prefix.Equal) {
			continue
		}
		s.bookCache.layers = append(s.bookCache.layers, slices.Clone(prefix))
		added = true
	}

	if added {
		sortLayers(s.bookCache.layers)
	}
}

// removeLayerFromBookCache drops one layer entry. Only for a layer known to
// have no children; a subtree change marks the tree dirty instead.
func (s *Shelf) removeLayerFromBookCache(layer Layers) {
	s.bookCache.Lock()
	defer s.bookCache.Unlock()

	s.bookCache.layers = slices.DeleteFunc(s.bookCache.layers, layer.Equal)
}

func sortLayers(layers []Layers) {
	sort.Slice(layers, func(i, j int) bool {
		return layers[i].String() < layers[j].String()
	})
}

func (s *Shelf) getUpdatedBookFromBookID(bookID string) (*Book, error) {
	s.bookCache.Lock()

	cacheEntry := s.bookCache.cache[bookID]
	if cacheEntry != nil {
		if !cacheEntry.book.IsStale() {
			s.bookCache.Unlock()
			return cacheEntry.book, nil
		}

		// If the cache entry is stale or doesn't exist, we need to refresh it.
		delete(s.bookCache.cache, bookID)

		book, err := openBook(s.dbRoot, s.Logger, cacheEntry.path)
		if err == nil {
			book.setLayers(cacheEntry.layers)
			s.bookCache.cache[bookID] = &bookIDCacheEntry{
				layers: cacheEntry.layers,
				path:   cacheEntry.path,
				book:   book,
			}
			s.bookCache.Unlock()
			return book, nil
		} else {
			s.Warn("Failed to refresh book cache entry, will attempt to refresh entire book cache", "bookID", bookID, "error", err)
		}
	}

	s.bookCache.Unlock()

	// If we reach here, it means the cache entry is either missing or stale and we failed to refresh it.
	// We should refresh the entire book cache to ensure we have the most up-to-date information.

	if err := s.refreshBookCacheIfNeeded(false); err != nil {
		return nil, util.Errorf("%w", err)
	}

	s.bookCache.RLock()
	defer s.bookCache.RUnlock()
	bookCacheEntry := s.bookCache.cache[bookID]
	if bookCacheEntry != nil {
		return bookCacheEntry.book, nil
	}

	return nil, util.Errorf("%w", ErrBookNotFound)
}

func (s *Shelf) updateBookCacheEntry(layers Layers, path string, book *Book) {
	s.bookCache.Lock()
	defer s.bookCache.Unlock()

	s.bookCache.cache[book.ID()] = &bookIDCacheEntry{
		layers: layers,
		path:   path,
		book:   book,
	}
}

func (s *Shelf) deleteBookCacheEntry(bookID string) {
	s.bookCache.Lock()
	defer s.bookCache.Unlock()

	delete(s.bookCache.cache, bookID)
}
