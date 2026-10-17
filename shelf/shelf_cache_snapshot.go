package shelf

import (
	"cmp"
	"errors"
	"os"
	"path"
	"slices"
	"strings"
	"time"

	"github.com/voilelab/plainshelf/internal/fsutil"
	"github.com/voilelab/plainshelf/internal/util"
	"github.com/voilelab/plainshelf/shelf/bookpkg"
)

/*
A remote shelf (NewShelfFromFS) is listed from the exported book cache instead
of being walked. Over pCloud or WebDAV every Stat and Open is a request, and a
walk costs several per book; the export is the whole listing in one file.

That makes the export the shelf's freshness, too: a remote shelf re-reads it
every scan interval and never checks books one by one, so a change shows up
once the writing installation exports again (book_cache_interval). A manual
rescan still walks the shelf for an answer that cannot wait.

Without a usable export the shelf is walked once when it opens, as a local one
would be, and afterwards only looks for an export to appear. A shelf with an
offline store that already holds a snapshot opens from it instead: a source no
installation exports is otherwise walked again at every start.
*/

// refreshRemoteBookCache brings a remote shelf's listing up to date from the
// newest usable exported book cache. With none, it runs withoutExport, or
// keeps what it has when that is nil.
//
// While the source is unavailable, a shelf with an offline store keeps its
// listing, or takes the stored snapshot when it has none yet.
func (s *Shelf) refreshRemoteBookCache(withoutExport func() error) error {
	err := s.refreshRemoteFromSource(withoutExport)
	if err == nil {
		s.afterRemoteListing()
		return nil
	}
	if errors.Is(err, fsutil.ErrSourceUnavailable) && s.listFromOfflineSnapshot() {
		s.Warn("shelf source is unavailable; listing from the offline snapshot", "error", err)
		return nil
	}
	return err
}

// listFromOfflineSnapshot reports whether the shelf has a listing to answer
// with, taking the stored snapshot when it has none yet.
func (s *Shelf) listFromOfflineSnapshot() bool {
	if s.offline == nil {
		return false
	}
	s.markRemoteRefreshAttempt()

	s.bookCache.RLock()
	listed := !s.bookCache.lastScanStart.IsZero()
	s.bookCache.RUnlock()
	if listed {
		return true
	}

	snapshot := s.offline.lastSnapshot()
	if snapshot == nil {
		return false
	}
	entries, folders, err := s.bookCacheSnapshot(snapshot)
	if err != nil {
		s.Warn("ignoring an unusable offline snapshot", "error", err)
		return false
	}

	s.bookCache.Lock()
	s.bookCache.cache = entries
	s.bookCache.folders = folders
	s.bookCache.treeDirty = false
	s.bookCache.lastScanStart = time.Unix(snapshot.Timestamp, 0)
	// Not the source's digest: the next export it offers is applied even when
	// its walk began in the same second.
	s.bookCache.remoteDigest = ""
	s.bookCache.Unlock()

	s.reconcileDownloads()
	return true
}

// openWithoutExport is what an opening remote shelf does without a usable
// export: list from the offline snapshot when there is one, walk otherwise.
func (s *Shelf) openWithoutExport() error {
	if s.listFromOfflineSnapshot() {
		s.Debug("listed the shelf from the offline snapshot; a rescan walks the source")
		return nil
	}
	return s.scanToBookCache()
}

func (s *Shelf) refreshRemoteFromSource(withoutExport func() error) error {
	cache, name, err := s.newestExportedBookCache()
	if err != nil {
		s.markRemoteRefreshAttempt()
		return err
	}
	if cache == nil {
		s.markRemoteRefreshAttempt()
		if withoutExport != nil {
			return withoutExport()
		}
		return nil
	}

	digest, err := bookCacheDigest(cache.Folders, cache.Books)
	if err != nil {
		return util.Errorf("%w", err)
	}

	takenAt := time.Unix(cache.Timestamp, 0)
	s.bookCache.RLock()
	current := s.bookCache.lastScanStart
	applied := s.bookCache.remoteDigest
	s.bookCache.RUnlock()

	// Already as new: older than the current listing, or the same content. The
	// timestamp has whole-second precision, so a re-export within the same
	// second is told apart by its content.
	if !current.IsZero() && (takenAt.Before(current) || (takenAt.Equal(current) && digest == applied)) {
		s.markRemoteRefreshAttempt()
		return nil
	}

	entries, folders, err := s.bookCacheSnapshot(cache)
	if err != nil {
		// Strict on purpose: building the listing from a half-valid file would
		// show a silently truncated library instead of the cache miss it is.
		s.Warn("ignoring an unusable exported book cache", "file", name, "error", err)
		s.markRemoteRefreshAttempt()
		if withoutExport != nil {
			return withoutExport()
		}
		return nil
	}

	s.bookCache.Lock()
	s.bookCache.cache = entries
	s.bookCache.folders = folders
	s.bookCache.treeDirty = false
	s.bookCache.lastFullScan = time.Now()
	s.bookCache.lastScanStart = takenAt
	s.bookCache.remoteDigest = digest
	s.bookCache.Unlock()

	s.Debug("listed the shelf from an exported book cache", "file", name, "books", len(entries), "folders", len(folders))
	return nil
}

