package shelf

import (
	"cmp"
	"errors"
	"math"
	"os"
	"path"
	"slices"
	"strings"
	"time"

	"github.com/voilelab/plainshelf/internal/fsutil"
	"github.com/voilelab/plainshelf/internal/util"
	"github.com/voilelab/plainshelf/shelf/bookpkg"
)

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
	entries, folders, folderMeta, err := s.bookCacheSnapshot(snapshot)
	if err != nil {
		s.Warn("ignoring an unusable offline snapshot", "error", err)
		return false
	}

	s.bookCache.Lock()
	s.bookCache.cache = entries
	s.bookCache.folders = folders
	s.bookCache.folderMeta = folderMeta
	s.bookCache.treeDirty = false
	s.bookCache.lastScanStart = time.Unix(snapshot.Timestamp, 0)
	s.bookCache.forgetAppliedExportDigestLocked()
	s.bookCache.Unlock()

	s.reconcileDownloads()
	return true
}

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

	digest, err := bookCacheDigest(cache)
	if err != nil {
		return util.Errorf("%w", err)
	}

	takenAt := time.Unix(cache.Timestamp, 0)
	s.bookCache.RLock()
	current := s.bookCache.lastScanStart
	applied := s.bookCache.remoteDigest
	s.bookCache.RUnlock()

	if exportIsNotNewer(takenAt, current, digest, applied) {
		s.markRemoteRefreshAttempt()
		return nil
	}

	entries, folders, folderMeta, err := s.bookCacheSnapshot(cache)
	if err != nil {
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
	s.bookCache.folderMeta = folderMeta
	s.bookCache.treeDirty = false
	s.bookCache.lastFullScan = time.Now()
	s.bookCache.lastScanStart = takenAt
	s.bookCache.remoteDigest = digest
	s.bookCache.Unlock()

	s.Debug("listed the shelf from an exported book cache", "file", name, "books", len(entries), "folders", len(folders))
	return nil
}

func (s *Shelf) markRemoteRefreshAttempt() {
	s.bookCache.Lock()
	s.bookCache.lastFullScan = time.Now()
	s.bookCache.Unlock()
}

func (c *bookCache) forgetAppliedExportDigestLocked() {
	c.remoteDigest = ""
}

func exportIsNotNewer(takenAt, current time.Time, digest, applied string) bool {
	return !current.IsZero() && (takenAt.Before(current) || (takenAt.Equal(current) && digest == applied))
}

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

func (s *Shelf) bookCacheSnapshot(cache *BookCacheFile) (map[string]*bookIDCacheEntry, []FolderPath, map[string]FolderMeta, error) {
	entries := make(map[string]*bookIDCacheEntry, len(cache.Books))
	for bookID, entry := range cache.Books {
		if entry.Meta == nil || entry.Meta.ID != bookID {
			return nil, nil, nil, util.Errorf("entry %q does not carry its own book ID", bookID)
		}
		if !isBookPackagePath(entry.Path) {
			return nil, nil, nil, util.Errorf("entry %q has an invalid path %q", bookID, entry.Path)
		}

		entries[bookID] = s.newRemoteBookIDCacheEntry(entry)
	}

	folders := make([]FolderPath, 0, len(cache.Folders))
	for _, name := range cache.Folders {
		if name == "/" {
			folders = append(folders, FolderPath{})
			continue
		}
		if !isRelativeShelfPath(name) {
			return nil, nil, nil, util.Errorf("invalid folder %q", name)
		}
		folders = append(folders, strings.Split(name, "/"))
	}
	sortFolders(folders)

	folderMeta := make(map[string]FolderMeta, len(cache.FolderMeta))
	for name, meta := range cache.FolderMeta {
		if name == "/" || meta == (BookCacheFolderMeta{}) || !slices.Contains(cache.Folders, name) {
			continue
		}
		folderMeta[name] = folderMetaFromExport(meta)
	}

	return entries, folders, folderMeta, nil
}

func (s *Shelf) newRemoteBookIDCacheEntry(entry BookCacheEntry) *bookIDCacheEntry {
	return &bookIDCacheEntry{
		folders:   bookFolderPath(entry.Path),
		path:      entry.Path,
		book:      bookpkg.FromSnapshot(s.dbRoot, s.Logger, entry.Path, entry.Meta),
		sourceMD5: entry.CurrentSourceMD5,
	}
}

func folderMetaFromExport(meta BookCacheFolderMeta) FolderMeta {
	entry := FolderMeta{Icon: meta.Icon, ReadOnly: meta.ReadOnly}
	if meta.SortKey != nil && !math.IsNaN(*meta.SortKey) && !math.IsInf(*meta.SortKey, 0) {
		entry.SortKey, entry.HasSortKey = *meta.SortKey, true
	}
	return entry
}

func isBookPackagePath(p string) bool {
	rest, ok := strings.CutPrefix(p, booksFolder+"/")
	return ok && isRelativeShelfPath(rest) && strings.HasSuffix(p, bookExtension)
}

func isRelativeShelfPath(p string) bool {
	if p == "" || path.Clean(p) != p || path.IsAbs(p) {
		return false
	}
	return !slices.ContainsFunc(strings.Split(p, "/"), func(segment string) bool {
		return segment == "." || segment == ".."
	})
}
