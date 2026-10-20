package shelf

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json/v2"
	"path"
	"slices"
	"strings"
	"time"

	"github.com/voilelab/plainshelf/internal/fsutil"
	"github.com/voilelab/plainshelf/internal/jsonopt"
	"github.com/voilelab/plainshelf/internal/util"
	"github.com/voilelab/plainshelf/internal/version"
)

const BookCacheSchemaVersion = 2

const bookCacheFilePrefix = "book-cache-"
const bookCacheFileSuffix = ".json"

const defaultBookCacheInterval = time.Hour

const bookCacheRetention = 30 * 24 * time.Hour

// BookCacheFile is the on-disk shape of app/book-cache-{writer_id}.json.
type BookCacheFile struct {
	// SchemaVersion is declared first so it marshals as the first key, keeping
	// the file self-describing when opened in a text editor (same reasoning as
	// BookMeta.SchemaVersion).
	SchemaVersion int `json:"schema_version"`

	// WriterID identifies the installation that produced this file, so several
	// machines sharing one shelf keep separate caches instead of overwriting
	// each other. See docs/concepts/shelf-cache-and-io.md.
	WriterID string `json:"writer_id"`

	// Timestamp is when the walk BEGAN, in Unix seconds — not when the file was
	// written, and not when the walk finished. A reader compares it against each
	// book.json's mtime: anything at or after it must be re-read.
	//
	// Each choice is the conservative one. A write time would claim freshness the
	// walk never verified; the walk's end time would cover books edited while it
	// was still running; and whole seconds cannot order two events inside one
	// second, which is why equality reads as "must re-read".
	Timestamp int64 `json:"timestamp"`

	// Generator records the build that wrote the file, for diagnostics only.
	Generator string `json:"generator"`

	// Folders lists every folder directory in the shelf, in the same shape the
	// API returns: "/" for the top level, "Fiction/Classics" for a nested one.
	// Recorded explicitly because an empty folder holds no book and could not be
	// reconstructed from Books.
	Folders []string `json:"folders"`

	// FolderMeta is each folder's folder.json as the walk read it, keyed like
	// Folders, for the folders that have one. Added without a version bump: a
	// reader that predates it ignores the member, and a file without it reads
	// as no folder having metadata. Omitted when empty, so a shelf with no
	// folder.json exports the bytes it did before this field existed.
	FolderMeta map[string]BookCacheFolderMeta `json:"folder_meta,omitzero"`

	// Books maps book ID to its location and metadata.
	Books map[string]BookCacheEntry `json:"books"`
}

// BookCacheFolderMeta is one folder's folder.json as exported: what a reader
// shows, not what it writes back, so there is no version to write against.
type BookCacheFolderMeta struct {
	SortKey *float64 `json:"sort_key,omitzero"`
	// Icon is the member as written; a reader shows the default icon for a
	// name it does not offer.
	Icon string `json:"icon,omitzero"`

	// ReadOnly reports a file the writing build could not read safely.
	ReadOnly bool `json:"read_only,omitzero"`
}

// BookCacheEntry is one book as the walk found it.
type BookCacheEntry struct {
	// Path is the book package directory relative to the shelf root, e.g.
	// "books/Fiction/dune.bookpkg". Stored rather than derived because a book's
	// folder name comes from its title at creation time and is independent of
	// its ID afterwards.
	Path string `json:"path"`

	// Meta comes from the in-memory cache rather than a re-read, so exporting
	// costs no per-book I/O — which is the point of the file. One consequence is
	// deliberate: a book.json predating schema versioning exports as version 1,
	// how this build reads it anyway, while a NEWER one keeps its own version so
	// a reader can tell it is not fully understood here.
	Meta *BookMeta `json:"meta"`

	// NSFW is the shelf's assembled answer — see Shelf.IsBookNSFW. Recorded
	// rather than left to the reader because a client that reads only this file
	// has not read shelf.json and cannot apply a folder rule, and two readers
	// disagreeing about adult content is the one disagreement this mark cannot
	// afford. Meta.NSFW stays the book's own half.
	//
	// Omitted when false, so a shelf that marks nothing exports the same bytes it
	// did before this field existed.
	NSFW bool `json:"nsfw,omitzero"`

	// CurrentSourceMD5 is the current source's content hash, so a reader that
	// keeps an offline copy can tell the text changed without reading the
	// source: an edit in place rewrites the source's meta.json, not book.json.
	// Omitted when unknown; a reader treats absent as "no hash to compare".
	CurrentSourceMD5 string `json:"current_source_md5,omitempty"`
}

