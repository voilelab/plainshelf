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

/*
The exported book cache is a read-optimized copy of the shelf listing, written
to app/book-cache-{writer_id}.json so that a client which reaches the shelf over
a slow, request-metered transport does not have to walk it.

The Android client reading a shelf from pCloud is the case this exists for: a
cold scan there costs two HTTP requests per book (get a link, then download
book.json), which is slow and can hit pCloud's rate limit. With this file it
costs one download regardless of how many books the shelf holds.

The file is disposable by design. It lives under app/, which the data model
already documents as rebuildable and skippable in backups, and every reader
treats a missing, unparseable or too-new file as a plain cache miss and falls
back to walking the shelf.
*/

// BookCacheSchemaVersion is the exported cache format version this build writes.
//
// Unlike book.json there is no migration path and none is needed: the file can
// always be regenerated from the shelf, so a reader that sees a version it does
// not know discards the file rather than trying to interpret it.
//
// v2 renamed the folder-list key from "layers" to "folders" (the layer→folder
// surface rename). Because a version mismatch is already a plain cache miss, an
// existing v1 export is simply discarded and rebuilt on the next export; no error
// reaches the caller.
const BookCacheSchemaVersion = 2

const bookCacheFilePrefix = "book-cache-"
const bookCacheFileSuffix = ".json"

// defaultBookCacheInterval is how often an exported cache is refreshed at most.
const defaultBookCacheInterval = time.Hour

// bookCacheRetention is how old another writer's cache must be before this one
// deletes it. Long enough that a machine which is simply switched off for a
// holiday keeps its entry, short enough that a decommissioned device does not
// litter app/ forever.
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

	// Timestamp is when the walk behind this file BEGAN, in Unix seconds — not
	// when the file was written, and not when the walk finished. A reader
	// compares it against the modification time of each book.json it can see:
	// anything at or after this may have changed since the walk read it and
	// must be re-read, everything older is covered by Books below.
	//
	// Each of those three choices is the conservative one. A write time would
	// claim freshness the walk never verified. The walk's end time would cover
	// books edited while it was still running, after it had already read them.
	// And whole seconds cannot order two events inside the same second, which
	// is why a reader treats equality as "must re-read" rather than "covered".
	Timestamp int64 `json:"timestamp"`

	// Generator records the build that wrote the file, for diagnostics only.
	Generator string `json:"generator"`

	// Folders lists every folder directory in the shelf, in the same shape the
	// API returns: "/" for the top level, "Fiction/Classics" for a nested one.
	// Recorded explicitly because an empty folder holds no book and could not be
	// reconstructed from Books.
	Folders []string `json:"folders"`

	// Books maps book ID to its location and metadata.
	Books map[string]BookCacheEntry `json:"books"`
}

// BookCacheEntry is one book as the walk found it.
type BookCacheEntry struct {
	// Path is the book package directory relative to the shelf root, e.g.
	// "books/Fiction/dune.bookpkg". Stored rather than derived because a book's
	// folder name comes from its title at creation time and is independent of
	// its ID afterwards.
	Path string `json:"path"`

	// Meta is the content of the book's book.json.
	//
	// Taken from the in-memory cache rather than re-read from disk, so exporting
	// costs no per-book I/O — the point of the file is to spare a slow shelf
	// exactly that kind of walk. One consequence is deliberate: a book.json
	// predating schema versioning is exported as version 1, which is how this
	// build reads it anyway, while a book whose version is NEWER than this build
	// keeps its own version so a reader can still tell it is not fully
	// understood here.
	Meta *BookMeta `json:"meta"`
}

// bookCacheFileName is the file this installation owns. Every other
// book-cache-*.json in app/ belongs to a different machine.
func (s *Shelf) bookCacheFileName() string {
	return bookCacheFilePrefix + s.bookCacheWriterID + bookCacheFileSuffix
}

// scheduleBookCacheExportIfNeeded considers an export in the background once the
// interval has elapsed. Whether anything is written is decided by comparing
// content, in exportBookCache.
//
// Deciding on content rather than a "something changed" flag is the point: the
// cache holds pointers to live *Book values, and every metadata edit — SetMeta,
// SetCover, DeleteCover, SetCurrentSource — mutates one in place without going
// near this file. A flag would have to be set from each, and the one missed
// would silently stop exporting.
//
// There is no ticker either. An export only has something to say after the shelf
// is read or written, and every such path passes through here, so polling would
// add a goroutine and a shutdown concern without making any file fresher. A
// shelf closed with unexported changes is handled by Close.
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
			// A cache the mobile client can fall back from is not worth failing
			// the read that happened to trigger it.
			s.Warn("failed to export the book cache", "error", err)
		}
	})

	// Same as the book check: the claim has to be released when nothing took it.
	// Close exports once more itself, so nothing is lost by skipping this one.
	if !started {
		done()
	}
}

// ExportBookCache rescans the shelf and writes the exported cache immediately,
// ignoring the interval. This is the manual trigger behind the API endpoint.
//
// The rescan is not optional: Timestamp promises a walk began at that moment,
// so writing without one would advertise freshness that was never checked.
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
		return time.Time{}, util.Errorf("%w", ErrShelfInitializing)
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

