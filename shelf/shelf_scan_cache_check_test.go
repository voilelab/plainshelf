package shelf

import (
	"os"
	"path"
	"testing"
	"time"
)

// These cases stand in for the mount the diagnosis exists for: a cloud storage
// gateway that leaves a directory's modification time alone when a child is
// added to it. On such a mount the scan cache reuses the directory's snapshot
// for ever, so a book copied in from outside is not merely late - no walk this
// build performs will ever list the directory again, and the only way a user
// could learn why is to be told.
//
// freezeDirTime replays that: change the directory, then put its modification
// time back where it was.
func freezeDirTime(t *testing.T, dir string, change func()) {
	t.Helper()

	before, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("Stat(%q): %v", dir, err)
	}

	change()

	if err := os.Chtimes(dir, before.ModTime(), before.ModTime()); err != nil {
		t.Fatalf("Chtimes(%q): %v", dir, err)
	}
}

// warmScanCache runs the ordinary walk twice, so the snapshot is populated and
// the shelf is in the steady state a real one is in almost all the time.
func warmScanCache(t *testing.T, s *Shelf, libRoot string) {
	t.Helper()

	ageShelfDirs(t, libRoot, time.Minute)
	mustScan(t, s)
	mustScan(t, s)
}

// The acceptance case: the user presses "update book list" because their book
// is not showing up, and the rescan tells them the cache and the directory
// disagree, naming the directory.
func TestRescanReportsAScanCacheThatMissedANewBook(t *testing.T) {
	libRoot := path.Join(t.TempDir(), "shelf_test")
	s := newTestShelf(t, &ShelfConf{LibRoot: libRoot, LockMode: "none", ScanInterval: "24h"})

	if _, err := s.NewBook(FolderPath{"Fiction"}, "Already Here"); err != nil {
		t.Fatalf("NewBook: %v", err)
	}
	warmScanCache(t, s, libRoot)

	fictionDir := path.Join(libRoot, booksFolder, "Fiction")
	freezeDirTime(t, fictionDir, func() {
		writeBookOnDisk(t, libRoot, path.Join("Fiction", "dropped-in"), "drop1nkb", "Dropped In")
	})

	// The symptom itself: a plain rescan cannot find the book, because the
	// directory it landed in still claims to be unchanged.
	plain, err := s.Rescan(RescanOptions{})
	if err != nil {
		t.Fatalf("Rescan: %v", err)
	}
	if plain.BookCount != 1 {
		t.Fatalf("BookCount = %d before the check, want 1: the fault this test needs is not reproduced", plain.BookCount)
	}
	if len(plain.ScanCacheMismatches) != 0 {
		t.Errorf("a rescan that was not asked to check reported %v", plain.ScanCacheMismatches)
	}

	result, err := s.Rescan(RescanOptions{CheckScanCache: true})
	if err != nil {
		t.Fatalf("Rescan: %v", err)
	}

	if len(result.ScanCacheMismatches) != 1 {
		t.Fatalf("ScanCacheMismatches = %+v, want the one directory the book landed in", result.ScanCacheMismatches)
	}
	mismatch := result.ScanCacheMismatches[0]
	if mismatch.Dir != path.Join(booksFolder, "Fiction") {
		t.Errorf("mismatch dir = %q, want %q", mismatch.Dir, path.Join(booksFolder, "Fiction"))
	}
	if len(mismatch.Missing) != 1 || mismatch.Missing[0] != "dropped-in.bookpkg" {
		t.Errorf("missing = %v, want the book package the snapshot does not list", mismatch.Missing)
	}
}

