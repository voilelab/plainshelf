package shelf

import (
	"cmp"
	"maps"
	"slices"
	"sync"
	"time"

	"github.com/voilelab/plainshelf/internal/util"
	"github.com/voilelab/plainshelf/shelf/bookpkg"
)

type bookIDCacheEntry struct {
	folders FolderPath
	path    string
	book    *Book

	charCount int

	sourceMD5 string

	charCountAt time.Time
}

func readBookIDCacheEntry(folders FolderPath, path string, book *Book) *bookIDCacheEntry {
	charCount, sourceMD5, charCountAt := readSourceStat(book)

	return &bookIDCacheEntry{
		folders:     folders,
		path:        path,
		book:        book,
		charCount:   charCount,
		sourceMD5:   sourceMD5,
		charCountAt: charCountAt,
	}
}

func readSourceStat(book *Book) (int, string, time.Time) {
	readAt := time.Now()
	charCount, md5 := book.CurrentSourceStat()
	return charCount, md5, readAt
}

func keepNewerCharCount(entry, published *bookIDCacheEntry) *bookIDCacheEntry {
	if published == nil || !published.charCountAt.After(entry.charCountAt) {
		return entry
	}

	merged := *entry
	merged.charCount = published.charCount
	merged.sourceMD5 = published.sourceMD5
	merged.charCountAt = published.charCountAt
	return &merged
}

type bookCache struct {
	sync.RWMutex
	cache map[string]*bookIDCacheEntry

	folders []FolderPath

	folderMeta map[string]FolderMeta

	treeDirty    bool
	lastFullScan time.Time

	scanInterval      time.Duration
	lastBookCheck     time.Time
	bookCheckInterval time.Duration
	refreshing        bool

	rescanID string

	rescanTokens   float64
	rescanTokensAt time.Time

	lastScanStart time.Time

	lastExport       time.Time
	lastExportDigest string
	exporting        bool

	remoteDigest string
}

