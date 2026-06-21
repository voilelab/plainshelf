package shelf

import (
	"maps"
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

	treeDirty    bool
	lastFullScan time.Time

	scanInterval      time.Duration
	lastBookCheck     time.Time
	bookCheckInterval time.Duration
	refreshing        bool
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
	s.bookCache.RUnlock()

	if !force && !treeDirty && time.Since(lastFullScan) < s.bookCache.scanInterval {
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

	err := s.iterateBooks(nil, func(b *Book) bool {
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

	s.bookCache.Lock()
	s.bookCache.cache = cache
	s.bookCache.treeDirty = false
	s.bookCache.lastFullScan = time.Now()
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
	// s.rlock() (shared flock), which prevents mutations from modifying the
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
	s.bookCache.RLock()
	treeDirty := s.bookCache.treeDirty
	lastFullScan := s.bookCache.lastFullScan
	lastBookCheck := s.bookCache.lastBookCheck
	refreshing := s.bookCache.refreshing
	s.bookCache.RUnlock()

	// Full scan: tree is structurally dirty or scan interval elapsed. Keep synchronous so
	// mutations are immediately visible to callers.
	if treeDirty || time.Since(lastFullScan) >= s.bookCache.scanInterval {
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

		if err := s.rlock(); err != nil {
			s.Warn("background book check skipped: failed to acquire lock", "error", err)
			return
		}
		defer s.unlock()

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