func (s *Shelf) bookCacheFileName() string {
	return bookCacheFilePrefix + s.bookCacheWriterID + bookCacheFileSuffix
}

func (s *Shelf) scheduleBookCacheExportIfNeeded() {
	if s.bookCacheWriterID == "" {
		return
	}

	s.bookCache.RLock()
	lastExport := s.bookCache.lastExport
	exporting := s.bookCache.exporting
	s.bookCache.RUnlock()

	if exporting || time.Since(lastExport) < s.bookCacheInterval {
		return
	}

	s.bookCache.Lock()
	if s.bookCache.exporting {
		s.bookCache.Unlock()
		return
	}
	s.bookCache.exporting = true
	s.bookCache.Unlock()

	done := func() {
		s.bookCache.Lock()
		s.bookCache.exporting = false
		s.bookCache.Unlock()
	}

	started := s.goBackground(func() {
		defer done()

		if err := s.exportBookCache(false); err != nil {
			s.Warn("failed to export the book cache", "error", err)
		}
	})

	if !started {
		done()
	}
}

func (s *Shelf) ExportBookCache() (time.Time, error) {
	// Before the writer ID, which read_only clears: "not configured" would be
	// true and would send the caller looking for a setting to change, when what
	// they need to know is that this shelf is never written to.
	if s.readOnly {
		return time.Time{}, util.Errorf("%w", fsutil.ErrReadOnly)
	}

	if s.bookCacheWriterID == "" {
		return time.Time{}, util.NewError("book cache export is not configured for this shelf")
	}

	if !s.IsReady() {
		return time.Time{}, s.notReady()
	}

	if err := s.shelfLock.RLock(); err != nil {
		return time.Time{}, util.Errorf("%w", err)
	}
	if err := s.refreshBookCacheIfNeeded(true); err != nil {
		s.shelfLock.Unlock()
		return time.Time{}, util.Errorf("%w", err)
	}
	s.shelfLock.Unlock()

	if err := s.exportBookCache(true); err != nil {
		return time.Time{}, util.Errorf("%w", err)
	}

	s.bookCache.RLock()
	defer s.bookCache.RUnlock()
	return s.bookCache.lastScanStart, nil
}

func (s *Shelf) exportBookCache(force bool) error {
	if s.bookCacheWriterID == "" {
		return nil
	}

	root, err := s.writeRoot()
	if err != nil {
		return util.Errorf("%w", err)
	}

	s.exportMu.Lock()
	defer s.exportMu.Unlock()

	payload := s.collectExportSnapshot(s.bookCacheWriterID)

	digest, err := bookCacheDigest(&payload)
	if err != nil {
		return util.Errorf("%w", err)
	}

	s.bookCache.RLock()
	unchanged := digest == s.bookCache.lastExportDigest
	s.bookCache.RUnlock()

	if !force && unchanged {
		s.bookCache.Lock()
		s.bookCache.lastExport = time.Now()
		s.bookCache.Unlock()
		return nil
	}

	data, err := json.Marshal(payload, jsonopt.Disk())
	if err != nil {
		return util.Errorf("%w", err)
	}

	target := path.Join(appFolder, s.bookCacheFileName())
	if err := fsutil.WriteFileAtomic(root, target, data); err != nil {
		return util.Errorf("%w", err)
	}

	s.bookCache.Lock()
	s.bookCache.lastExport = time.Now()
	s.bookCache.lastExportDigest = digest
	s.bookCache.Unlock()

	s.Debug("exported book cache", "path", target, "books", len(payload.Books), "folders", len(payload.Folders))

	s.pruneStaleBookCaches(root)
	return nil
}

func (s *Shelf) collectExportSnapshot(writerID string) BookCacheFile {
	s.bookCache.RLock()
	defer s.bookCache.RUnlock()
	return BookCacheFile{
		SchemaVersion: BookCacheSchemaVersion,
		WriterID:      writerID,
		Timestamp:     s.bookCache.lastScanStart.Unix(),
		Generator:     "plainshelf/" + version.Version,
		Folders:       s.collectExportFoldersLocked(),
		FolderMeta:    s.collectExportFolderMetaLocked(),
		Books:         s.collectExportBooksLocked(),
	}
}

