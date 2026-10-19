package shelf

import (
	"errors"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/voilelab/plainshelf/internal/fsutil"
)

func mustListFolderMeta(t *testing.T, s *Shelf) map[string]FolderMeta {
	t.Helper()
	metas, err := s.ListFolderMeta()
	if err != nil {
		t.Fatalf("ListFolderMeta: %v", err)
	}
	return metas
}

func listedIcons(t *testing.T, s *Shelf) map[string]string {
	t.Helper()
	icons := map[string]string{}
	for name, meta := range mustListFolderMeta(t, s) {
		icons[name] = meta.Icon
	}
	return icons
}

func assertIcons(t *testing.T, got, want map[string]string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("icons = %v, want %v", got, want)
	}
	for name, icon := range want {
		if got[name] != icon {
			t.Fatalf("icons = %v, want %v", got, want)
		}
	}
}

// A long scan interval keeps every listing on the cache, so whatever a test sees
// change did not come from a full scan it did not ask for.
func newCachedFolderMetaShelf(t *testing.T, conf ShelfConf, folders ...FolderPath) (*Shelf, string) {
	t.Helper()

	conf.LibRoot = filepath.Join(t.TempDir(), "shelf")
	conf.LockMode = "none"
	conf.ScanInterval = "10m"
	conf.BookCheckInterval = "10m"
	s := newTestShelf(t, &conf)
	for _, folder := range folders {
		if err := s.NewFolder(folder[:len(folder)-1], folder[len(folder)-1]); err != nil {
			t.Fatalf("NewFolder(%v): %v", folder, err)
		}
	}
	return s, conf.LibRoot
}

func TestListFolderMetaFollowsWritesAtOnceAndOutsideEditsAfterARescan(t *testing.T) {
	s, libRoot := newCachedFolderMetaShelf(t, ShelfConf{}, FolderPath{"Fiction"}, FolderPath{"Fiction", "Classics"})

	assertIcons(t, listedIcons(t, s), map[string]string{})

	if err := s.SetFolderIcon(FolderPath{"Fiction", "Classics"}, "book", ""); err != nil {
		t.Fatalf("SetFolderIcon: %v", err)
	}
	assertIcons(t, listedIcons(t, s), map[string]string{"Fiction/Classics": "book"})

	// An edit made outside PlainShelf waits for the next full scan; in place, it
	// does not move the folder's mtime, so the scan must read the file anyway.
	dir := filepath.Join(libRoot, booksFolder, "Fiction", "Classics")
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	writeFolderMeta(t, libRoot, `{"icon":"moon"}`, "Fiction", "Classics")
	writeFolderMeta(t, libRoot, `{"icon":"star"}`, "Fiction")
	if err := os.Chtimes(dir, info.ModTime(), info.ModTime()); err != nil {
		t.Fatalf("Chtimes: %v", err)
	}
	assertIcons(t, listedIcons(t, s), map[string]string{"Fiction/Classics": "book"})

	if _, err := s.RescanUnthrottled(); err != nil {
		t.Fatalf("RescanUnthrottled: %v", err)
	}
	assertIcons(t, listedIcons(t, s), map[string]string{"Fiction": "star", "Fiction/Classics": "moon"})

	// Clearing removes the entry with the file.
	meta := mustListFolderMeta(t, s)["Fiction"]
	if err := s.SetFolderIcon(FolderPath{"Fiction"}, "", meta.Version); err != nil {
		t.Fatalf("SetFolderIcon(clear): %v", err)
	}
	assertIcons(t, listedIcons(t, s), map[string]string{"Fiction/Classics": "moon"})
}

// A write refused because the cache was stale must leave the cache fresh, or the
// caller's retry would be handed the same stale version until the next scan.
func TestFolderMetaConflictRefreshesTheCachedVersion(t *testing.T) {
	s, libRoot := newCachedFolderMetaShelf(t, ShelfConf{}, FolderPath{"Fiction"})
	if err := s.SetFolderIcon(FolderPath{"Fiction"}, "book", ""); err != nil {
		t.Fatalf("SetFolderIcon: %v", err)
	}
	stale := mustListFolderMeta(t, s)["Fiction"]

	writeFolderMeta(t, libRoot, `{"icon":"moon","schema_version":1}`, "Fiction")
	if err := s.SetFolderIcon(FolderPath{"Fiction"}, "star", stale.Version); !errors.Is(err, ErrFolderMetaConflict) {
		t.Fatalf("SetFolderIcon with a stale version: %v, want ErrFolderMetaConflict", err)
	}

	fresh := mustListFolderMeta(t, s)["Fiction"]
	if fresh.Icon != "moon" || fresh.Version == stale.Version {
		t.Fatalf("cache after the conflict = %+v, want the file on disk", fresh)
	}
	if err := s.SetFolderIcon(FolderPath{"Fiction"}, "star", fresh.Version); err != nil {
		t.Fatalf("retry with the refreshed version: %v", err)
	}
}

