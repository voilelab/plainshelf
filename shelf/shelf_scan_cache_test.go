package shelf

import (
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"testing"
	"time"
)

// ageShelfDirs pushes every directory under books/ back in time.
//
// The walk deliberately refuses to remember a directory that was modified
// within fsutil.RacyWindow, so a shelf that was just built by the test is
// entirely unrememberable. Ageing it is how a test reaches the steady state a
// real shelf is in almost all the time, without sleeping for it.
func ageShelfDirs(t *testing.T, libRoot string, age time.Duration) {
	t.Helper()

	at := time.Now().Add(-age)
	root := filepath.Join(libRoot, booksFolder)
	err := filepath.WalkDir(root, func(pth string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			return nil
		}
		return os.Chtimes(pth, at, at)
	})
	if err != nil {
		t.Fatalf("age shelf dirs: %v", err)
	}
}

// openShelf builds a shelf the test closes itself, for the cases that check
// what a shutdown writes.
func openShelf(t *testing.T, conf *ShelfConf) *Shelf {
	t.Helper()

	s, err := NewShelf(conf)
	if err != nil {
		t.Fatalf("NewShelf: %v", err)
	}
	if err := s.WaitReady(t.Context()); err != nil {
		t.Fatalf("WaitReady: %v", err)
	}
	return s
}

func mustScan(t *testing.T, s *Shelf) scanStats {
	t.Helper()

	if err := s.scanToBookCache(); err != nil {
		t.Fatalf("scanToBookCache: %v", err)
	}
	return s.lastScanStats()
}

// cachedFolderNames reads the folder list the last scan left in the cache,
// without the refresh that GetAllFolders would schedule.
func cachedFolderNames(s *Shelf) []string {
	return folderNames(s.listFoldersFromCache())
}

// The point of the whole feature: a second walk of an unchanged tree lists no
// directory at all, and still sees exactly the same shelf.
func TestScanCacheReusesUnchangedDirectories(t *testing.T) {
	tmpLib := path.Join(t.TempDir(), "shelf_test")
	s := newTestShelf(t, &ShelfConf{LibRoot: tmpLib})

	for _, folder := range []FolderPath{{"Fiction"}, {"Fiction", "Classics"}, {"Tech"}} {
		if _, err := s.NewBook(folder, "Book in "+folder.String()); err != nil {
			t.Fatalf("NewBook %v: %v", folder, err)
		}
	}
	ageShelfDirs(t, tmpLib, time.Minute)

	cold := mustScan(t, s)
	if cold.ReadDirs != cold.Dirs {
		t.Fatalf("cold scan listed %d of %d directories, want all", cold.ReadDirs, cold.Dirs)
	}

	warm := mustScan(t, s)
	if warm.Dirs != cold.Dirs {
		t.Fatalf("warm scan walked %d directories, want %d", warm.Dirs, cold.Dirs)
	}
	if warm.ReadDirs != 0 {
		t.Errorf("warm scan listed %d directories, want 0", warm.ReadDirs)
	}
	if warm.ReusedDirs != warm.Dirs {
		t.Errorf("warm scan reused %d of %d directories, want all", warm.ReusedDirs, warm.Dirs)
	}

	if got, want := len(s.listBooksFromCache()), 3; got != want {
		t.Errorf("warm scan found %d books, want %d", got, want)
	}
	for _, want := range []string{"", "Fiction", "Fiction/Classics", "Tech"} {
		if !slices.Contains(cachedFolderNames(s), want) {
			t.Errorf("warm scan lost layer %q, got %v", want, cachedFolderNames(s))
		}
	}
}

// A new folder directory created outside PlainShelf is what the full scan exists
// for; the cache must not swallow it.
func TestScanCacheFindsFolderAddedAfterWarmScan(t *testing.T) {
	tmpLib := path.Join(t.TempDir(), "shelf_test")
	s := newTestShelf(t, &ShelfConf{LibRoot: tmpLib})

	if _, err := s.NewBook(FolderPath{"Fiction"}, "Alpha"); err != nil {
		t.Fatalf("NewBook: %v", err)
	}
	ageShelfDirs(t, tmpLib, time.Minute)
	mustScan(t, s)
	mustScan(t, s)

	if err := os.Mkdir(path.Join(tmpLib, booksFolder, "Fiction", "Poetry"), 0755); err != nil {
		t.Fatalf("mkdir layer: %v", err)
	}

	mustScan(t, s)
	if !slices.Contains(cachedFolderNames(s), "Fiction/Poetry") {
		t.Errorf("scan did not find the externally created layer, got %v", cachedFolderNames(s))
	}
}

// The snapshot is what makes the first walk of a process cheap, which is the
// walk a user actually waits for.
func TestScanCacheSurvivesReopen(t *testing.T) {
	tmpLib := path.Join(t.TempDir(), "shelf_test")
	conf := &ShelfConf{LibRoot: tmpLib}

	first := openShelf(t, conf)
	if _, err := first.NewBook(FolderPath{"Fiction", "Classics"}, "Alpha"); err != nil {
		t.Fatalf("NewBook: %v", err)
	}
	ageShelfDirs(t, tmpLib, time.Minute)
	mustScan(t, first)
	if err := first.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	snapshotPath := path.Join(tmpLib, appFolder, scanCacheFileName)
	if _, err := os.Stat(snapshotPath); err != nil {
		t.Fatalf("stat %s: %v", snapshotPath, err)
	}

	second := newTestShelf(t, conf)
	stats := second.lastScanStats()
	if stats.ReusedDirs == 0 {
		t.Errorf("the first scan after reopening reused no directory (%+v)", stats)
	}
	if got, want := len(second.listBooksFromCache()), 1; got != want {
		t.Errorf("reopened shelf listed %d books, want %d", got, want)
	}
}