func (s *Shelf) collectExportFolders() []string {
	s.bookCache.RLock()
	defer s.bookCache.RUnlock()
	return s.collectExportFoldersLocked()
}

func (s *Shelf) collectExportFoldersLocked() []string {
	var folders []string
	for _, folder := range s.bookCache.folders {
		name := folder.String()
		if name == "" {
			name = "/"
		}
		folders = append(folders, name)
	}

	slices.Sort(folders)
	return folders
}

func (s *Shelf) collectExportFolderMetaLocked() map[string]BookCacheFolderMeta {
	if len(s.bookCache.folderMeta) == 0 {
		return nil
	}
	metas := make(map[string]BookCacheFolderMeta, len(s.bookCache.folderMeta))
	for name, meta := range s.bookCache.folderMeta {
		entry := BookCacheFolderMeta{Icon: meta.Icon, ReadOnly: meta.ReadOnly}
		if meta.HasSortKey {
			key := meta.SortKey
			entry.SortKey = &key
		}
		metas[name] = entry
	}
	return metas
}

func (s *Shelf) collectExportBooksLocked() map[string]BookCacheEntry {
	books := make(map[string]BookCacheEntry, len(s.bookCache.cache))
	for bookID, entry := range s.bookCache.cache {
		meta := entry.book.GetMeta()
		books[bookID] = BookCacheEntry{
			Path:             entry.path,
			Meta:             meta,
			NSFW:             s.IsBookNSFW(entry.folders, meta),
			CurrentSourceMD5: entry.sourceMD5,
		}
	}
	return books
}

// bookCacheDigest fingerprints the exportable content of the shelf. Marshalling
// is enough to make it order-independent, but only because jsonopt.DiskCompact
// carries Deterministic: json/v2 leaves map order unspecified otherwise, and an
// unchanged shelf would re-export on every scan. The folder slice is already
// sorted.
func bookCacheDigest(cache *BookCacheFile) (string, error) {
	data, err := json.Marshal(struct {
		Folders    []string                       `json:"folders"`
		FolderMeta map[string]BookCacheFolderMeta `json:"folder_meta,omitzero"`
		Books      map[string]BookCacheEntry      `json:"books"`
	}{Folders: cache.Folders, FolderMeta: cache.FolderMeta, Books: cache.Books}, jsonopt.DiskCompact())
	if err != nil {
		return "", util.Errorf("%w", err)
	}

	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func (s *Shelf) pruneStaleBookCaches(root fsutil.FS) {
	entries, err := s.dbRoot.ReadDir(appFolder)
	if err != nil {
		s.Warn("failed to list the app folder while pruning book caches", "error", err)
		return
	}

	cutoff := time.Now().Add(-bookCacheRetention)
	own := s.bookCacheFileName()

	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || name == own {
			continue
		}
		if !strings.HasPrefix(name, bookCacheFilePrefix) || !strings.HasSuffix(name, bookCacheFileSuffix) {
			continue
		}

		filePath := path.Join(appFolder, name)
		cache, err := s.readBookCacheFile(filePath)
		if err != nil {
			s.Debug("keeping an unreadable book cache file", "path", filePath, "error", err)
			continue
		}
		if cache.WriterID == s.bookCacheWriterID || !time.Unix(cache.Timestamp, 0).Before(cutoff) {
			continue
		}

		if err := root.Remove(filePath); err != nil {
			s.Warn("failed to remove a stale book cache file", "path", filePath, "error", err)
			continue
		}
		s.Debug("removed a stale book cache file", "path", filePath, "writer_id", cache.WriterID)
	}
}

func (s *Shelf) readBookCacheFile(filePath string) (*BookCacheFile, error) {
	file, err := s.dbRoot.Open(filePath)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}
	defer file.Close()

	var cache BookCacheFile
	if err := json.UnmarshalRead(file, &cache); err != nil {
		return nil, util.Errorf("%w", err)
	}
	if cache.SchemaVersion != BookCacheSchemaVersion || cache.WriterID == "" || cache.Timestamp == 0 {
		return nil, util.NewError("not a book cache file this build recognizes")
	}
	return &cache, nil
}