func TestListFolderMetaFollowsFolderChanges(t *testing.T) {
	s, _ := newCachedFolderMetaShelf(t, ShelfConf{}, FolderPath{"Fiction"}, FolderPath{"Empty"}, FolderPath{"Archive"})
	for _, folder := range []FolderPath{{"Fiction"}, {"Empty"}} {
		if err := s.SetFolderIcon(folder, "book", ""); err != nil {
			t.Fatalf("SetFolderIcon(%v): %v", folder, err)
		}
	}

	if err := s.DeleteFolder(FolderPath{"Empty"}); err != nil {
		t.Fatalf("DeleteFolder: %v", err)
	}
	assertIcons(t, listedIcons(t, s), map[string]string{"Fiction": "book"})

	if err := s.RenameFolder(FolderPath{"Fiction"}, "SF"); err != nil {
		t.Fatalf("RenameFolder: %v", err)
	}
	assertIcons(t, listedIcons(t, s), map[string]string{"SF": "book"})

	if err := s.MoveFolder(FolderPath{"SF"}, FolderPath{"Archive"}); err != nil {
		t.Fatalf("MoveFolder: %v", err)
	}
	assertIcons(t, listedIcons(t, s), map[string]string{"Archive/SF": "book"})
}

// folderMetaOpens counts the folder.json files a shelf opens.
type folderMetaOpens struct {
	fsutil.FS
	opens atomic.Int64
}

func (f *folderMetaOpens) Open(name string) (fs.File, error) {
	if path.Base(name) == FolderMetaFile {
		f.opens.Add(1)
	}
	return f.FS.Open(name)
}

// The I/O a full scan adds is one read per folder that has a folder.json, warm
// scan cache or not; a folder without one, and a listing between scans, cost
// nothing.
func TestFolderMetaScanReadsOnlyFoldersThatHaveIt(t *testing.T) {
	s, libRoot := newCachedFolderMetaShelf(t, ShelfConf{})
	const folders, withMeta = 40, 5
	for i := range folders {
		name := "Folder" + string(rune('A'+i%26)) + string(rune('a'+i/26))
		if _, err := s.NewBook(FolderPath{name}, "Book "+name); err != nil {
			t.Fatalf("NewBook: %v", err)
		}
		if i < withMeta {
			writeFolderMeta(t, libRoot, `{"icon":"book"}`, name)
		}
	}
	ageShelfDirs(t, libRoot, time.Minute)

	counter := &folderMetaOpens{FS: writeRootForTest(t, s)}
	s.dbRoot = counter

	cold := mustScan(t, s)
	if got := counter.opens.Swap(0); got != withMeta {
		t.Errorf("cold scan opened %d folder.json files, want %d", got, withMeta)
	}

	warm := mustScan(t, s)
	if warm.ReadDirs != 0 {
		t.Fatalf("warm scan listed %d of %d directories; the test needs a reused snapshot", warm.ReadDirs, warm.Dirs)
	}
	if got := counter.opens.Swap(0); got != withMeta {
		t.Errorf("warm scan opened %d folder.json files, want %d", got, withMeta)
	}
	if got := len(mustListFolderMeta(t, s)); got != withMeta {
		t.Errorf("warm scan found metadata for %d folders, want %d", got, withMeta)
	}
	if got := counter.opens.Load(); got != 0 {
		t.Errorf("a listing between scans opened %d folder.json files, want 0", got)
	}
	t.Logf("%d folders, %d with folder.json: cold scan listed %d dirs, warm scan reused %d; each full scan adds %d reads",
		folders, withMeta, cold.ReadDirs, warm.ReusedDirs, withMeta)
}

