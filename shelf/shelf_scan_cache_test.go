package shelf

import (
	"context"
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
	if err := s.WaitReady(context.Background()); err != nil {
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

// cachedLayerNames reads the layer list the last scan left in the cache,
// without the refresh that GetAllLayers would schedule.
func cachedLayerNames(s *Shelf) []string {
	return layerNames(s.listLayersFromCache())
}

// The point of the whole feature: a second walk of an unchanged tree lists no
// directory at all, and still sees exactly the same shelf.
func TestScanCacheReusesUnchangedDirectories(t *testing.T) {
	tmpLib := path.Join(t.TempDir(), "shelf_test")
	s := newTestShelf(t, &ShelfConf{LibRoot: tmpLib})

	for _, layer := range []Layers{{"Fiction"}, {"Fiction", "Classics"}, {"Tech"}} {
		if _, err := s.NewBook(layer, "Book in "+layer.String()); err != nil {
			t.Fatalf("NewBook %v: %v", layer, err)
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
		if !slices.Contains(cachedLayerNames(s), want) {
			t.Errorf("warm scan lost layer %q, got %v", want, cachedLayerNames(s))
		}
	}
}

// A directory's mtime only reports on its direct children, so the cache has to
// be consulted per directory rather than per subtree: a book added deep in the
// tree must still be found while every directory above it is reused.
func TestScanCacheFindsBookAddedUnderReusedParent(t *testing.T) {
	tmpLib := path.Join(t.TempDir(), "shelf_test")
	s := newTestShelf(t, &ShelfConf{LibRoot: tmpLib})

	if _, err := s.NewBook(Layers{"Fiction", "Classics"}, "Alpha"); err != nil {
		t.Fatalf("NewBook: %v", err)
	}
	ageShelfDirs(t, tmpLib, time.Minute)
	mustScan(t, s)

	// Copy an existing book package in from outside, the way a file manager or
	// a sync client would.
	src := path.Join(tmpLib, s.listBooksFromCache()[0].FolderPath())
	dst := path.Join(tmpLib, booksFolder, "Fiction", "Classics", "beta.bookpkg")
	if err := os.CopyFS(dst, os.DirFS(src)); err != nil {
		t.Fatalf("copy book package: %v", err)
	}

	warm := mustScan(t, s)
	if warm.ReadDirs == 0 {
		t.Errorf("scan listed no directory, but Fiction/Classics changed")
	}
	if warm.ReusedDirs == 0 {
		t.Errorf("scan reused no directory, but books/ and books/Fiction did not change")
	}

	// The copy carries the same book ID, so the listing cannot count it; what
	// the walk has to have noticed is the directory itself.
	found := false
	if _, err := s.iterateShelfTree(nil, func(b *Book) bool {
		if b.FolderPath() == path.Join(booksFolder, "Fiction", "Classics", "beta.bookpkg") {
			found = true
		}
		return true
	}); err != nil {
		t.Fatalf("iterateShelfTree: %v", err)
	}
	if !found {
		t.Errorf("the walk did not find the book package added under a reused parent")
	}
}

// A new layer directory created outside PlainShelf is what the full scan exists
// for; the cache must not swallow it.
func TestScanCacheFindsLayerAddedAfterWarmScan(t *testing.T) {
	tmpLib := path.Join(t.TempDir(), "shelf_test")
	s := newTestShelf(t, &ShelfConf{LibRoot: tmpLib})

	if _, err := s.NewBook(Layers{"Fiction"}, "Alpha"); err != nil {
		t.Fatalf("NewBook: %v", err)
	}
	ageShelfDirs(t, tmpLib, time.Minute)
	mustScan(t, s)
	mustScan(t, s)

	if err := os.Mkdir(path.Join(tmpLib, booksFolder, "Fiction", "Poetry"), 0755); err != nil {
		t.Fatalf("mkdir layer: %v", err)
	}

	mustScan(t, s)
	if !slices.Contains(cachedLayerNames(s), "Fiction/Poetry") {
		t.Errorf("scan did not find the externally created layer, got %v", cachedLayerNames(s))
	}
}

// Timestamps are coarse. A directory modified in the same tick the walk read it
// would keep that mtime, so remembering it would hide every later change to it
// forever. Such a directory must simply not enter the snapshot.
func TestScanCacheRefusesRacyDirectories(t *testing.T) {
	tmpLib := path.Join(t.TempDir(), "shelf_test")
	s := newTestShelf(t, &ShelfConf{LibRoot: tmpLib})

	if _, err := s.NewBook(Layers{"Fiction"}, "Alpha"); err != nil {
		t.Fatalf("NewBook: %v", err)
	}

	// No ageing here: every directory was modified moments ago.
	mustScan(t, s)
	warm := mustScan(t, s)
	if warm.ReusedDirs != 0 {
		t.Errorf("scan reused %d directories modified inside the racy window, want 0", warm.ReusedDirs)
	}

	// And the change that race would have hidden is still found.
	if err := os.Mkdir(path.Join(tmpLib, booksFolder, "Fiction", "Poetry"), 0755); err != nil {
		t.Fatalf("mkdir layer: %v", err)
	}
	mustScan(t, s)
	if !slices.Contains(cachedLayerNames(s), "Fiction/Poetry") {
		t.Errorf("scan did not find the layer added after a racy scan, got %v", cachedLayerNames(s))
	}
}

// The snapshot is what makes the first walk of a process cheap, which is the
// walk a user actually waits for.
func TestScanCacheSurvivesReopen(t *testing.T) {
	tmpLib := path.Join(t.TempDir(), "shelf_test")
	conf := &ShelfConf{LibRoot: tmpLib}

	first := openShelf(t, conf)
	if _, err := first.NewBook(Layers{"Fiction", "Classics"}, "Alpha"); err != nil {
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
	if _, err := first.NewBook(Layers{"Fiction"}, "Alpha"); err != nil {
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
	if _, err := s.NewBook(Layers{"Fiction"}, "Alpha"); err != nil {
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

	const layers = 60
	const perLayer = 50

	tmpLib := path.Join(t.TempDir(), "shelf_test")
	booksDir := path.Join(tmpLib, booksFolder)
	for i := range layers {
		for j := range perLayer {
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

// A directory's mtime identifies its content, not the directory. If one is
// moved away and another takes its place carrying the same mtime - which
// coarse-timestamp filesystems and timestamp-preserving copies both make
// possible - matching the mtime alone would serve the old directory's children
// forever, and the books under the new one would never appear.
func TestScanCacheDistrustsAReplacedDirectory(t *testing.T) {
	tmpLib := path.Join(t.TempDir(), "shelf_test")
	s := newTestShelf(t, &ShelfConf{LibRoot: tmpLib})

	booksDir := path.Join(tmpLib, booksFolder)
	if err := os.MkdirAll(path.Join(booksDir, "Fiction", "kept"), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	ageShelfDirs(t, tmpLib, time.Minute)
	mustScan(t, s)
	mustScan(t, s)

	info, err := os.Stat(path.Join(booksDir, "Fiction"))
	if err != nil {
		t.Fatalf("stat: %v", err)
	}

	// Move the layer out of the shelf and put a different one in its place,
	// then give it the mtime the scan remembered.
	replaced := path.Join(t.TempDir(), "moved-away")
	if err := os.Rename(path.Join(booksDir, "Fiction"), replaced); err != nil {
		t.Fatalf("move away: %v", err)
	}
	staged := path.Join(t.TempDir(), "Fiction")
	if err := os.MkdirAll(path.Join(staged, "brought-in"), 0755); err != nil {
		t.Fatalf("mkdir staged: %v", err)
	}
	if err := os.Rename(staged, path.Join(booksDir, "Fiction")); err != nil {
		t.Fatalf("move in: %v", err)
	}
	at := info.ModTime()
	if err := os.Chtimes(path.Join(booksDir, "Fiction"), at, at); err != nil {
		t.Fatalf("chtimes: %v", err)
	}

	mustScan(t, s)
	names := cachedLayerNames(s)
	if !slices.Contains(names, "Fiction/brought-in") {
		t.Errorf("scan did not find the replacement directory's layer, got %v", names)
	}
	if slices.Contains(names, "Fiction/kept") {
		t.Errorf("scan still reports the replaced directory's layer, got %v", names)
	}
}
