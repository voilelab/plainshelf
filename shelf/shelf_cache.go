package shelf

import (
	"maps"
	"sort"
	"sync"
	"time"

	"github.com/voilelab/plainshelf/internal/util"
)

const minBookCacheFullScanInterval = time.Minute

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
}

func newBookCache() *bookCache {
	return &bookCache{
		cache: make(map[string]*bookIDCacheEntry),
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

	if !force && !treeDirty && time.Since(lastFullScan) < minBookCacheFullScanInterval {
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
	s.bookCache.Lock()
	cache := maps.Clone(s.bookCache.cache)
	s.bookCache.Unlock()

	staleIDs := []string{}
	for bookID, cacheEntry := range cache {
		if cacheEntry.book.IsStale() {
			staleIDs = append(staleIDs, bookID)
		}
	}

	for _, staleID := range staleIDs {
		cacheEntry := cache[staleID]

		delete(cache, staleID)

		book, err := openBook(s.dbRoot, s.Logger, cacheEntry.path)
		if err != nil {
			s.Warn("Failed to refresh book cache entry, skipping", "bookID", cacheEntry.book.ID(), "error", err)
			continue
		}

		if book.ID() != cacheEntry.book.ID() {
			s.Warn("Book ID mismatch when refreshing book cache entry, skipping", "expectedBookID", cacheEntry.book.ID(), "actualBookID", book.ID())
			continue
		}

		book.setLayers(cacheEntry.layers)

		cache[book.ID()] = &bookIDCacheEntry{
			layers: cacheEntry.layers,
			path:   cacheEntry.path,
			book:   book,
		}
	}

	s.bookCache.Lock()
	s.bookCache.cache = cache
	s.bookCache.treeDirty = false
	s.bookCache.Unlock()
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
	defer s.bookCache.Unlock()

	cacheEntry := s.bookCache.cache[bookID]
	if cacheEntry != nil {
		if !cacheEntry.book.IsStale() {
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

			return book, nil
		} else {
			s.Warn("Failed to refresh book cache entry, will attempt to refresh entire book cache", "bookID", bookID, "error", err)
		}
	}

	// If we reach here, it means the cache entry is either missing or stale and we failed to refresh it.
	// We should refresh the entire book cache to ensure we have the most up-to-date information.

	// FIXME: This is a bad implementation, because malusers can cause DoS by repeatedly requesting non-existent or stale book IDs,
	// which will cause the entire book cache to be refreshed on each request.
	// We should implement a better caching strategy in the future to avoid this issue.

	s.Warn("Book ID not found in cache or cache entry is stale, refreshing entire book cache", "bookID", bookID)

	s.bookCache.cache = make(map[string]*bookIDCacheEntry)
	err := s.iterateBooks(nil, func(b *Book) bool {
		s.bookCache.cache[b.ID()] = &bookIDCacheEntry{
			layers: b.Layers(),
			path:   b.FolderPath(),
			book:   b,
		}
		return true
	})
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

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
