package shelf

import (
	"errors"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/voilelab/plainshelf/internal/fsutil"
	"github.com/voilelab/plainshelf/internal/util"
)

// switchFS stands in for a remote source that can go offline: every call
// fails with fsutil.ErrSourceUnavailable while down is set.
type switchFS struct {
	inner fsutil.ReadFS
	down  atomic.Bool
}

func (f *switchFS) check(name string) error {
	if f.down.Load() {
		return util.Errorf("%s: %w", name, fsutil.ErrSourceUnavailable)
	}
	return nil
}

func (f *switchFS) Open(name string) (fs.File, error) {
	if err := f.check(name); err != nil {
		return nil, err
	}
	return f.inner.Open(name)
}

func (f *switchFS) ReadDir(name string) ([]fs.DirEntry, error) {
	if err := f.check(name); err != nil {
		return nil, err
	}
	return f.inner.ReadDir(name)
}

func (f *switchFS) Stat(name string) (fs.FileInfo, error) {
	if err := f.check(name); err != nil {
		return nil, err
	}
	return f.inner.Stat(name)
}

// failAfterFS fails every Open after the first n inside books/, cutting a
// download off part way.
type failAfterFS struct {
	fsutil.ReadFS
	n atomic.Int32
}

func (f *failAfterFS) Open(name string) (fs.File, error) {
	if strings.HasPrefix(name, booksFolder+"/") && f.n.Add(-1) < 0 {
		return nil, util.Errorf("%s: %w", name, fsutil.ErrSourceUnavailable)
	}
	return f.ReadFS.Open(name)
}

// offlineFixture is a source shelf with two books, each with text and a
// cover, exported the way a sharing server would.
type offlineFixture struct {
	libRoot string
	ids     map[string]string // title → book ID
}

