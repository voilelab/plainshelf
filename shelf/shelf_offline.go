package shelf

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json/v2"
	"errors"
	"io"
	"io/fs"
	"maps"
	"os"
	"path"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/voilelab/plainshelf/internal/fsutil"
	"github.com/voilelab/plainshelf/internal/jsonopt"
	"github.com/voilelab/plainshelf/internal/logutil"
	"github.com/voilelab/plainshelf/internal/util"
	"github.com/voilelab/plainshelf/internal/version"
)

/*
A virtual shelf opened with an offline directory keeps books on the device. The
directory mirrors the shelf: a downloaded book sits at the same books/… path it
has on the source, so offlineFS answers a read for it from the device by path
alone and the book code never knows.

	{offline}/
	  downloads.json  which books are here, and the version each was taken at
	  snapshot.json   the last listing read from the source
	  shelf.json      the source's shelf.json, so folder rules hold offline
	  books/…         downloaded book packages, as on the source
	  tmp/            downloads in progress, cleared on open

Nothing here is ever written to the source.
*/

var ErrDownloadUnsupported = util.NewError("this shelf does not keep offline copies")

// ErrBookNotDownloaded wraps fsutil.ErrSourceUnavailable for a read inside a
// book that has no offline copy.
var ErrBookNotDownloaded = util.NewError("book is not downloaded and the shelf source is unavailable")

const offlineSchemaVersion = 1
const offlineDownloadsFile = "downloads.json"
const offlineSnapshotFile = "snapshot.json"
const offlineTmpFolder = "tmp"

type DownloadState string

const (
	DownloadNotDownloaded   DownloadState = "not_downloaded"
	DownloadDownloaded      DownloadState = "downloaded"
	DownloadUpdateAvailable DownloadState = "update_available"
)

// Download is one book kept offline.
type Download struct {
	BookID       string
	Folder       FolderPath
	Meta         *BookMeta
	DownloadedAt time.Time
	SizeBytes    int64
	State        DownloadState
}

type offlineDownload struct {
	Path         string        `json:"path"`
	DownloadedAt util.JSONTime `json:"downloaded_at"`
	SizeBytes    int64         `json:"size_bytes"`

	// Version is bookVersion of the listing entry at download time.
	Version string `json:"version"`

	// Meta is kept so the downloads list can name a book the source has since
	// dropped from its listing.
	Meta *BookMeta `json:"meta"`
}

type offlineDownloadsJSON struct {
	SchemaVersion int                        `json:"schema_version"`
	Books         map[string]offlineDownload `json:"books"`
}

type offlineSnapshotJSON struct {
	SchemaVersion int            `json:"schema_version"`
	Listing       *BookCacheFile `json:"listing"`
}

type offlineStore struct {
	logger logutil.Logger
	root   *os.Root
	fs     *fsutil.RootFS

	// downloadMu serialises downloads and removals, which rewrite
	// downloads.json whole.
	downloadMu sync.Mutex

	mu        sync.RWMutex
	downloads map[string]offlineDownload
	byPath    map[string]string

	snapshot       *BookCacheFile
	snapshotDigest string
}

func openOfflineStore(dir string, logger logutil.Logger) (*offlineStore, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, util.Errorf("%w", err)
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	store := &offlineStore{
		logger:    logger,
		root:      root,
		fs:        fsutil.NewRootFS(root),
		downloads: map[string]offlineDownload{},
		byPath:    map[string]string{},
	}

	// Only ever holds unfinished downloads.
	if err := store.fs.RemoveAll(offlineTmpFolder); err != nil {
		_ = root.Close()
		return nil, util.Errorf("%w", err)
	}
	if err := store.fs.MkdirAll(offlineTmpFolder); err != nil {
		_ = root.Close()
		return nil, util.Errorf("%w", err)
	}

	store.loadDownloads()
	store.loadSnapshot()
	return store, nil
}

func (o *offlineStore) close() error {
	if err := o.root.Close(); err != nil {
		return util.Errorf("%w", err)
	}
	return nil
}