func TestBookCacheExportCarriesFolderMeta(t *testing.T) {
	s, libRoot := newCachedFolderMetaShelf(t, ShelfConf{BookCacheWriterID: testWriterID}, FolderPath{"Fiction"}, FolderPath{"Plain"})

	if _, err := s.ExportBookCache(); err != nil {
		t.Fatalf("ExportBookCache: %v", err)
	}
	exportPath := filepath.Join(libRoot, appFolder, bookCacheFilePrefix+testWriterID+bookCacheFileSuffix)
	raw, err := os.ReadFile(exportPath)
	if err != nil {
		t.Fatalf("read export: %v", err)
	}
	if strings.Contains(string(raw), "folder_meta") {
		t.Fatalf("a shelf without folder.json exported a folder_meta member:\n%s", raw)
	}

	if err := s.SetFolderIcon(FolderPath{"Fiction"}, "rocket", ""); err != nil {
		t.Fatalf("SetFolderIcon: %v", err)
	}
	writeFolderMeta(t, libRoot, `{"icon":`, "Plain")
	if _, err := s.ExportBookCache(); err != nil {
		t.Fatalf("ExportBookCache: %v", err)
	}
	cache, err := readBookCacheFileAt(exportPath)
	if err != nil {
		t.Fatalf("read export: %v", err)
	}
	want := map[string]BookCacheFolderMeta{
		"Fiction": {Icon: "rocket"},
		"Plain":   {ReadOnly: true},
	}
	if len(cache.FolderMeta) != len(want) || cache.FolderMeta["Fiction"] != want["Fiction"] || cache.FolderMeta["Plain"] != want["Plain"] {
		t.Fatalf("exported folder_meta = %v, want %v", cache.FolderMeta, want)
	}
}

// A remote shelf takes folder metadata from the export, so a folder with an icon
// costs the reader no request of its own.
func TestRemoteShelfListsFolderMetaFromTheExport(t *testing.T) {
	libRoot := seedExportedShelf(t, "One", "Two")
	writeFolderMeta(t, libRoot, `{"icon":"rocket"}`, "Fiction")

	// Re-export from a writer that now sees the file.
	writer := newTestShelf(t, &ShelfConf{LibRoot: libRoot, LockMode: "none", BookCacheWriterID: testWriterID})
	if _, err := writer.ExportBookCache(); err != nil {
		t.Fatalf("ExportBookCache: %v", err)
	}

	s, counter := openRemoteShelf(t, libRoot, ShelfConf{})
	assertIcons(t, listedIcons(t, s), map[string]string{"Fiction": "rocket"})
	if reached := booksCalls(counter.take()); len(reached) > 0 {
		t.Errorf("listing folder metadata reached into books/: %v", reached)
	}
	if err := s.SetFolderIcon(FolderPath{"Fiction"}, "star", ""); !errors.Is(err, fsutil.ErrReadOnly) {
		t.Errorf("SetFolderIcon on a remote shelf: %v, want ErrReadOnly", err)
	}
}

// An export from a build that predates folder_meta, or one naming folders it
// does not list, still lists the shelf; only the unusable metadata is dropped.
func TestRemoteShelfReadsAnExportWithoutUsableFolderMeta(t *testing.T) {
	libRoot := seedExportedShelf(t, "One", "Two")
	exportPath := filepath.Join(libRoot, appFolder, bookCacheFilePrefix+testWriterID+bookCacheFileSuffix)
	cache, err := readBookCacheFileAt(exportPath)
	if err != nil {
		t.Fatalf("read export: %v", err)
	}
	cache.FolderMeta = map[string]BookCacheFolderMeta{
		"/":       {Icon: "star"},
		"Missing": {Icon: "star"},
		"../Out":  {Icon: "star"},
		"Fiction": {Icon: "icon-from-a-newer-build"},
	}
	writeRawBookCache(t, libRoot, bookCacheFilePrefix+testWriterID+bookCacheFileSuffix, cache)

	s, _ := openRemoteShelf(t, libRoot, ShelfConf{})
	if got := listedTitles(t, s); len(got) != 2 {
		t.Fatalf("listed %v, want both books", got)
	}
	metas := mustListFolderMeta(t, s)
	if len(metas) != 1 || metas["Fiction"].Icon != "icon-from-a-newer-build" || metas["Fiction"].DisplayIcon() != "" {
		t.Fatalf("folder meta = %+v, want only Fiction, shown as the default", metas)
	}

	cache.FolderMeta = nil
	writeRawBookCache(t, libRoot, bookCacheFilePrefix+testWriterID+bookCacheFileSuffix, cache)
	s, _ = openRemoteShelf(t, libRoot, ShelfConf{})
	if metas := mustListFolderMeta(t, s); len(metas) != 0 {
		t.Fatalf("an export without folder_meta listed %v", metas)
	}
}

