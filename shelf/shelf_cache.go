package shelf

import (
	"maps"
	"slices"
	"sort"
	"sync"
	"time"

	"github.com/voilelab/plainshelf/internal/util"
	"github.com/voilelab/plainshelf/shelf/bookpkg"
)

type bookIDCacheEntry struct {
	folders FolderPath
	path   string
	book   *Book

	// charCount is the character count of the book's current source, read when
	// this entry was built. It is kept here because a listing that reports it
	// would otherwise open and decode one source meta.json per book on the
	// request path - the same N-filesystem-operations cost the background
	// refresh below exists to keep off it.
	//
	// A published entry is never modified: every path that changes a stored
	// value replaces the whole entry, so a reader that copied values out under
	// the cache lock cannot observe a half-updated one.
	charCount int

	// charCountAt is when the read behind charCount began, and is what orders
	// two observations of the same book against each other.
	//
	// A character count is written without touching book.json, so a walk that
	// read a source before it was rewritten and the refresh that followed the
	// write are two views of the same book, publishable in either order, and
	// nothing else in the entry tells which is older. Publishing the older would
	// stick, because the book.json stat check cannot see a count change. So a
	// count is only ever replaced by one observed later; see keepNewerCharCount.
	charCountAt time.Time
}

// newBookIDCacheEntry builds the cache entry for one book. It reads the book's
// current source meta.json, so call it before taking the cache lock wherever
// that is possible.
func newBookIDCacheEntry(folders FolderPath, path string, book *Book) *bookIDCacheEntry {
	charCount, charCountAt := readCharCount(book)

	return &bookIDCacheEntry{
		folders:      folders,
		path:        path,
		book:        book,
		charCount:   charCount,
		charCountAt: charCountAt,
	}
}

// readCharCount reads a book's current-source character count and reports the
// moment the read began. The start and not the end: a read that overlaps a
// write may see either state, and the caller that started later is the one
// whose write it followed.
func readCharCount(book *Book) (int, time.Time) {
	readAt := time.Now()
	return book.CurrentSourceCharCount(), readAt
}

// keepNewerCharCount returns entry with the character count of the entry it
// replaces, when that one was observed later. Call it under the cache write
// lock, with the entry currently published for the same book.
//
// This is what stops a full scan from undoing a refresh: the scan reads a book
// early in its walk and publishes the whole map at the end, so a source
// rewritten in between is already refreshed by the time the scan lands.
func keepNewerCharCount(entry, published *bookIDCacheEntry) *bookIDCacheEntry {
	if published == nil || !published.charCountAt.After(entry.charCountAt) {
		return entry
	}

	merged := *entry
	merged.charCount = published.charCount
	merged.charCountAt = published.charCountAt
	return &merged
}

type bookCache struct {
	sync.RWMutex
	cache map[string]*bookIDCacheEntry

	// folders is every folder directory the shelf holds, sorted, with the root as
	// an empty Folders. Kept beside the book cache because both come out of the
	// same walk, and kept as its own list because an empty folder holds no book
	// and could not be rebuilt from cache above — the same reason
	// BookCacheFile.Folders exists.
	folders []FolderPath

	treeDirty    bool
	lastFullScan time.Time

	scanInterval      time.Duration
	lastBookCheck     time.Time
	bookCheckInterval time.Duration
	refreshing        bool

	// rescanID names the manual rescan currently walking the shelf, empty when
	// none is. See shelf_rescan.go for why a second one is refused rather than
	// queued.
	rescanID string

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

func (s *Shelf) scanToBookCache() error {
	cache := make(map[string]*bookIDCacheEntry)

	// Recorded before the walk, not after. A book read early in the walk can be
	// edited before the walk ends, so only the start time is a moment the whole
	// result is known to be current as of. The exported cache publishes this as
	// its Timestamp and readers use it to decide what they must re-read.
	scanStart := time.Now()

	var folders []FolderPath

	stats, err := s.iterateShelfTree(func(ls FolderPath) bool {
		folders = append(folders, ls)
		return true
	}, func(b *Book) bool {
		cache[b.ID()] = newBookIDCacheEntry(bookFolderPath(b.PackagePath()), b.PackagePath(), b)
		return true
	})
	if err != nil {
		return util.Errorf("%w", err)
	}

	sortFolders(folders)

	s.Debug("completed a full shelf scan",
		"books", len(cache), "folders", len(folders),
		"dirs", stats.Dirs, "listed_dirs", stats.ReadDirs, "reused_dirs", stats.ReusedDirs,
		"duration", stats.Duration)

	s.bookCache.Lock()
	for bookID, entry := range cache {
		cache[bookID] = keepNewerCharCount(entry, s.bookCache.cache[bookID])
	}
	s.bookCache.cache = cache
	s.bookCache.folders = folders
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

		book, err := bookpkg.Open(s.dbRoot, s.Logger, cacheEntry.path)
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

		updated[bookID] = newBookIDCacheEntry(cacheEntry.folders, cacheEntry.path, book)
	}

	// Apply only the changed entries under a brief write lock.
	s.bookCache.Lock()
	for bookID, entry := range updated {
		if entry != nil {
			s.bookCache.cache[bookID] = keepNewerCharCount(entry, s.bookCache.cache[bookID])
		} else {
			delete(s.bookCache.cache, bookID)
		}
	}
	s.bookCache.lastBookCheck = time.Now()
	s.bookCache.Unlock()
}