// loadDownloads starts empty on an unreadable file: the copies stay on disk
// and a new download of the same book replaces them.
func (o *offlineStore) loadDownloads() {
	var file offlineDownloadsJSON
	if !o.readJSON(offlineDownloadsFile, &file) {
		return
	}
	if file.SchemaVersion != offlineSchemaVersion {
		o.logger.Warn("ignoring an offline download list from another build", "schema_version", file.SchemaVersion)
		return
	}
	for bookID, entry := range file.Books {
		if entry.Meta == nil || entry.Meta.ID != bookID || !isBookPackagePath(entry.Path) {
			o.logger.Warn("ignoring an unusable offline download entry", "book_id", bookID, "path", entry.Path)
			continue
		}
		o.downloads[bookID] = entry
		o.byPath[entry.Path] = bookID
	}
}

func (o *offlineStore) loadSnapshot() {
	var file offlineSnapshotJSON
	if !o.readJSON(offlineSnapshotFile, &file) {
		return
	}
	if file.SchemaVersion != offlineSchemaVersion || file.Listing == nil {
		o.logger.Warn("ignoring an offline snapshot from another build", "schema_version", file.SchemaVersion)
		return
	}
	digest, err := bookCacheDigest(file.Listing.Folders, file.Listing.Books)
	if err != nil {
		o.logger.Warn("ignoring an unusable offline snapshot", "error", err)
		return
	}
	o.snapshot, o.snapshotDigest = file.Listing, digest
}

// readJSON reports whether name was read; a missing file is silent.
func (o *offlineStore) readJSON(name string, v any) bool {
	data, err := fs.ReadFile(o.root.FS(), name)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			o.logger.Warn("failed to read an offline file", "file", name, "error", err)
		}
		return false
	}
	if err := json.Unmarshal(data, v, jsonopt.Disk()); err != nil {
		o.logger.Warn("ignoring an unreadable offline file", "file", name, "error", err)
		return false
	}
	return true
}

func (o *offlineStore) writeJSON(name string, v any) error {
	data, err := json.Marshal(v, jsonopt.Disk())
	if err != nil {
		return util.Errorf("%w", err)
	}
	if err := fsutil.WriteFileAtomic(o.fs, name, data); err != nil {
		return util.Errorf("%w", err)
	}
	return nil
}

// writeDownloadsLocked persists o.downloads; o.mu must not be held, downloadMu
// must be.
func (o *offlineStore) writeDownloadsLocked() error {
	o.mu.RLock()
	file := offlineDownloadsJSON{SchemaVersion: offlineSchemaVersion, Books: maps.Clone(o.downloads)}
	o.mu.RUnlock()
	return o.writeJSON(offlineDownloadsFile, file)
}

func (o *offlineStore) hasPackage(pkgPath string) bool {
	o.mu.RLock()
	defer o.mu.RUnlock()
	_, ok := o.byPath[pkgPath]
	return ok
}

func (o *offlineStore) download(bookID string) (offlineDownload, bool) {
	o.mu.RLock()
	defer o.mu.RUnlock()
	entry, ok := o.downloads[bookID]
	return entry, ok
}

// saveSnapshot writes listing unless its content is what is already stored.
func (o *offlineStore) saveSnapshot(listing *BookCacheFile) error {
	digest, err := bookCacheDigest(listing.Folders, listing.Books)
	if err != nil {
		return util.Errorf("%w", err)
	}

	o.mu.RLock()
	unchanged := digest == o.snapshotDigest
	o.mu.RUnlock()
	if unchanged {
		return nil
	}

	if err := o.writeJSON(offlineSnapshotFile, offlineSnapshotJSON{SchemaVersion: offlineSchemaVersion, Listing: listing}); err != nil {
		return util.Errorf("%w", err)
	}

	o.mu.Lock()
	o.snapshot, o.snapshotDigest = listing, digest
	o.mu.Unlock()
	return nil
}

func (o *offlineStore) lastSnapshot() *BookCacheFile {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.snapshot
}