func seedOfflineSource(t *testing.T) offlineFixture {
	t.Helper()

	libRoot := t.TempDir()
	s := newTestShelf(t, &ShelfConf{LibRoot: libRoot, LockMode: "none", BookCacheWriterID: testWriterID})
	fixture := offlineFixture{libRoot: libRoot, ids: map[string]string{}}
	for i, title := range []string{"Dune", "Solaris"} {
		folder := FolderPath{}
		if i == 1 {
			folder = FolderPath{"Fiction"}
		}
		book, err := s.NewBook(folder, title)
		if err != nil {
			t.Fatalf("NewBook(%q): %v", title, err)
		}
		source, err := book.NewSource(strings.NewReader(title + " text"))
		if err != nil {
			t.Fatalf("NewSource: %v", err)
		}
		if err := book.SetCurrentSource(source.ID()); err != nil {
			t.Fatalf("SetCurrentSource: %v", err)
		}
		if err := book.SetCover([]byte("cover of "+title), "png"); err != nil {
			t.Fatalf("SetCover: %v", err)
		}
		fixture.ids[title] = book.ID()
	}
	if _, err := s.ExportBookCache(); err != nil {
		t.Fatalf("ExportBookCache: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	return fixture
}

// openOffline opens libRoot as a virtual shelf behind a switchFS and a
// countingFS, keeping its offline store in offlineDir.
func openOffline(t *testing.T, libRoot, offlineDir string, down bool) (*Shelf, *switchFS, *countingFS) {
	t.Helper()

	rt, err := os.OpenRoot(libRoot)
	if err != nil {
		t.Fatalf("OpenRoot: %v", err)
	}
	t.Cleanup(func() { rt.Close() })

	counter := &countingFS{inner: fsutil.NewRootFS(rt)}
	source := &switchFS{inner: counter}
	source.down.Store(down)

	s, err := NewShelfFromFS(&ShelfConf{ScanInterval: "1h"}, source, offlineDir)
	if err != nil {
		t.Fatalf("NewShelfFromFS: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	if err := s.WaitReady(t.Context()); err != nil {
		t.Fatalf("WaitReady: %v", err)
	}
	return s, source, counter
}

func readCurrentText(t *testing.T, s *Shelf, bookID string) (string, error) {
	t.Helper()

	book, err := s.GetBook(bookID)
	if err != nil {
		return "", err
	}
	source, err := book.ResolveCurrentSource()
	if err != nil {
		return "", err
	}
	data, err := source.ReadContent()
	return string(data), err
}

func TestDownloadMirrorsThePackageOutsideTheShelf(t *testing.T) {
	fixture := seedOfflineSource(t)
	offlineDir := t.TempDir()
	s, _, _ := openOffline(t, fixture.libRoot, offlineDir, false)

	download, err := s.DownloadBook(fixture.ids["Solaris"])
	if err != nil {
		t.Fatalf("DownloadBook: %v", err)
	}
	if download.State != DownloadDownloaded || download.SizeBytes == 0 || download.DownloadedAt.IsZero() {
		t.Errorf("download = %+v, want a dated, sized, downloaded copy", download)
	}

	book, err := s.GetBook(fixture.ids["Solaris"])
	if err != nil {
		t.Fatalf("GetBook: %v", err)
	}
	var sourceFiles, offlineFiles []string
	walkFiles(t, filepath.Join(fixture.libRoot, book.PackagePath()), &sourceFiles)
	walkFiles(t, filepath.Join(offlineDir, book.PackagePath()), &offlineFiles)
	if len(sourceFiles) == 0 || !slices.Equal(sourceFiles, offlineFiles) {
		t.Errorf("offline copy holds %v, want the source package %v", offlineFiles, sourceFiles)
	}

	if entries, err := os.ReadDir(filepath.Join(offlineDir, offlineTmpFolder)); err != nil || len(entries) > 0 {
		t.Errorf("tmp/ after a download = %v (%v), want empty", entries, err)
	}
	if _, err := os.Stat(filepath.Join(offlineDir, offlineDownloadsFile)); err != nil {
		t.Errorf("download list: %v", err)
	}
}

func walkFiles(t *testing.T, root string, out *[]string) {
	t.Helper()
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			rel, _ := filepath.Rel(root, p)
			*out = append(*out, filepath.ToSlash(rel))
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	slices.Sort(*out)
}

func TestInterruptedDownloadLeavesNoPartialBook(t *testing.T) {
	fixture := seedOfflineSource(t)
	offlineDir := t.TempDir()

	rt, err := os.OpenRoot(fixture.libRoot)
	if err != nil {
		t.Fatalf("OpenRoot: %v", err)
	}
	t.Cleanup(func() { rt.Close() })
	source := &failAfterFS{ReadFS: fsutil.NewRootFS(rt)}
	source.n.Store(1 << 20)

	s, err := NewShelfFromFS(&ShelfConf{ScanInterval: "1h"}, source, offlineDir)
	if err != nil {
		t.Fatalf("NewShelfFromFS: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	if err := s.WaitReady(t.Context()); err != nil {
		t.Fatalf("WaitReady: %v", err)
	}

	source.n.Store(1)
	if _, err := s.DownloadBook(fixture.ids["Dune"]); !errors.Is(err, fsutil.ErrSourceUnavailable) {
		t.Fatalf("DownloadBook error = %v, want %v", err, fsutil.ErrSourceUnavailable)
	}

	if _, err := os.Stat(filepath.Join(offlineDir, booksFolder)); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("books/ after a failed download: %v, want absent", err)
	}
	if entries, _ := os.ReadDir(filepath.Join(offlineDir, offlineTmpFolder)); len(entries) > 0 {
		t.Errorf("tmp/ after a failed download holds %v", entries)
	}
	if state, _ := s.DownloadStatus(fixture.ids["Dune"]); state != DownloadNotDownloaded {
		t.Errorf("state = %q, want %q", state, DownloadNotDownloaded)
	}
}

func TestDownloadedBookIsReadWithoutTheSource(t *testing.T) {
	fixture := seedOfflineSource(t)
	offlineDir := t.TempDir()
	s, _, counter := openOffline(t, fixture.libRoot, offlineDir, false)

	dune := fixture.ids["Dune"]
	if _, err := s.DownloadBook(dune); err != nil {
		t.Fatalf("DownloadBook: %v", err)
	}
	counter.take()

	text, err := readCurrentText(t, s, dune)
	if err != nil || text != "Dune text" {
		t.Fatalf("read = %q, %v; want the downloaded text", text, err)
	}
	book, _ := s.GetBook(dune)
	if cover, _, err := book.OpenCover(); err != nil || string(cover) != "cover of Dune" {
		t.Fatalf("cover = %q, %v", cover, err)
	}

	if calls := counter.take(); len(calls) > 0 {
		t.Errorf("reading a downloaded book reached the source: %v", calls)
	}
}

func TestOfflineShelfListsFromSnapshot(t *testing.T) {
	fixture := seedOfflineSource(t)
	offlineDir := t.TempDir()

	online, _, _ := openOffline(t, fixture.libRoot, offlineDir, false)
	if _, err := online.DownloadBook(fixture.ids["Dune"]); err != nil {
		t.Fatalf("DownloadBook: %v", err)
	}
	snapshotAt, ok := online.SnapshotAt()
	if !ok {
		t.Fatal("SnapshotAt reported no listing on a listed virtual shelf")
	}
	if err := online.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	s, _, _ := openOffline(t, fixture.libRoot, offlineDir, true)
	if got, want := listedTitles(t, s), []string{"Dune", "Solaris"}; !slices.Equal(got, want) {
		t.Errorf("offline listing = %v, want %v", got, want)
	}
	if got, ok := s.SnapshotAt(); !ok || !got.Equal(snapshotAt) {
		t.Errorf("SnapshotAt offline = %v, %v; want %v", got, ok, snapshotAt)
	}

	if text, err := readCurrentText(t, s, fixture.ids["Dune"]); err != nil || text != "Dune text" {
		t.Errorf("downloaded book offline = %q, %v", text, err)
	}
	if _, err := readCurrentText(t, s, fixture.ids["Solaris"]); !errors.Is(err, ErrBookNotDownloaded) {
		t.Errorf("book without a copy offline: error = %v, want %v", err, ErrBookNotDownloaded)
	}
}

func TestOfflineShelfWithoutSnapshotFailsToOpen(t *testing.T) {
	fixture := seedOfflineSource(t)

	rt, err := os.OpenRoot(fixture.libRoot)
	if err != nil {
		t.Fatalf("OpenRoot: %v", err)
	}
	t.Cleanup(func() { rt.Close() })
	source := &switchFS{inner: fsutil.NewRootFS(rt)}
	source.down.Store(true)

	s, err := NewShelfFromFS(&ShelfConf{}, source, t.TempDir())
	if err != nil {
		t.Fatalf("NewShelfFromFS: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	if err := s.WaitReady(t.Context()); !errors.Is(err, fsutil.ErrSourceUnavailable) {
		t.Errorf("WaitReady = %v, want %v", err, fsutil.ErrSourceUnavailable)
	}
}

func TestSourceGoingOfflineKeepsTheListing(t *testing.T) {
	fixture := seedOfflineSource(t)
	s, source, _ := openOffline(t, fixture.libRoot, t.TempDir(), false)
	if err := s.SetScanInterval("1ms"); err != nil {
		t.Fatalf("SetScanInterval: %v", err)
	}

	source.down.Store(true)
	time.Sleep(5 * time.Millisecond)
	if got, want := listedTitles(t, s), []string{"Dune", "Solaris"}; !slices.Equal(got, want) {
		t.Errorf("listing after the source went down = %v, want %v", got, want)
	}
}

func TestEditedSourceMarksTheCopyUpdateAvailable(t *testing.T) {
	fixture := seedOfflineSource(t)
	s, _, _ := openOffline(t, fixture.libRoot, t.TempDir(), false)
	dune := fixture.ids["Dune"]
	if _, err := s.DownloadBook(dune); err != nil {
		t.Fatalf("DownloadBook: %v", err)
	}

	// The exporter edits the text in place: book.json does not change.
	local := newTestShelf(t, &ShelfConf{LibRoot: fixture.libRoot, LockMode: "none", BookCacheWriterID: testWriterID})
	book, err := local.GetBook(dune)
	if err != nil {
		t.Fatalf("GetBook: %v", err)
	}
	source, err := book.ResolveCurrentSource()
	if err != nil {
		t.Fatalf("ResolveCurrentSource: %v", err)
	}
	if err := source.UpdateContent(strings.NewReader("Dune, revised")); err != nil {
		t.Fatalf("UpdateContent: %v", err)
	}
	local.RefreshBookCharCount(dune)
	if _, err := local.ExportBookCache(); err != nil {
		t.Fatalf("ExportBookCache: %v", err)
	}
	cachePath := path.Join(fixture.libRoot, appFolder, bookCacheFilePrefix+testWriterID+bookCacheFileSuffix)
	cache, err := readBookCacheFileAt(cachePath)
	if err != nil {
		t.Fatalf("read export: %v", err)
	}
	cache.Timestamp += 10
	writeRawBookCache(t, fixture.libRoot, path.Base(cachePath), cache)

	if err := s.refreshBookCacheIfNeeded(true); err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if state, _ := s.DownloadStatus(dune); state != DownloadUpdateAvailable {
		t.Errorf("state after an edit in place = %q, want %q", state, DownloadUpdateAvailable)
	}
	if text, _ := readCurrentText(t, s, dune); text != "Dune text" {
		t.Errorf("copy read %q before re-downloading, want the downloaded text", text)
	}

	if _, err := s.DownloadBook(dune); err != nil {
		t.Fatalf("re-download: %v", err)
	}
	if state, _ := s.DownloadStatus(dune); state != DownloadDownloaded {
		t.Errorf("state after re-downloading = %q, want %q", state, DownloadDownloaded)
	}
	if text, _ := readCurrentText(t, s, dune); text != "Dune, revised" {
		t.Errorf("copy read %q after re-downloading, want the edited text", text)
	}
}

func TestUnchangedListingDoesNotRewriteTheSnapshot(t *testing.T) {
	fixture := seedOfflineSource(t)
	offlineDir := t.TempDir()
	s, _, _ := openOffline(t, fixture.libRoot, offlineDir, false)

	snapshotPath := filepath.Join(offlineDir, offlineSnapshotFile)
	past := time.Now().Add(-time.Hour).Truncate(time.Second)
	if err := os.Chtimes(snapshotPath, past, past); err != nil {
		t.Fatalf("Chtimes: %v", err)
	}

	// A new export of the same content.
	cachePath := path.Join(fixture.libRoot, appFolder, bookCacheFilePrefix+testWriterID+bookCacheFileSuffix)
	cache, err := readBookCacheFileAt(cachePath)
	if err != nil {
		t.Fatalf("read export: %v", err)
	}
	cache.Timestamp += 10
	writeRawBookCache(t, fixture.libRoot, path.Base(cachePath), cache)
	if err := s.refreshBookCacheIfNeeded(true); err != nil {
		t.Fatalf("refresh: %v", err)
	}

	info, err := os.Stat(snapshotPath)
	if err != nil {
		t.Fatalf("stat snapshot: %v", err)
	}
	if !info.ModTime().Equal(past) {
		t.Errorf("snapshot rewritten at %v for an unchanged listing", info.ModTime())
	}
}

func TestRemovingADownloadTouchesOnlyTheOfflineStore(t *testing.T) {
	fixture := seedOfflineSource(t)
	offlineDir := t.TempDir()
	s, _, _ := openOffline(t, fixture.libRoot, offlineDir, false)
	dune := fixture.ids["Dune"]
	if _, err := s.DownloadBook(dune); err != nil {
		t.Fatalf("DownloadBook: %v", err)
	}
	book, _ := s.GetBook(dune)

	if err := s.RemoveDownload(dune); err != nil {
		t.Fatalf("RemoveDownload: %v", err)
	}
	if state, _ := s.DownloadStatus(dune); state != DownloadNotDownloaded {
		t.Errorf("state = %q, want %q", state, DownloadNotDownloaded)
	}
	if _, err := os.Stat(filepath.Join(offlineDir, book.PackagePath())); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("offline copy after removal: %v, want absent", err)
	}
	if _, err := os.Stat(filepath.Join(fixture.libRoot, book.PackagePath())); err != nil {
		t.Errorf("source package after removal: %v", err)
	}
	if len(s.Downloads()) != 0 {
		t.Errorf("Downloads() = %v, want none", s.Downloads())
	}
	if err := s.RemoveDownload(dune); err != nil {
		t.Errorf("removing a download twice: %v", err)
	}
}

func TestDownloadsSurviveReopening(t *testing.T) {
	fixture := seedOfflineSource(t)
	offlineDir := t.TempDir()
	s, _, _ := openOffline(t, fixture.libRoot, offlineDir, false)
	if _, err := s.DownloadBook(fixture.ids["Dune"]); err != nil {
		t.Fatalf("DownloadBook: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	reopened, _, counter := openOffline(t, fixture.libRoot, offlineDir, false)
	downloads := reopened.Downloads()
	if len(downloads) != 1 || downloads[0].BookID != fixture.ids["Dune"] || downloads[0].State != DownloadDownloaded {
		t.Fatalf("Downloads() after reopening = %+v", downloads)
	}
	counter.take()
	if _, err := readCurrentText(t, reopened, fixture.ids["Dune"]); err != nil {
		t.Fatalf("read: %v", err)
	}
	if calls := counter.take(); len(calls) > 0 {
		t.Errorf("reading a downloaded book after reopening reached the source: %v", calls)
	}
}

func TestLocalShelfDoesNotDownload(t *testing.T) {
	s := newTestShelf(t, &ShelfConf{LibRoot: t.TempDir(), LockMode: "none"})
	book, err := s.NewBook(FolderPath{}, "Dune")
	if err != nil {
		t.Fatalf("NewBook: %v", err)
	}
	if s.CanDownload() || s.IsVirtual() {
		t.Error("a local shelf reports itself virtual or downloadable")
	}
	if _, err := s.DownloadBook(book.ID()); !errors.Is(err, ErrDownloadUnsupported) {
		t.Errorf("DownloadBook error = %v, want %v", err, ErrDownloadUnsupported)
	}
}

func TestOfflineShelfKeepsFolderRules(t *testing.T) {
	fixture := seedOfflineSource(t)
	rules := `{"content": {"nsfw_folders": [{"path": "Fiction"}]}}`
	if err := os.WriteFile(filepath.Join(fixture.libRoot, shelfConfigFile), []byte(rules), 0644); err != nil {
		t.Fatalf("write shelf.json: %v", err)
	}
	offlineDir := t.TempDir()
	online, _, _ := openOffline(t, fixture.libRoot, offlineDir, false)
	if err := online.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	s, _, _ := openOffline(t, fixture.libRoot, offlineDir, true)
	if _, ok := s.NSFWFolderRule(FolderPath{"Fiction"}); !ok {
		t.Error("folder rule from shelf.json lost while the source is unavailable")
	}
}

// reexport has the exporter at libRoot export again, dated later than the
// listing a test shelf already holds.
func reexport(t *testing.T, local *Shelf, libRoot string) {
	t.Helper()
	if _, err := local.ExportBookCache(); err != nil {
		t.Fatalf("ExportBookCache: %v", err)
	}
	cachePath := path.Join(libRoot, appFolder, bookCacheFilePrefix+testWriterID+bookCacheFileSuffix)
	cache, err := readBookCacheFileAt(cachePath)
	if err != nil {
		t.Fatalf("read export: %v", err)
	}
	cache.Timestamp += 10
	writeRawBookCache(t, libRoot, path.Base(cachePath), cache)
}

func TestCopyIsNotReadForAnotherBookAtItsPath(t *testing.T) {
	fixture := seedOfflineSource(t)
	offlineDir := t.TempDir()
	s, _, _ := openOffline(t, fixture.libRoot, offlineDir, false)
	if _, err := s.DownloadBook(fixture.ids["Dune"]); err != nil {
		t.Fatalf("DownloadBook: %v", err)
	}

	// The source deletes Dune and makes a new book of the same title, which
	// lands at the same package path.
	local := newTestShelf(t, &ShelfConf{LibRoot: fixture.libRoot, LockMode: "none", BookCacheWriterID: testWriterID})
	if err := local.DeleteBook(fixture.ids["Dune"]); err != nil {
		t.Fatalf("DeleteBook: %v", err)
	}
	book, err := local.NewBook(FolderPath{}, "Dune")
	if err != nil {
		t.Fatalf("NewBook: %v", err)
	}
	source, err := book.NewSource(strings.NewReader("another Dune"))
	if err != nil {
		t.Fatalf("NewSource: %v", err)
	}
	if err := book.SetCurrentSource(source.ID()); err != nil {
		t.Fatalf("SetCurrentSource: %v", err)
	}
	reexport(t, local, fixture.libRoot)
	if err := s.refreshBookCacheIfNeeded(true); err != nil {
		t.Fatalf("refresh: %v", err)
	}

	if text, err := readCurrentText(t, s, book.ID()); err != nil || text != "another Dune" {
		t.Errorf("new book at a reused path read %q, %v; want its own text", text, err)
	}
	if state, _ := s.DownloadStatus(book.ID()); state != DownloadNotDownloaded {
		t.Errorf("new book state = %q, want %q", state, DownloadNotDownloaded)
	}
}

func TestRedownloadingAMovedBookDropsItsOldCopy(t *testing.T) {
	fixture := seedOfflineSource(t)
	offlineDir := t.TempDir()
	s, _, _ := openOffline(t, fixture.libRoot, offlineDir, false)
	dune := fixture.ids["Dune"]
	first, err := s.DownloadBook(dune)
	if err != nil {
		t.Fatalf("DownloadBook: %v", err)
	}
	book, _ := s.GetBook(dune)
	oldPath := book.PackagePath()

	local := newTestShelf(t, &ShelfConf{LibRoot: fixture.libRoot, LockMode: "none", BookCacheWriterID: testWriterID})
	if _, err := local.MoveBook(dune, FolderPath{"Fiction"}); err != nil {
		t.Fatalf("MoveBook: %v", err)
	}
	reexport(t, local, fixture.libRoot)
	if err := s.refreshBookCacheIfNeeded(true); err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if state, _ := s.DownloadStatus(dune); state != DownloadUpdateAvailable {
		t.Errorf("moved book state = %q, want %q", state, DownloadUpdateAvailable)
	}

	second, err := s.DownloadBook(dune)
	if err != nil {
		t.Fatalf("re-download: %v", err)
	}
	if second.Folder.String() != "Fiction" || second.State != DownloadDownloaded {
		t.Errorf("re-download = %+v (first %+v)", second, first)
	}
	if _, err := os.Stat(filepath.Join(offlineDir, oldPath)); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("copy at the old path after re-downloading: %v, want removed", err)
	}
}

func TestRescanReadsTheSourceAndSavesTheSnapshot(t *testing.T) {
	fixture := seedOfflineSource(t)
	offlineDir := t.TempDir()
	s, _, _ := openOffline(t, fixture.libRoot, offlineDir, false)
	dune := fixture.ids["Dune"]
	if _, err := s.DownloadBook(dune); err != nil {
		t.Fatalf("DownloadBook: %v", err)
	}

	// Renamed on the source, with no new export: only a walk can see it.
	local := newTestShelf(t, &ShelfConf{LibRoot: fixture.libRoot, LockMode: "none"})
	book, err := local.GetBook(dune)
	if err != nil {
		t.Fatalf("GetBook: %v", err)
	}
	meta := book.GetMeta()
	meta.Title = "Dune Messiah"
	if err := book.SetMeta(meta); err != nil {
		t.Fatalf("SetMeta: %v", err)
	}

	if _, err := s.RescanUnthrottled(); err != nil {
		t.Fatalf("Rescan: %v", err)
	}
	if got, want := listedTitles(t, s), []string{"Dune Messiah", "Solaris"}; !slices.Equal(got, want) {
		t.Errorf("listing after a rescan = %v, want the source's %v", got, want)
	}
	if state, _ := s.DownloadStatus(dune); state != DownloadUpdateAvailable {
		t.Errorf("state after a rescan saw the rename = %q, want %q", state, DownloadUpdateAvailable)
	}
	if text, _ := readCurrentText(t, s, dune); text != "Dune text" {
		t.Errorf("downloaded book read %q after the rescan, want its copy", text)
	}

	data, err := os.ReadFile(filepath.Join(offlineDir, offlineSnapshotFile))
	if err != nil || !strings.Contains(string(data), "Dune Messiah") {
		t.Errorf("snapshot after a rescan does not hold the rescanned listing (%v)", err)
	}
}

func TestRescanWhileOfflineKeepsTheListing(t *testing.T) {
	fixture := seedOfflineSource(t)
	s, source, _ := openOffline(t, fixture.libRoot, t.TempDir(), false)

	source.down.Store(true)
	if _, err := s.RescanUnthrottled(); !errors.Is(err, fsutil.ErrSourceUnavailable) {
		t.Errorf("rescan offline error = %v, want %v", err, fsutil.ErrSourceUnavailable)
	}
	if got, want := listedTitles(t, s), []string{"Dune", "Solaris"}; !slices.Equal(got, want) {
		t.Errorf("listing after an offline rescan = %v, want %v", got, want)
	}
}