// scheduleBookCacheRefreshIfNeeded triggers a refresh based on the current cache state.
//
// Full scans (tree dirty or scan interval elapsed) run synchronously so callers
// immediately see structural changes such as moved or renamed folders.
//
// Per-book staleness checks (within the scan interval) run in a background
// goroutine so list operations are never blocked by N filesystem stat calls —
// the main performance concern on SMB mounts, rate-limited by bookCheckInterval.
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

	done := func() {
		s.bookCache.Lock()
		s.bookCache.refreshing = false
		s.bookCache.Unlock()
	}

	started := s.goBackground(func() {
		defer done()

		if err := s.shelfLock.RLock(); err != nil {
			s.Warn("background book check skipped: failed to acquire lock", "error", err)
			return
		}
		defer s.shelfLock.Unlock()

		s.onlyRefreshBooksInCache()
	})

	// A closing shelf runs no check, so the flag it was claimed with has to be
	// given back here instead: leaving it set would make a shelf that is opened
	// and closed in a loop look permanently busy to the next read.
	if !started {
		done()
	}
}

func (s *Shelf) listBooksFromCache() []*Book {
	var books []*Book
	for _, listing := range s.listBookListingsFromCache() {
		books = append(books, listing.Book)
	}
	return books
}

// listBookListingsFromCache returns every cached book together with the values
// the cache keeps beside it. The values are copied out under the lock so that
// no caller reads a cache entry after releasing it.
func (s *Shelf) listBookListingsFromCache() []BookListing {
	s.bookCache.RLock()
	defer s.bookCache.RUnlock()

	var listings []BookListing
	for _, cacheEntry := range s.bookCache.cache {
		listings = append(listings, BookListing{
			Book:      cacheEntry.book,
			Folders:    slices.Clone(cacheEntry.folders),
			CharCount: cacheEntry.charCount,
		})
	}

	sort.Slice(listings, func(i, j int) bool {
		return listings[i].Book.ID() < listings[j].Book.ID()
	})

	return listings
}

// listFoldersFromCache returns the cached folder list. The copy is deliberate:
// the caller marshals it into a response while the next scan may already be
// replacing the cached slice.
func (s *Shelf) listFoldersFromCache() []FolderPath {
	s.bookCache.RLock()
	defer s.bookCache.RUnlock()

	folders := make([]FolderPath, len(s.bookCache.folders))
	for i, folder := range s.bookCache.folders {
		folders[i] = slices.Clone(folder)
	}
	return folders
}

// addFolderToBookCache records a folder and every ancestor it needed, so that a
// directory this process just created shows up in the next listing instead of
// waiting for the scan interval. Every place that MkdirAll's a folder path calls
// it; a folder created outside PlainShelf is still found by the next scan.
func (s *Shelf) addFolderToBookCache(folder FolderPath) {
	s.bookCache.Lock()
	defer s.bookCache.Unlock()

	added := false
	for i := 0; i <= len(folder); i++ {
		prefix := folder[:i]
		if slices.ContainsFunc(s.bookCache.folders, prefix.Equal) {
			continue
		}
		s.bookCache.folders = append(s.bookCache.folders, slices.Clone(prefix))
		added = true
	}

	if added {
		sortFolders(s.bookCache.folders)
	}
}

// removeFolderFromBookCache drops one folder entry. Only for a folder known to
// have no children; a subtree change marks the tree dirty instead.
func (s *Shelf) removeFolderFromBookCache(folder FolderPath) {
	s.bookCache.Lock()
	defer s.bookCache.Unlock()

	s.bookCache.folders = slices.DeleteFunc(s.bookCache.folders, folder.Equal)
}