// mirror keeps the offline copy of a small file in step with the source.
// data nil means the source has no such file.
func (o *offlineStore) mirror(name string, data []byte) {
	current, err := fs.ReadFile(o.root.FS(), name)
	switch {
	case data == nil && errors.Is(err, fs.ErrNotExist):
		return
	case data == nil:
		err = o.fs.Remove(name)
	case err == nil && bytes.Equal(current, data):
		return
	default:
		err = fsutil.WriteFileAtomic(o.fs, name, data)
	}
	if err != nil {
		o.logger.Warn("failed to keep an offline copy of a shelf file", "file", name, "error", err)
	}
}

// offlineFS answers reads inside a downloaded book from the offline store and
// everything else from the source.
type offlineFS struct {
	remote fsutil.ReadFS
	store  *offlineStore
}

func (f offlineFS) route(name string) (fsutil.ReadFS, bool) {
	if pkgPath, ok := bookPackageOf(name); ok && f.store.hasPackage(pkgPath) {
		return f.store.fs, true
	}
	return f.remote, false
}

func (f offlineFS) Open(name string) (fs.File, error) {
	if name == shelfConfigFile {
		return f.openMirrored(name)
	}
	fsys, local := f.route(name)
	file, err := fsys.Open(name)
	if err != nil && !local {
		return nil, notDownloaded(name, err)
	}
	return file, err
}

func (f offlineFS) ReadDir(name string) ([]fs.DirEntry, error) {
	fsys, local := f.route(name)
	entries, err := fsys.ReadDir(name)
	if err != nil && !local {
		return nil, notDownloaded(name, err)
	}
	return entries, err
}

func (f offlineFS) Stat(name string) (fs.FileInfo, error) {
	fsys, local := f.route(name)
	info, err := fsys.Stat(name)
	if err != nil && !local {
		return nil, notDownloaded(name, err)
	}
	return info, err
}

// openMirrored reads name from the source and keeps a copy, falling back to
// the copy while the source is unavailable.
func (f offlineFS) openMirrored(name string) (fs.File, error) {
	file, err := f.remote.Open(name)
	if err != nil {
		if errors.Is(err, fsutil.ErrSourceUnavailable) {
			return f.store.fs.Open(name)
		}
		if errors.Is(err, fs.ErrNotExist) {
			f.store.mirror(name, nil)
		}
		return nil, err
	}
	data, err := io.ReadAll(file)
	_ = file.Close()
	if err != nil {
		return nil, util.Errorf("%w", err)
	}
	f.store.mirror(name, data)
	return f.store.fs.Open(name)
}

func notDownloaded(name string, err error) error {
	if _, inBook := bookPackageOf(name); inBook && errors.Is(err, fsutil.ErrSourceUnavailable) {
		return util.Errorf("%w: %w", ErrBookNotDownloaded, err)
	}
	return err
}

// bookPackageOf returns the book package directory name lies in, or name
// itself when it is one.
func bookPackageOf(name string) (string, bool) {
	segments := strings.Split(name, "/")
	if len(segments) < 2 || segments[0] != booksFolder {
		return "", false
	}
	for i := 1; i < len(segments); i++ {
		if strings.HasSuffix(segments[i], bookExtension) {
			return strings.Join(segments[:i+1], "/"), true
		}
	}
	return "", false
}

// bookVersion identifies what an offline copy was taken of: the book's
// metadata and its current source's text. Nothing in book.json changes when a
// source is edited in place, hence the hash beside it.
func bookVersion(meta *BookMeta, sourceMD5 string) string {
	data, err := json.Marshal(meta, jsonopt.DiskCompact())
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(append(append(data, '\n'), sourceMD5...))
	return hex.EncodeToString(sum[:16])
}

// CanDownload reports whether books on this shelf can be kept offline.
func (s *Shelf) CanDownload() bool {
	return s.offline != nil
}

// IsVirtual reports a shelf opened by NewShelfFromFS.
func (s *Shelf) IsVirtual() bool {
	return s.remote
}