// exportBookCache builds the cache payload and writes it when it differs from
// what was written last. With force it writes unconditionally.
//
// It intentionally does not take the shelf lock. The only file it writes is
// named after this installation, so no other instance contends for it, and the
// write is atomic. Reading a directory tree while another process mutates it
// can miss a book being added — exactly the staleness this file is already
// defined to tolerate, and the next export corrects it. Taking the lock here
// would instead mean acquiring it from Close and from goroutines whose caller
// already holds it.
func (s *Shelf) exportBookCache(force bool) error {
	if s.bookCacheWriterID == "" {
		return nil
	}

	// A read-only shelf never reaches this point, because read_only clears the
	// writer ID the guard above checks. Narrowing here anyway keeps that true
	// if the two ever stop being wired together.
	root, err := s.writeRoot()
	if err != nil {
		return util.Errorf("%w", err)
	}

	s.exportMu.Lock()
	defer s.exportMu.Unlock()

	folders := s.collectExportFolders()
	books := s.collectExportBooks()

	// Hash only the content, never the timestamp, so an unchanged shelf does
	// not rewrite the file every interval. That matters most on the transports
	// this file exists for: on pCloud or SMB an identical rewrite is a pointless
	// upload, and on a sync client it is a pointless conflict opportunity.
	digest, err := bookCacheDigest(folders, books)
	if err != nil {
		return util.Errorf("%w", err)
	}

	s.bookCache.RLock()
	scannedAt := s.bookCache.lastScanStart
	unchanged := digest == s.bookCache.lastExportDigest
	s.bookCache.RUnlock()

	if !force && unchanged {
		// Nothing to say, but the question has now been asked and answered.
		s.bookCache.Lock()
		s.bookCache.lastExport = time.Now()
		s.bookCache.Unlock()
		return nil
	}

	payload := BookCacheFile{
		SchemaVersion: BookCacheSchemaVersion,
		WriterID:      s.bookCacheWriterID,
		Timestamp:     scannedAt.Unix(),
		Generator:     "plainshelf/" + version.Version,
		Folders:       folders,
		Books:         books,
	}

	data, err := json.Marshal(payload, jsonopt.Disk())
	if err != nil {
		return util.Errorf("%w", err)
	}

	// WriteFileAtomic, not WriteFile: a reader may be a sync client copying the
	// shelf away, and it must never see a half-written listing. Its temp naming
	// is also the one scans deliberately ignore (see book.go).
	target := path.Join(appFolder, s.bookCacheFileName())
	if err := fsutil.WriteFileAtomic(root, target, data); err != nil {
		return util.Errorf("%w", err)
	}

	s.bookCache.Lock()
	s.bookCache.lastExport = time.Now()
	s.bookCache.lastExportDigest = digest
	s.bookCache.Unlock()

	s.Debug("exported book cache", "path", target, "books", len(books), "folders", len(folders))

	s.pruneStaleBookCaches(root)
	return nil
}

// collectExportFolders lists every folder directory, in the shape the API and the
// pCloud client both use ("/" for the top level).
//
// Snapshots the in-memory cache, like collectExportBooks: both halves of the
// file then describe the same walk, and an export costs no filesystem access of
// its own. The scan behind that cache is what ExportBookCache forces before
// writing.
func (s *Shelf) collectExportFolders() []string {
	var folders []string
	for _, folder := range s.listFoldersFromCache() {
		name := folder.String()
		if name == "" {
			name = "/"
		}
		folders = append(folders, name)
	}

	slices.Sort(folders)
	return folders
}

// collectExportBooks snapshots the in-memory cache. No filesystem access: the
// cache already holds every book.json this process has read.
func (s *Shelf) collectExportBooks() map[string]BookCacheEntry {
	s.bookCache.RLock()
	defer s.bookCache.RUnlock()

	books := make(map[string]BookCacheEntry, len(s.bookCache.cache))
	for bookID, entry := range s.bookCache.cache {
		books[bookID] = BookCacheEntry{
			Path: entry.path,
			Meta: entry.book.GetMeta(),
		}
	}
	return books
}

// bookCacheDigest fingerprints the exportable content of the shelf. Marshalling
// is enough to make it order-independent, but only because jsonopt.DiskCompact
// carries Deterministic: json/v2 leaves map order unspecified otherwise, and an
// unchanged shelf would re-export on every scan. The folder slice is already
// sorted.
func bookCacheDigest(folders []string, books map[string]BookCacheEntry) (string, error) {
	data, err := json.Marshal(struct {
		Folders []string                  `json:"folders"`
		Books   map[string]BookCacheEntry `json:"books"`
	}{Folders: folders, Books: books}, jsonopt.DiskCompact())
	if err != nil {
		return "", util.Errorf("%w", err)
	}

	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

// pruneStaleBookCaches removes cache files left behind by installations that
// have not written for bookCacheRetention.
//
// Only files this code can fully parse and that name a different writer are
// removed. Anything unreadable is left alone: app/ is shared with whatever else
// a user or a future build puts there, and silently deleting a file we do not
// understand is a worse failure than leaving a stale one.
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
	if err := json.UnmarshalRead(file, &cache, jsonopt.Read()); err != nil {
		return nil, util.Errorf("%w", err)
	}
	if cache.SchemaVersion != BookCacheSchemaVersion || cache.WriterID == "" || cache.Timestamp == 0 {
		return nil, util.NewError("not a book cache file this build recognizes")
	}
	return &cache, nil
}