func newBookCache(scanInterval, bookCheckInterval time.Duration) *bookCache {
	return &bookCache{
		cache: make(map[string]*bookIDCacheEntry),

		rescanTokens: rescanBurst,

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

	if s.remote {
		if !force && time.Since(lastFullScan) < scanInterval {
			return nil
		}
		var withoutExport func() error
		if force {
			withoutExport = s.scanToBookCache
		}
		return s.refreshRemoteBookCache(withoutExport)
	}

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

	scanStart := time.Now()

	var folders, metaFolders []FolderPath

	stats, err := s.iterateShelfTree(func(ls FolderPath) bool {
		folders = append(folders, ls)
		return true
	}, func(folder FolderPath) {
		metaFolders = append(metaFolders, folder)
	}, func(b *Book) bool {
		entry := readBookIDCacheEntry(bookFolderPath(b.PackagePath()), b.PackagePath(), b)
		if s.offline != nil {
			entry.book = bookpkg.FromSnapshot(s.dbRoot, s.Logger, b.PackagePath(), b.GetMeta())
		}
		cache[b.ID()] = entry
		return true
	})
	if err != nil {
		return util.Errorf("%w", err)
	}

	sortFolders(folders)

	folderMeta, err := s.readFolderMetaForScan(s.walkRoot(), metaFolders)
	if err != nil {
		return util.Errorf("%w", err)
	}

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
	s.bookCache.folderMeta = folderMeta
	s.bookCache.treeDirty = false
	s.bookCache.lastFullScan = time.Now()
	s.bookCache.lastScanStart = scanStart
	s.bookCache.Unlock()

	if s.remote {
		s.afterRemoteListing()
	}
	return nil
}

func (s *Shelf) onlyRefreshBooksInCache() {
	s.bookCache.RLock()
	snapshot := maps.Clone(s.bookCache.cache)
	s.bookCache.RUnlock()

	refreshed := make(map[string]*bookIDCacheEntry)
	var removed []string

	for bookID, cacheEntry := range snapshot {
		if !cacheEntry.book.IsStale() {
			continue
		}

		book, err := bookpkg.Open(s.dbRoot, s.Logger, cacheEntry.path)
		if err != nil {
			s.Warn("Failed to refresh book cache entry, skipping", "bookID", cacheEntry.book.ID(), "error", err)
			removed = append(removed, bookID)
			continue
		}

		if book.ID() != cacheEntry.book.ID() {
			s.Warn("Book ID mismatch when refreshing book cache entry, skipping", "expectedBookID", cacheEntry.book.ID(), "actualBookID", book.ID())
			removed = append(removed, bookID)
			continue
		}

		refreshed[bookID] = readBookIDCacheEntry(cacheEntry.folders, cacheEntry.path, book)
	}

	s.bookCache.Lock()
	for bookID, entry := range refreshed {
		s.bookCache.cache[bookID] = keepNewerCharCount(entry, s.bookCache.cache[bookID])
	}
	for _, bookID := range removed {
		delete(s.bookCache.cache, bookID)
	}
	s.bookCache.lastBookCheck = time.Now()
	s.bookCache.Unlock()
}

func (s *Shelf) scheduleBookCacheRefreshIfNeeded() {
	defer s.scheduleBookCacheExportIfNeeded()

	s.bookCache.RLock()
	treeDirty := s.bookCache.treeDirty
	lastFullScan := s.bookCache.lastFullScan
	lastBookCheck := s.bookCache.lastBookCheck
	refreshing := s.bookCache.refreshing
	scanInterval := s.bookCache.scanInterval
	bookCheckInterval := s.bookCache.bookCheckInterval
	s.bookCache.RUnlock()

	if s.remote {
		if time.Since(lastFullScan) >= scanInterval {
			_ = s.refreshBookCacheIfNeeded(false)
		}
		return
	}

	if treeDirty || time.Since(lastFullScan) >= scanInterval {
		_ = s.refreshBookCacheIfNeeded(false)
		return
	}

	if refreshing || time.Since(lastBookCheck) < bookCheckInterval {
		return
	}

	s.startBookCacheWork(&s.bookCache.refreshing, func() {
		if err := s.shelfLock.RLock(); err != nil {
			s.Warn("background book check skipped: failed to acquire lock", "error", err)
			return
		}
		defer s.shelfLock.Unlock()

		s.onlyRefreshBooksInCache()
	})
}

func (s *Shelf) startBookCacheWork(inProgress *bool, work func()) {
	s.bookCache.Lock()
	if *inProgress {
		s.bookCache.Unlock()
		return
	}
	*inProgress = true
	s.bookCache.Unlock()

	done := func() {
		s.bookCache.Lock()
		*inProgress = false
		s.bookCache.Unlock()
	}
	if !s.goBackground(func() {
		defer done()
		work()
	}) {
		done()
	}
}

func (s *Shelf) listBookListingsFromCache() []BookListing {
	s.bookCache.RLock()
	defer s.bookCache.RUnlock()

	var listings []BookListing
	for _, cacheEntry := range s.bookCache.cache {
		listings = append(listings, BookListing{
			Book:      cacheEntry.book,
			Folders:   slices.Clone(cacheEntry.folders),
			CharCount: cacheEntry.charCount,
		})
	}

	slices.SortFunc(listings, func(a, b BookListing) int {
		return cmp.Compare(a.Book.ID(), b.Book.ID())
	})

	return listings
}

func (s *Shelf) listFoldersFromCache() []FolderPath {
	s.bookCache.RLock()
	defer s.bookCache.RUnlock()

	folders := make([]FolderPath, len(s.bookCache.folders))
	for i, folder := range s.bookCache.folders {
		folders[i] = slices.Clone(folder)
	}
	sortFolderSiblings(folders, s.bookCache.folderMeta)
	return folders
}

func sortFolderSiblings(folders []FolderPath, metas map[string]FolderMeta) {
	if !slices.ContainsFunc(folders, func(f FolderPath) bool { return metas[f.String()].HasSortKey }) {
		return
	}
	children := map[string][]FolderPath{}
	present := map[string]bool{}
	seen := map[string]bool{}
	var add func(FolderPath)
	add = func(folder FolderPath) {
		if len(folder) == 0 || seen[folder.String()] {
			return
		}
		parent := folder[:len(folder)-1]
		add(parent)
		seen[folder.String()] = true
		children[parent.String()] = append(children[parent.String()], folder)
	}
	for _, folder := range folders {
		present[folder.String()] = true
		add(folder)
	}
	result := make([]FolderPath, 0, len(folders))
	if present[""] {
		result = append(result, FolderPath{})
	}
	var visit func(string)
	visit = func(parent string) {
		siblings := children[parent]
		slices.SortStableFunc(siblings, func(a, b FolderPath) int {
			x, y := metas[a.String()], metas[b.String()]
			if x.HasSortKey != y.HasSortKey {
				if x.HasSortKey {
					return -1
				}
				return 1
			}
			if x.HasSortKey {
				return cmp.Compare(x.SortKey, y.SortKey)
			}
			return 0
		})
		for _, folder := range siblings {
			if present[folder.String()] {
				result = append(result, folder)
			}
			visit(folder.String())
		}
	}
	visit("")
	copy(folders, result)
}

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

func (s *Shelf) removeFolderFromBookCache(folder FolderPath) {
	s.bookCache.Lock()
	defer s.bookCache.Unlock()

	s.bookCache.folders = slices.DeleteFunc(s.bookCache.folders, folder.Equal)
	delete(s.bookCache.folderMeta, folder.String())
}

func sortFolders(folders []FolderPath) {
	slices.SortFunc(folders, func(a, b FolderPath) int {
		return cmp.Compare(a.String(), b.String())
	})
}

func (s *Shelf) getUpdatedBookFromBookID(bookID string) (*Book, FolderPath, error) {
	s.bookCache.Lock()

	cacheEntry := s.bookCache.cache[bookID]
	if cacheEntry != nil {
		if !cacheEntry.book.IsStale() {
			book, folders := cacheEntry.book, slices.Clone(cacheEntry.folders)
			s.bookCache.Unlock()
			return book, folders, nil
		}

		delete(s.bookCache.cache, bookID)

		book, err := bookpkg.Open(s.dbRoot, s.Logger, cacheEntry.path)
		if err == nil {
			s.bookCache.cache[bookID] = keepNewerCharCount(
				readBookIDCacheEntry(cacheEntry.folders, cacheEntry.path, book),
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
	entry := readBookIDCacheEntry(folders, path, book)

	s.bookCache.Lock()
	defer s.bookCache.Unlock()

	s.bookCache.cache[book.ID()] = keepNewerCharCount(entry, s.bookCache.cache[book.ID()])
}

func (s *Shelf) RefreshBookCharCount(bookID string) {
	s.bookCache.RLock()
	cacheEntry := s.bookCache.cache[bookID]
	s.bookCache.RUnlock()

	if cacheEntry == nil {
		return
	}

	charCount, sourceMD5, charCountAt := readSourceStat(cacheEntry.book)

	s.bookCache.Lock()
	defer s.bookCache.Unlock()

	published := s.bookCache.cache[bookID]
	if published == nil || published.charCountAt.After(charCountAt) {
		return
	}

	refreshed := *published
	refreshed.charCount = charCount
	refreshed.sourceMD5 = sourceMD5
	refreshed.charCountAt = charCountAt
	s.bookCache.cache[bookID] = &refreshed
}

func (s *Shelf) deleteBookCacheEntry(bookID string) {
	s.bookCache.Lock()
	defer s.bookCache.Unlock()

	delete(s.bookCache.cache, bookID)
}