// SnapshotAt is when the walk behind a virtual shelf's listing began, whether
// the listing came from the source just now or from the offline snapshot.
func (s *Shelf) SnapshotAt() (time.Time, bool) {
	if !s.remote {
		return time.Time{}, false
	}
	s.bookCache.RLock()
	defer s.bookCache.RUnlock()
	return s.bookCache.lastScanStart, !s.bookCache.lastScanStart.IsZero()
}

// DownloadBook copies a book package from the source into the offline store,
// replacing an earlier copy. The copy is built under tmp/ and renamed into
// place, so a failure leaves the earlier state.
func (s *Shelf) DownloadBook(bookID string) (Download, error) {
	if s.offline == nil {
		return Download{}, util.Errorf("%w", ErrDownloadUnsupported)
	}
	if !s.IsReady() {
		return Download{}, util.Errorf("%w", ErrShelfInitializing)
	}

	s.bookCache.RLock()
	entry := s.bookCache.cache[bookID]
	s.bookCache.RUnlock()
	if entry == nil {
		return Download{}, util.Errorf("%w", ErrBookNotFound)
	}
	meta := entry.book.GetMeta()
	record := offlineDownload{
		Path:    entry.path,
		Version: bookVersion(meta, entry.sourceMD5),
		Meta:    meta,
	}

	o := s.offline
	o.downloadMu.Lock()
	defer o.downloadMu.Unlock()

	staging := path.Join(offlineTmpFolder, fsutil.RandomString(12))
	size, err := copyTree(s.remoteRoot, o.fs, entry.path, staging)
	if err != nil {
		_ = o.fs.RemoveAll(staging)
		return Download{}, util.Errorf("%w", err)
	}

	if err := o.fs.MkdirAll(path.Dir(entry.path)); err != nil {
		_ = o.fs.RemoveAll(staging)
		return Download{}, util.Errorf("%w", err)
	}

	// An earlier copy is moved aside rather than deleted first, so a failed
	// rename puts it back.
	previous := ""
	if _, err := o.fs.Stat(entry.path); err == nil {
		previous = staging + ".previous"
		if err := o.fs.Rename(entry.path, previous); err != nil {
			_ = o.fs.RemoveAll(staging)
			return Download{}, util.Errorf("%w", err)
		}
	}
	if err := o.fs.Rename(staging, entry.path); err != nil {
		if previous != "" {
			_ = o.fs.Rename(previous, entry.path)
		}
		_ = o.fs.RemoveAll(staging)
		return Download{}, util.Errorf("%w", err)
	}

	record.SizeBytes = size
	record.DownloadedAt = util.JSONTime(time.Now().UTC().Truncate(time.Second))

	o.mu.Lock()
	if old, ok := o.downloads[bookID]; ok {
		delete(o.byPath, old.Path)
	}
	o.downloads[bookID] = record
	o.byPath[record.Path] = bookID
	o.mu.Unlock()

	if previous != "" {
		_ = o.fs.RemoveAll(previous)
	}
	if err := o.writeDownloadsLocked(); err != nil {
		return Download{}, util.Errorf("%w", err)
	}

	return s.downloadFor(bookID, record), nil
}

// RemoveDownload deletes a book's offline copy. A book that has none is not an
// error.
func (s *Shelf) RemoveDownload(bookID string) error {
	if s.offline == nil {
		return util.Errorf("%w", ErrDownloadUnsupported)
	}

	o := s.offline
	o.downloadMu.Lock()
	defer o.downloadMu.Unlock()

	o.mu.Lock()
	record, ok := o.downloads[bookID]
	if ok {
		delete(o.downloads, bookID)
		delete(o.byPath, record.Path)
	}
	o.mu.Unlock()
	if !ok {
		return nil
	}

	// The list first: a copy it no longer names is unreachable, while an entry
	// naming a deleted copy would be read.
	if err := o.writeDownloadsLocked(); err != nil {
		return util.Errorf("%w", err)
	}
	if err := o.fs.RemoveAll(record.Path); err != nil {
		return util.Errorf("%w", err)
	}
	return nil
}