// Legacy exports must not pin the default icon once a metadata-capable writer
// publishes, including a re-export within the same timestamp second.
func TestRemoteFolderMetaRefreshesLegacyAndSameSecondExports(t *testing.T) {
	libRoot := seedExportedShelf(t, "One", "Two")
	name := bookCacheFilePrefix + testWriterID + bookCacheFileSuffix
	cache, err := readBookCacheFileAt(filepath.Join(libRoot, appFolder, name))
	if err != nil {
		t.Fatal(err)
	}
	s, counter := openRemoteShelf(t, libRoot, ShelfConf{})
	assertIcons(t, listedIcons(t, s), map[string]string{})
	counter.take()

	for _, icon := range []string{"rocket", "moon", ""} {
		cache.FolderMeta = map[string]BookCacheFolderMeta{"Fiction": {Icon: icon}}
		writeRawBookCache(t, libRoot, name, cache)
		if err := s.refreshRemoteBookCache(nil); err != nil {
			t.Fatal(err)
		}
		want := map[string]string{}
		if icon != "" {
			want["Fiction"] = icon
		}
		assertIcons(t, listedIcons(t, s), want)
	}
	if calls := booksCalls(counter.take()); len(calls) != 0 {
		t.Fatalf("metadata refresh accessed books/: %v", calls)
	}
}

