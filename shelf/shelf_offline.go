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
)

var ErrDownloadUnsupported = util.NewError("this shelf does not keep offline copies")

var ErrBookNotDownloaded = util.NewError("book is not downloaded and the shelf source is unavailable")

const offlineSchemaVersion = 1
const offlineDownloadsFile = "downloads.json"
const offlineSnapshotFile = "snapshot.json"
const offlineStagingFolder = "tmp"

type DownloadState string

const (
	DownloadNotDownloaded   DownloadState = "not_downloaded"
	DownloadDownloaded      DownloadState = "downloaded"
	DownloadUpdateAvailable DownloadState = "update_available"
)

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

	Version string `json:"version"`

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
	fs     fsutil.FS

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

	if err := store.fs.RemoveAll(offlineStagingFolder); err != nil {
		_ = root.Close()
		return nil, util.Errorf("%w", err)
	}
	if err := store.fs.MkdirAll(offlineStagingFolder); err != nil {
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
	digest, err := bookCacheDigest(file.Listing)
	if err != nil {
		o.logger.Warn("ignoring an unusable offline snapshot", "error", err)
		return
	}
	o.snapshot, o.snapshotDigest = file.Listing, digest
}

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

func (o *offlineStore) writeDownloadsUnderDownloadMu() error {
	o.mu.RLock()
	file := offlineDownloadsJSON{SchemaVersion: offlineSchemaVersion, Books: maps.Clone(o.downloads)}
	o.mu.RUnlock()
	return o.writeJSON(offlineDownloadsFile, file)
}