// DownloadStatus reports a book's offline state and, when it has a copy, when
// that was taken.
func (s *Shelf) DownloadStatus(bookID string) (DownloadState, time.Time) {
	if s.offline == nil {
		return DownloadNotDownloaded, time.Time{}
	}
	record, ok := s.offline.download(bookID)
	if !ok {
		return DownloadNotDownloaded, time.Time{}
	}
	return s.downloadFor(bookID, record).State, time.Time(record.DownloadedAt)
}

// Downloads lists every book kept offline, ordered by book ID.
func (s *Shelf) Downloads() []Download {
	if s.offline == nil {
		return nil
	}

	s.offline.mu.RLock()
	records := maps.Clone(s.offline.downloads)
	s.offline.mu.RUnlock()

	downloads := make([]Download, 0, len(records))
	for bookID, record := range records {
		downloads = append(downloads, s.downloadFor(bookID, record))
	}
	slices.SortFunc(downloads, func(a, b Download) int { return strings.Compare(a.BookID, b.BookID) })
	return downloads
}

// downloadFor compares a stored copy with the current listing. A book the
// listing no longer holds keeps its copy and reports it as downloaded.
func (s *Shelf) downloadFor(bookID string, record offlineDownload) Download {
	download := Download{
		BookID:       bookID,
		Folder:       bookFolderPath(record.Path),
		Meta:         record.Meta,
		DownloadedAt: time.Time(record.DownloadedAt),
		SizeBytes:    record.SizeBytes,
		State:        DownloadDownloaded,
	}

	s.bookCache.RLock()
	entry := s.bookCache.cache[bookID]
	s.bookCache.RUnlock()
	if entry != nil && bookVersion(entry.book.GetMeta(), entry.sourceMD5) != record.Version {
		download.State = DownloadUpdateAvailable
	}
	return download
}

// saveOfflineSnapshot stores the current listing for use while the source is
// unavailable.
func (s *Shelf) saveOfflineSnapshot() {
	if s.offline == nil {
		return
	}

	s.bookCache.RLock()
	takenAt := s.bookCache.lastScanStart
	s.bookCache.RUnlock()

	listing := &BookCacheFile{
		SchemaVersion: BookCacheSchemaVersion,
		WriterID:      "offline",
		Timestamp:     takenAt.Unix(),
		Generator:     "plainshelf/" + version.Version,
		Folders:       s.collectExportFolders(),
		Books:         s.collectExportBooks(),
	}
	if err := s.offline.saveSnapshot(listing); err != nil {
		s.Warn("failed to save the offline snapshot", "error", err)
	}
}

// copyTree copies the directory src on from to dst on to and returns the bytes
// copied.
func copyTree(from fsutil.ReadFS, to fsutil.FS, src, dst string) (int64, error) {
	if err := to.MkdirAll(dst); err != nil {
		return 0, util.Errorf("%w", err)
	}
	entries, err := from.ReadDir(src)
	if err != nil {
		return 0, util.Errorf("%w", err)
	}

	var total int64
	for _, entry := range entries {
		srcPath, dstPath := path.Join(src, entry.Name()), path.Join(dst, entry.Name())
		if entry.IsDir() {
			n, err := copyTree(from, to, srcPath, dstPath)
			if err != nil {
				return 0, err
			}
			total += n
			continue
		}

		n, err := copyFile(from, to, srcPath, dstPath)
		if err != nil {
			return 0, err
		}
		total += n
	}
	return total, nil
}

func copyFile(from fsutil.ReadFS, to fsutil.FS, src, dst string) (int64, error) {
	in, err := from.Open(src)
	if err != nil {
		return 0, util.Errorf("%w", err)
	}
	defer in.Close()

	out, err := to.OpenWriter(dst)
	if err != nil {
		return 0, util.Errorf("%w", err)
	}
	n, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil {
		return 0, util.Errorf("%w", copyErr)
	}
	if closeErr != nil {
		return 0, util.Errorf("%w", closeErr)
	}
	return n, nil
}