// A shelf whose mount behaves reports nothing, whether or not it changed since
// the last walk. A diagnosis that fires on a healthy shelf would send users to
// turn off the setting that is doing its job.
func TestRescanReportsNothingWhenTheScanCacheAgrees(t *testing.T) {
	libRoot := path.Join(t.TempDir(), "shelf_test")
	s := newTestShelf(t, &ShelfConf{LibRoot: libRoot, LockMode: "none", ScanInterval: "24h"})

	if _, err := s.NewBook(FolderPath{"Fiction"}, "Already Here"); err != nil {
		t.Fatalf("NewBook: %v", err)
	}
	warmScanCache(t, s, libRoot)

	if result, err := s.Rescan(RescanOptions{CheckScanCache: true}); err != nil {
		t.Fatalf("Rescan: %v", err)
	} else if len(result.ScanCacheMismatches) != 0 {
		t.Errorf("unchanged shelf reported %+v, want nothing", result.ScanCacheMismatches)
	}

	// A book added the way any working filesystem reports it: the directory's
	// modification time moves, the walk relists, nothing is wrong.
	writeBookOnDisk(t, libRoot, path.Join("Fiction", "properly-added"), "prop1nkb", "Properly Added")

	result, err := s.Rescan(RescanOptions{CheckScanCache: true})
	if err != nil {
		t.Fatalf("Rescan: %v", err)
	}
	if len(result.ScanCacheMismatches) != 0 {
		t.Errorf("shelf on a working mount reported %+v, want nothing", result.ScanCacheMismatches)
	}
	if result.BookCount != 2 {
		t.Errorf("BookCount = %d, want 2", result.BookCount)
	}
}

// With scan_cache off there is no snapshot to disagree with, so the check has
// nothing to compare and must stay silent rather than blame the setting that is
// already off.
func TestRescanReportsNothingWithTheScanCacheOff(t *testing.T) {
	libRoot := path.Join(t.TempDir(), "shelf_test")
	scanCacheOff := false
	s := newTestShelf(t, &ShelfConf{LibRoot: libRoot, LockMode: "none", ScanInterval: "24h", ScanCache: &scanCacheOff})

	if _, err := s.NewBook(FolderPath{"Fiction"}, "Already Here"); err != nil {
		t.Fatalf("NewBook: %v", err)
	}
	warmScanCache(t, s, libRoot)

	fictionDir := path.Join(libRoot, booksFolder, "Fiction")
	freezeDirTime(t, fictionDir, func() {
		writeBookOnDisk(t, libRoot, path.Join("Fiction", "dropped-in"), "drop1nkb", "Dropped In")
	})

	result, err := s.Rescan(RescanOptions{CheckScanCache: true})
	if err != nil {
		t.Fatalf("Rescan: %v", err)
	}
	if len(result.ScanCacheMismatches) != 0 {
		t.Errorf("shelf with scan_cache off reported %+v, want nothing", result.ScanCacheMismatches)
	}
	// And the book is found regardless, because every directory is listed.
	if result.BookCount != 2 {
		t.Errorf("BookCount = %d, want 2", result.BookCount)
	}
}

// The cost the ticket is careful about: the check must not reach the refresh
// behind an ordinary listing. A walk that is not the user's rescan still reuses
// every unchanged directory.
func TestOrdinaryScanStillReusesDirectories(t *testing.T) {
	libRoot := path.Join(t.TempDir(), "shelf_test")
	s := newTestShelf(t, &ShelfConf{LibRoot: libRoot, LockMode: "none", ScanInterval: "24h"})

	for _, folder := range []FolderPath{{"Fiction"}, {"Fiction", "Classics"}, {"Tech"}} {
		if _, err := s.NewBook(folder, "Book in "+folder.String()); err != nil {
			t.Fatalf("NewBook %v: %v", folder, err)
		}
	}
	warmScanCache(t, s, libRoot)

	if _, err := s.Rescan(RescanOptions{}); err != nil {
		t.Fatalf("Rescan: %v", err)
	}
	stats := s.lastScanStats()
	if stats.ReadDirs != 0 {
		t.Errorf("a rescan without the check listed %d directories, want 0", stats.ReadDirs)
	}
	if stats.CheckedDirs != 0 {
		t.Errorf("a rescan without the check verified %d directories, want 0", stats.CheckedDirs)
	}

	if _, err := s.Rescan(RescanOptions{CheckScanCache: true}); err != nil {
		t.Fatalf("Rescan: %v", err)
	}
	checked := s.lastScanStats()
	if checked.CheckedDirs != checked.Dirs {
		t.Errorf("the check verified %d of %d directories, want all", checked.CheckedDirs, checked.Dirs)
	}
}