// markRemoteRefreshAttempt restarts the scan interval, so a shelf with no new
// export asks again one interval later rather than on every read.
func (s *Shelf) markRemoteRefreshAttempt() {
	s.bookCache.Lock()
	s.bookCache.lastFullScan = time.Now()
	s.bookCache.Unlock()
}

// newestExportedBookCache reads every book-cache-*.json in app/ and returns the
// usable one whose walk began last, ties broken by name so the choice does not
// depend on listing order.
//
// The pCloud client picks by file modification time instead, which is enough
// there because it still lists the tree and only takes book.json contents from
// the cache. Here the cache is the whole listing, so the pick has to be the most
// recent walk, which only the content records. The candidates number the
// installations writing the shelf, not its books.
//
// The error is set only when the source is unavailable.
func (s *Shelf) newestExportedBookCache() (*BookCacheFile, string, error) {
	entries, err := s.dbRoot.ReadDir(appFolder)
	if err != nil {
		if errors.Is(err, fsutil.ErrSourceUnavailable) {
			return nil, "", util.Errorf("%w", err)
		}
		if !errors.Is(err, os.ErrNotExist) {
			s.Warn("failed to list the app folder for an exported book cache", "error", err)
		}
		return nil, "", nil
	}

	var best *BookCacheFile
	var bestName string
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasPrefix(name, bookCacheFilePrefix) || !strings.HasSuffix(name, bookCacheFileSuffix) {
			continue
		}

		cache, err := s.readBookCacheFile(path.Join(appFolder, name))
		if err != nil {
			s.Debug("skipping an unreadable book cache file", "file", name, "error", err)
			continue
		}

		if best == nil || cmp.Or(cmp.Compare(cache.Timestamp, best.Timestamp), cmp.Compare(name, bestName)) > 0 {
			best, bestName = cache, name
		}
	}
	return best, bestName, nil
}

// bookCacheSnapshot turns an exported cache into book cache entries without
// reading any book package. The file comes from another installation and maybe
// another machine, so every path is checked to name a book package under books/
// before a read is ever addressed to it.
func (s *Shelf) bookCacheSnapshot(cache *BookCacheFile) (map[string]*bookIDCacheEntry, []FolderPath, error) {
	entries := make(map[string]*bookIDCacheEntry, len(cache.Books))
	for bookID, entry := range cache.Books {
		if entry.Meta == nil || entry.Meta.ID != bookID {
			return nil, nil, util.Errorf("entry %q does not carry its own book ID", bookID)
		}
		if !isBookPackagePath(entry.Path) {
			return nil, nil, util.Errorf("entry %q has an invalid path %q", bookID, entry.Path)
		}

		// Not built with newBookIDCacheEntry: that reads the current source's
		// character count, a request per book. A remote listing has none to show.
		entries[bookID] = &bookIDCacheEntry{
			folders:   bookFolderPath(entry.Path),
			path:      entry.Path,
			book:      bookpkg.FromSnapshot(s.dbRoot, s.Logger, entry.Path, entry.Meta),
			sourceMD5: entry.CurrentSourceMD5,
		}
	}

	folders := make([]FolderPath, 0, len(cache.Folders))
	for _, name := range cache.Folders {
		if name == "/" {
			folders = append(folders, FolderPath{})
			continue
		}
		if !isRelativeShelfPath(name) {
			return nil, nil, util.Errorf("invalid folder %q", name)
		}
		folders = append(folders, strings.Split(name, "/"))
	}
	sortFolders(folders)

	return entries, folders, nil
}

// isBookPackagePath reports whether p is a clean path to a book package under
// books/, as the exporter writes it.
func isBookPackagePath(p string) bool {
	rest, ok := strings.CutPrefix(p, booksFolder+"/")
	return ok && isRelativeShelfPath(rest) && strings.HasSuffix(p, bookExtension)
}

// isRelativeShelfPath reports whether p is a clean, relative, slash-separated
// path with no "." or ".." segment.
func isRelativeShelfPath(p string) bool {
	if p == "" || path.Clean(p) != p || path.IsAbs(p) {
		return false
	}
	return !slices.ContainsFunc(strings.Split(p, "/"), func(segment string) bool {
		return segment == "." || segment == ".."
	})
}