func (o *offlineStore) pathInUseLocked(pkgPath string) bool {
	for _, record := range o.downloads {
		if record.Path == pkgPath {
			return true
		}
	}
	return false
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

func (o *offlineStore) saveSnapshot(listing *BookCacheFile) error {
	digest, err := bookCacheDigest(listing)
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

type offlineZipFS struct {
	offlineFS
}

func (f offlineZipFS) OpenZip(names ...string) (io.ReadCloser, error) {
	for _, name := range names {
		if _, local := f.route(name); local {
			return nil, errors.ErrUnsupported
		}
	}
	z, ok := f.remote.(fsutil.ZipOpener)
	if !ok {
		return nil, errors.ErrUnsupported
	}
	stream, err := z.OpenZip(names...)
	if err != nil && len(names) > 0 {
		return nil, notDownloaded(names[0], err)
	}
	return stream, err
}

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

func bookVersion(meta *BookMeta, sourceMD5 string) string {
	data, err := json.Marshal(meta, jsonopt.DiskCompact())
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(append(append(data, '\n'), sourceMD5...))
	return hex.EncodeToString(sum[:16])
}

func (s *Shelf) walkRoot() fsutil.ReadFS {
	if s.offline != nil {
		return fsutil.ReadOnly(s.remoteRoot)
	}
	return s.dbRoot
}

func (s *Shelf) afterRemoteListing() {
	if s.offline == nil {
		return
	}
	s.reconcileDownloads()
	s.saveOfflineSnapshot()
}

func (s *Shelf) reconcileDownloads() {
	if s.offline == nil {
		return
	}

	s.bookCache.RLock()
	pathOf := make(map[string]string, len(s.bookCache.cache))
	idAt := make(map[string]string, len(s.bookCache.cache))
	for bookID, entry := range s.bookCache.cache {
		pathOf[bookID] = entry.path
		idAt[entry.path] = bookID
	}
	s.bookCache.RUnlock()

	o := s.offline
	o.mu.Lock()
	defer o.mu.Unlock()
	o.byPath = make(map[string]string, len(o.downloads))
	for bookID, record := range o.downloads {
		if listedPath, ok := pathOf[bookID]; ok && listedPath != record.Path {
			continue
		}
		if other, ok := idAt[record.Path]; ok && other != bookID {
			continue
		}
		o.byPath[record.Path] = bookID
	}
}

func (s *Shelf) CanDownload() bool {
	return s.offline != nil
}

func (s *Shelf) IsVirtual() bool {
	return s.remote
}

func (s *Shelf) SnapshotAt() (time.Time, bool) {
	if !s.remote {
		return time.Time{}, false
	}
	s.bookCache.RLock()
	defer s.bookCache.RUnlock()
	return s.bookCache.lastScanStart, !s.bookCache.lastScanStart.IsZero()
}

func (s *Shelf) DownloadBook(bookID string) (Download, error) {
	if s.offline == nil {
		return Download{}, util.Errorf("%w", ErrDownloadUnsupported)
	}
	if !s.IsReady() {
		return Download{}, s.notReady()
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

	staging := path.Join(offlineStagingFolder, fsutil.RandomString(12))
	size, err := copyTree(s.remoteRoot, o.fs, entry.path, staging)
	if err != nil {
		_ = o.fs.RemoveAll(staging)
		return Download{}, util.Errorf("%w", err)
	}

	if err := o.fs.MkdirAll(path.Dir(entry.path)); err != nil {
		_ = o.fs.RemoveAll(staging)
		return Download{}, util.Errorf("%w", err)
	}

	previous, err := replaceDirKeepingPrevious(o.fs, staging, entry.path)
	if err != nil {
		return Download{}, util.Errorf("%w", err)
	}

	record.SizeBytes = size
	record.DownloadedAt = util.JSONTime(time.Now().UTC().Truncate(time.Second))

	o.mu.Lock()
	old, hadOld := o.downloads[bookID]
	if hadOld && o.byPath[old.Path] == bookID {
		delete(o.byPath, old.Path)
	}
	for otherID, other := range o.downloads {
		if otherID != bookID && other.Path == record.Path {
			delete(o.downloads, otherID)
		}
	}
	o.downloads[bookID] = record
	o.byPath[record.Path] = bookID
	keepOld := hadOld && o.pathInUseLocked(old.Path)
	o.mu.Unlock()

	if previous != "" {
		_ = o.fs.RemoveAll(previous)
	}
	if err := o.writeDownloadsUnderDownloadMu(); err != nil {
		return Download{}, util.Errorf("%w", err)
	}
	if hadOld && old.Path != record.Path && !keepOld {
		_ = o.fs.RemoveAll(old.Path)
	}

	return s.downloadFor(bookID, record), nil
}

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
		if o.byPath[record.Path] == bookID {
			delete(o.byPath, record.Path)
		}
	}
	shared := ok && o.pathInUseLocked(record.Path)
	o.mu.Unlock()
	if !ok {
		return nil
	}

	if err := o.writeDownloadsUnderDownloadMu(); err != nil {
		return util.Errorf("%w", err)
	}
	if shared {
		return nil
	}
	if err := o.fs.RemoveAll(record.Path); err != nil {
		return util.Errorf("%w", err)
	}
	return nil
}

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
	if entry != nil && (entry.path != record.Path || bookVersion(entry.book.GetMeta(), entry.sourceMD5) != record.Version) {
		download.State = DownloadUpdateAvailable
	}
	return download
}

func (s *Shelf) saveOfflineSnapshot() {
	if s.offline == nil {
		return
	}

	listing := s.collectExportSnapshot("offline")
	if err := s.offline.saveSnapshot(&listing); err != nil {
		s.Warn("failed to save the offline snapshot", "error", err)
	}
}

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

func replaceDirKeepingPrevious(root fsutil.FS, staging, dst string) (string, error) {
	previous := ""
	if _, err := root.Stat(dst); err == nil {
		previous = staging + ".previous"
		if err := root.Rename(dst, previous); err != nil {
			_ = root.RemoveAll(staging)
			return "", util.Errorf("%w", err)
		}
	}
	if err := root.Rename(staging, dst); err != nil {
		if previous != "" {
			_ = root.Rename(previous, dst)
		}
		_ = root.RemoveAll(staging)
		return "", util.Errorf("%w", err)
	}

	return previous, nil
}