func TestFolderMetaSurvivesOfflineSnapshotAndReconnect(t *testing.T) {
	libRoot := seedExportedShelf(t, "One", "Two")
	name := bookCacheFilePrefix + testWriterID + bookCacheFileSuffix
	cache, err := readBookCacheFileAt(filepath.Join(libRoot, appFolder, name))
	if err != nil {
		t.Fatal(err)
	}
	cache.FolderMeta = map[string]BookCacheFolderMeta{"Fiction": {Icon: "rocket"}}
	writeRawBookCache(t, libRoot, name, cache)
	offlineDir := t.TempDir()
	online, _, counter := openOffline(t, libRoot, offlineDir, false)
	assertIcons(t, listedIcons(t, online), map[string]string{"Fiction": "rocket"})
	if calls := booksCalls(counter.take()); len(calls) != 0 {
		t.Fatalf("opening from export accessed books/: %v", calls)
	}
	if err := online.Close(); err != nil {
		t.Fatal(err)
	}

	s, source, counter := openOffline(t, libRoot, offlineDir, true)
	assertIcons(t, listedIcons(t, s), map[string]string{"Fiction": "rocket"})
	if err := s.refreshRemoteBookCache(nil); err != nil {
		t.Fatal(err)
	}
	assertIcons(t, listedIcons(t, s), map[string]string{"Fiction": "rocket"})

	// Reconnect to a new export whose timestamp is unchanged. The offline
	// snapshot's digest must not suppress metadata fetched from the source.
	cache.FolderMeta["Fiction"] = BookCacheFolderMeta{Icon: "moon"}
	writeRawBookCache(t, libRoot, name, cache)
	source.down.Store(false)
	if err := s.refreshRemoteBookCache(nil); err != nil {
		t.Fatal(err)
	}
	assertIcons(t, listedIcons(t, s), map[string]string{"Fiction": "moon"})
	if calls := booksCalls(counter.take()); len(calls) != 0 {
		t.Fatalf("reconnecting accessed books/: %v", calls)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, _, _ := openOffline(t, libRoot, offlineDir, true)
	assertIcons(t, listedIcons(t, reopened), map[string]string{"Fiction": "moon"})
}

func TestFolderMetaOnlyChangesReexportButUnchangedMetadataDoesNot(t *testing.T) {
	s, libRoot := newCachedFolderMetaShelf(t, ShelfConf{BookCacheWriterID: testWriterID})
	for _, icon := range FolderIcons() {
		if err := s.NewFolder(nil, icon); err != nil {
			t.Fatal(err)
		}
		if err := s.SetFolderIcon(FolderPath{icon}, icon, ""); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.ExportBookCache(); err != nil {
		t.Fatal(err)
	}
	filePath := filepath.Join(libRoot, appFolder, s.bookCacheFileName())
	for range 8 {
		shiftModTime(t, filePath, -time.Hour)
		before, err := os.Stat(filePath)
		if err != nil {
			t.Fatal(err)
		}
		if err := s.exportBookCache(false); err != nil {
			t.Fatal(err)
		}
		after, err := os.Stat(filePath)
		if err != nil || !after.ModTime().Equal(before.ModTime()) {
			t.Fatalf("unchanged folder metadata rewrote the export: %v", err)
		}
	}
	meta := mustListFolderMeta(t, s)["book"]
	if err := s.SetFolderIcon(FolderPath{"book"}, "moon", meta.Version); err != nil {
		t.Fatal(err)
	}
	if err := s.exportBookCache(false); err != nil {
		t.Fatal(err)
	}
	cache, err := readBookCacheFileAt(filePath)
	if err != nil || cache.FolderMeta["book"].Icon != "moon" {
		t.Fatalf("metadata-only edit was not exported: %v, %v", cache, err)
	}
}

// Each simulated scan publishes a complete generation under the cache lock.
// An export must take its timestamp, paths, metadata and books from that same
// generation even while the next scan is being published.
func TestExportSnapshotKeepsOneScanGeneration(t *testing.T) {
	s, _ := newCachedFolderMetaShelf(t, ShelfConf{})
	book, err := s.NewBook(FolderPath{"First"}, "One")
	if err != nil {
		t.Fatal(err)
	}
	publish := func(generation int64) {
		name, icon := "First", "book"
		if generation == 2 {
			name, icon = "Second", "moon"
		}
		s.bookCache.Lock()
		s.bookCache.folders = []FolderPath{{}, {name}}
		s.bookCache.folderMeta = map[string]FolderMeta{name: {Icon: icon}}
		s.bookCache.cache = map[string]*bookIDCacheEntry{book.ID(): {
			folders: FolderPath{name}, path: "books/" + name + "/one.bookpkg", book: book,
		}}
		s.bookCache.lastScanStart = time.Unix(generation, 0)
		s.bookCache.Unlock()
	}
	publish(1)
	stop, done := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(done)
		for {
			select {
			case <-stop:
				return
			default:
				publish(1)
				publish(2)
			}
		}
	}()
	defer func() { close(stop); <-done }()
	for range 2000 {
		cache := s.collectExportSnapshot(testWriterID)
		name, icon := "First", "book"
		if cache.Timestamp == 2 {
			name, icon = "Second", "moon"
		}
		if len(cache.Folders) != 2 || cache.Folders[1] != name || len(cache.FolderMeta) != 1 ||
			cache.FolderMeta[name].Icon != icon || cache.Books[book.ID()].Path != "books/"+name+"/one.bookpkg" {
			t.Fatalf("export mixed scan generations: %+v", cache)
		}
	}
}

func TestFolderMetaRemoteListingIO(t *testing.T) {
	s, libRoot := newCachedFolderMetaShelf(t, ShelfConf{BookCacheWriterID: testWriterID})
	for i := range 40 {
		name := "Folder" + string(rune('A'+i%26)) + string(rune('a'+i/26))
		if err := s.NewFolder(nil, name); err != nil {
			t.Fatal(err)
		}
		if i < 5 {
			if err := s.SetFolderIcon(FolderPath{name}, "book", ""); err != nil {
				t.Fatal(err)
			}
		}
	}
	if _, err := s.ExportBookCache(); err != nil {
		t.Fatal(err)
	}
	remote, counter := openRemoteShelf(t, libRoot, ShelfConf{ScanInterval: "10m", BookCheckInterval: "10m"})
	cold := counter.take()
	assertIcons(t, listedIcons(t, remote), map[string]string{
		"FolderAa": "book", "FolderBa": "book", "FolderCa": "book", "FolderDa": "book", "FolderEa": "book",
	})
	for range 5 {
		if _, err := remote.GetAllFolders(); err != nil {
			t.Fatal(err)
		}
		mustListFolderMeta(t, remote)
	}
	warm := counter.take()
	if len(warm) != 0 {
		t.Fatalf("warm listings made remote calls: %v", warm)
	}
	if err := remote.refreshRemoteBookCache(nil); err != nil {
		t.Fatal(err)
	}
	refresh := counter.take()
	if calls := booksCalls(append(cold, refresh...)); len(calls) != 0 {
		t.Fatalf("export-backed listings accessed books/: %v", calls)
	}
	t.Logf("40 folders, 5 with metadata: cold=%v; warm=%v; export refresh=%v", cold, warm, refresh)
}