func sortFolders(folders []FolderPath) {
	sort.Slice(folders, func(i, j int) bool {
		return folders[i].String() < folders[j].String()
	})
}

// getUpdatedBookFromBookID resolves a book and the folder it sits in from the
// cache, refreshing a stale entry first. The book and the folder come from the
// same cache entry read under one lock, so a concurrent refresh cannot pair a
// book with a different entry's folder, or report a nested book as having none.
func (s *Shelf) getUpdatedBookFromBookID(bookID string) (*Book, FolderPath, error) {
	s.bookCache.Lock()

	cacheEntry := s.bookCache.cache[bookID]
	if cacheEntry != nil {
		if !cacheEntry.book.IsStale() {
			book, folders := cacheEntry.book, slices.Clone(cacheEntry.folders)
			s.bookCache.Unlock()
			return book, folders, nil
		}

		// If the cache entry is stale or doesn't exist, we need to refresh it.
		delete(s.bookCache.cache, bookID)

		book, err := bookpkg.Open(s.dbRoot, s.Logger, cacheEntry.path)
		if err == nil {
			s.bookCache.cache[bookID] = keepNewerCharCount(
				newBookIDCacheEntry(cacheEntry.folders, cacheEntry.path, book),
				s.bookCache.cache[bookID],
			)
			folders := slices.Clone(cacheEntry.folders)
			s.bookCache.Unlock()
			return book, folders, nil
		} else {
			s.Warn("Failed to refresh book cache entry, will attempt to refresh entire book cache", "bookID", bookID, "error", err)
		}
	}

	s.bookCache.Unlock()

	// If we reach here, it means the cache entry is either missing or stale and we failed to refresh it.
	// We should refresh the entire book cache to ensure we have the most up-to-date information.

	if err := s.refreshBookCacheIfNeeded(false); err != nil {
		return nil, nil, util.Errorf("%w", err)
	}

	s.bookCache.RLock()
	defer s.bookCache.RUnlock()
	bookCacheEntry := s.bookCache.cache[bookID]
	if bookCacheEntry != nil {
		return bookCacheEntry.book, slices.Clone(bookCacheEntry.folders), nil
	}

	return nil, nil, util.Errorf("%w", ErrBookNotFound)
}

func (s *Shelf) updateBookCacheEntry(folders FolderPath, path string, book *Book) {
	// Built before the lock: the entry reads the book's current source from
	// disk, and no listing should wait on that.
	entry := newBookIDCacheEntry(folders, path, book)

	s.bookCache.Lock()
	defer s.bookCache.Unlock()

	s.bookCache.cache[book.ID()] = keepNewerCharCount(entry, s.bookCache.cache[book.ID()])
}

// RefreshBookCharCount re-reads one book's current-source character count into
// its cache entry.
//
// Everything else in an entry is kept current by the book.json stat check in
// Book.IsStale, but a character count lives in the source's own meta.json and
// writing it leaves book.json untouched. That change is invisible to the check,
// so the entry would answer with the previous count until the next full scan.
// Every request path that rewrites a source's content or moves the
// current-source pointer calls this instead.
//
// A book not in the cache is a no-op: the entry built for it later reads the
// count from disk anyway.
func (s *Shelf) RefreshBookCharCount(bookID string) {
	s.bookCache.RLock()
	cacheEntry := s.bookCache.cache[bookID]
	s.bookCache.RUnlock()

	if cacheEntry == nil {
		return
	}

	charCount, charCountAt := readCharCount(cacheEntry.book)

	s.bookCache.Lock()
	defer s.bookCache.Unlock()

	// A scan or a per-book refresh may have replaced the entry while the count
	// was being read, so the count lands on whatever is published now rather
	// than on the entry it was read from - discarding it would leave a count
	// read before this book's source was rewritten in place.
	published := s.bookCache.cache[bookID]
	if published == nil || published.charCountAt.After(charCountAt) {
		return
	}

	refreshed := *published
	refreshed.charCount = charCount
	refreshed.charCountAt = charCountAt
	s.bookCache.cache[bookID] = &refreshed
}

func (s *Shelf) deleteBookCacheEntry(bookID string) {
	s.bookCache.Lock()
	defer s.bookCache.Unlock()

	delete(s.bookCache.cache, bookID)
}
