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
	return s.scanCache.LastStats()
}

func cachedFolderNames(s *Shelf) []string {
	return folderNames(s.listFoldersFromCache())
}

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

	if got, want := len(s.listBookListingsFromCache()), 3; got != want {
		t.Errorf("warm scan found %d books, want %d", got, want)
	}
	for _, want := range []string{"", "Fiction", "Fiction/Classics", "Tech"} {
		if !slices.Contains(cachedFolderNames(s), want) {
			t.Errorf("warm scan lost layer %q, got %v", want, cachedFolderNames(s))
		}
	}
}

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
	stats := second.scanCache.LastStats()
	if stats.ReusedDirs == 0 {
		t.Errorf("the first scan after reopening reused no directory (%+v)", stats)
	}
	if got, want := len(second.listBookListingsFromCache()), 1; got != want {
		t.Errorf("reopened shelf listed %d books, want %d", got, want)
	}
}

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
	if got, want := len(second.listBookListingsFromCache()), 1; got != want {
		t.Errorf("shelf with a corrupt snapshot listed %d books, want %d", got, want)
	}
	if stats := second.scanCache.LastStats(); stats.ReadDirs == 0 {
		t.Errorf("shelf with a corrupt snapshot listed no directory (%+v)", stats)
	}
}

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

func TestScanCacheWithManyDirectoriesIsNotRewritten(t *testing.T) {
	tmpLib := path.Join(t.TempDir(), "shelf_test")
	conf := &ShelfConf{LibRoot: tmpLib}

	first := openShelf(t, conf)
	for i := range 16 {
		folder := FolderPath{"Shelf" + strconv.Itoa(i)}
		if _, err := first.NewBook(folder, "Book "+strconv.Itoa(i)); err != nil {
			t.Fatalf("NewBook: %v", err)
		}
	}
	ageShelfDirs(t, tmpLib, time.Minute)
	mustScan(t, first)
	if err := first.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	snapshotPath := path.Join(tmpLib, appFolder, scanCacheFileName)
	want, err := os.ReadFile(snapshotPath)
	if err != nil {
		t.Fatalf("read %s: %v", snapshotPath, err)
	}

	for round := range 8 {
		before, err := os.Stat(snapshotPath)
		if err != nil {
			t.Fatalf("stat %s: %v", snapshotPath, err)
		}

		next := openShelf(t, conf)
		mustScan(t, next)
		if err := next.Close(); err != nil {
			t.Fatalf("Close: %v", err)
		}

		after, err := os.Stat(snapshotPath)
		if err != nil {
			t.Fatalf("stat %s: %v", snapshotPath, err)
		}
		if !after.ModTime().Equal(before.ModTime()) {
			t.Fatalf("round %d rewrote the snapshot of an unchanged shelf", round)
		}
		got, err := os.ReadFile(snapshotPath)
		if err != nil {
			t.Fatalf("re-read %s: %v", snapshotPath, err)
		}
		if string(got) != string(want) {
			t.Fatalf("round %d changed the snapshot on disk", round)
		}
	}
}