// The snapshot is disposable by definition, so anything unreadable in its place
// is a cache miss and never an error.
func TestScanCacheIgnoresUnreadableSnapshot(t *testing.T) {
	tmpLib := path.Join(t.TempDir(), "shelf_test")
	conf := &ShelfConf{LibRoot: tmpLib}

	first := openShelf(t, conf)
	if _, err := first.NewBook(FolderPath{"Fiction"}, "Alpha"); err != nil {
		t.Fatalf("NewBook: %v", err)
	}
	if err := first.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	snapshotPath := path.Join(tmpLib, appFolder, scanCacheFileName)
	if err := os.WriteFile(snapshotPath, []byte("{not json"), 0644); err != nil {
		t.Fatalf("write snapshot: %v", err)
	}

	second := newTestShelf(t, conf)
	if got, want := len(second.listBooksFromCache()), 1; got != want {
		t.Errorf("shelf with a corrupt snapshot listed %d books, want %d", got, want)
	}
	if stats := second.lastScanStats(); stats.ReadDirs == 0 {
		t.Errorf("shelf with a corrupt snapshot listed no directory (%+v)", stats)
	}
}

// The escape hatch for a mount whose directory mtimes cannot be trusted.
func TestScanCacheOffListsEveryDirectory(t *testing.T) {
	tmpLib := path.Join(t.TempDir(), "shelf_test")
	scanCacheOff := false
	conf := &ShelfConf{LibRoot: tmpLib, ScanCache: &scanCacheOff}

	s := openShelf(t, conf)
	if _, err := s.NewBook(FolderPath{"Fiction"}, "Alpha"); err != nil {
		t.Fatalf("NewBook: %v", err)
	}
	ageShelfDirs(t, tmpLib, time.Minute)

	mustScan(t, s)
	warm := mustScan(t, s)
	if warm.ReusedDirs != 0 {
		t.Errorf("disabled cache reused %d directories, want 0", warm.ReusedDirs)
	}
	if warm.ReadDirs != warm.Dirs {
		t.Errorf("disabled cache listed %d of %d directories, want all", warm.ReadDirs, warm.Dirs)
	}

	if err := s.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if _, err := os.Stat(path.Join(tmpLib, appFolder, scanCacheFileName)); !os.IsNotExist(err) {
		t.Errorf("disabled cache still wrote a snapshot (err=%v)", err)
	}
}

// The measurement behind the ticket: on a directory-heavy shelf, what does the
// second full scan cost compared with the first? Reported rather than asserted
// on wall time, which is not a stable thing to gate a test on.
func TestScanCacheLargeTreeMeasurement(t *testing.T) {
	if testing.Short() {
		t.Skip("builds a few thousand directories")
	}

	const folders = 60
	const perFolder = 50

	tmpLib := path.Join(t.TempDir(), "shelf_test")
	booksDir := path.Join(tmpLib, booksFolder)
	for i := range folders {
		for j := range perFolder {
			dir := path.Join(booksDir, "layer-"+strconv.Itoa(i), "sub-"+strconv.Itoa(j))
			if err := os.MkdirAll(dir, 0755); err != nil {
				t.Fatalf("mkdir: %v", err)
			}
		}
	}

	s := newTestShelf(t, &ShelfConf{LibRoot: tmpLib})
	ageShelfDirs(t, tmpLib, time.Minute)

	cold := mustScan(t, s)
	warm := mustScan(t, s)

	t.Logf("cold scan: %d directories, %d listed, %d reused, %s", cold.Dirs, cold.ReadDirs, cold.ReusedDirs, cold.Duration)
	t.Logf("warm scan: %d directories, %d listed, %d reused, %s", warm.Dirs, warm.ReadDirs, warm.ReusedDirs, warm.Duration)

	if warm.ReadDirs != 0 {
		t.Errorf("warm scan listed %d directories, want 0", warm.ReadDirs)
	}
}

// An unchanged shelf must not rewrite its snapshot: the digest short-circuit is
// the property most easily broken when the cache is extracted, so it is guarded
// end to end here as well as by the unit tests in shelf/scancache. Open a shelf
// that already has a settled snapshot, close it again, and the file's mtime must
// not move.
func TestScanCacheUnchangedShelfIsNotRewritten(t *testing.T) {
	tmpLib := path.Join(t.TempDir(), "shelf_test")
	conf := &ShelfConf{LibRoot: tmpLib}

	first := openShelf(t, conf)
	if _, err := first.NewBook(FolderPath{"Fiction", "Classics"}, "Alpha"); err != nil {
		t.Fatalf("NewBook: %v", err)
	}
	ageShelfDirs(t, tmpLib, time.Minute)
	mustScan(t, first)
	if err := first.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	snapshotPath := path.Join(tmpLib, appFolder, scanCacheFileName)
	before, err := os.Stat(snapshotPath)
	if err != nil {
		t.Fatalf("stat %s: %v", snapshotPath, err)
	}

	// Reopen and close without changing anything: the loaded snapshot's digest
	// matches what the walk reproduces, so neither the initial scan nor Close
	// should touch the file.
	second := openShelf(t, conf)
	if err := second.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	after, err := os.Stat(snapshotPath)
	if err != nil {
		t.Fatalf("stat %s: %v", snapshotPath, err)
	}
	if !after.ModTime().Equal(before.ModTime()) {
		t.Errorf("an unchanged shelf rewrote its snapshot: mtime moved from %s to %s", before.ModTime(), after.ModTime())
	}
}
