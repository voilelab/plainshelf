package shelf

import (
	"encoding/json"
	"errors"
	"os"
	"path"
	"testing"
)

// writeBookOnDisk drops a book package into books/ the way an external file
// operation would: PlainShelf never sees the write, so only a walk can find it.
func writeBookOnDisk(t *testing.T, libRoot, dirName, bookID, title string) {
	t.Helper()

	bookDir := path.Join(libRoot, booksFolder, dirName+".bookpkg")
	if err := os.MkdirAll(bookDir, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	data, err := json.Marshal(map[string]any{
		"schema_version": BookMetaSchemaVersion,
		"id":             bookID,
		"title":          title,
	})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if err := os.WriteFile(path.Join(bookDir, BookMetaFile), data, 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}

func TestRescanFindsBooksAddedOnDiskWithinTheScanInterval(t *testing.T) {
	libRoot := t.TempDir()
	s := newTestShelf(t, &ShelfConf{
		LibRoot:  libRoot,
		LockMode: "none",
		// Long enough that nothing but the explicit rescan can walk the tree
		// again, which is the whole point of the endpoint behind it.
		ScanInterval: "24h",
	})

	writeBookOnDisk(t, libRoot, "added-behind-our-back", "onda1skb", "On Disk")

	books, err := s.ListBooks()
	if err != nil {
		t.Fatalf("ListBooks: %v", err)
	}
	if len(books) != 0 {
		t.Fatalf("ListBooks found %d books before the rescan, want 0", len(books))
	}

	result, err := s.Rescan(RescanOptions{})
	if err != nil {
		t.Fatalf("Rescan: %v", err)
	}
	if result.BookCount != 1 {
		t.Errorf("BookCount = %d, want 1", result.BookCount)
	}
	if result.ID == "" {
		t.Error("Rescan returned an empty scan ID")
	}
	if result.StartedAt.IsZero() {
		t.Error("Rescan returned a zero start time")
	}

	books, err = s.ListBooks()
	if err != nil {
		t.Fatalf("ListBooks after rescan: %v", err)
	}
	if len(books) != 1 || books[0].ID() != "onda1skb" {
		t.Fatalf("ListBooks after rescan = %v, want the book added on disk", books)
	}
}

// The folder count describes the shelf, not the books, so a folder nobody has
// filed a book into still counts.
func TestRescanCountsFoldersIncludingEmptyOnes(t *testing.T) {
	s := newTestShelf(t, &ShelfConf{LibRoot: t.TempDir(), LockMode: "none"})

	if err := s.NewFolder(FolderPath{"Fiction"}, "Classics"); err != nil {
		t.Fatalf("NewFolder: %v", err)
	}
	if err := s.NewFolder(FolderPath{}, "Empty"); err != nil {
		t.Fatalf("NewFolder: %v", err)
	}
	if _, err := s.NewBook(FolderPath{"Fiction", "Classics"}, "Dune"); err != nil {
		t.Fatalf("NewBook: %v", err)
	}

	result, err := s.Rescan(RescanOptions{})
	if err != nil {
		t.Fatalf("Rescan: %v", err)
	}
	if result.BookCount != 1 {
		t.Errorf("BookCount = %d, want 1", result.BookCount)
	}
	// "/", "Empty", "Fiction", "Fiction/Classics".
	if result.FolderCount != 4 {
		t.Errorf("LayerCount = %d, want 4", result.FolderCount)
	}
}

// A rescan is a read. Unlike ExportBookCache, which forces the same walk, it
// must leave nothing behind on disk.
func TestRescanWritesNothing(t *testing.T) {
	libRoot := t.TempDir()
	s := newTestShelf(t, &ShelfConf{
		LibRoot:           libRoot,
		LockMode:          "none",
		BookCacheWriterID: testWriterID,
		ScanInterval:      "24h",
		// Long enough that the export cannot come due on its own during the test.
		BookCacheInterval: "24h",
	})
	waitForBookCacheExport(t, libRoot, testWriterID)

	before := readBookCacheFileOrFail(t, libRoot)

	writeBookOnDisk(t, libRoot, "added-behind-our-back", "onda1skb", "On Disk")
	if _, err := s.Rescan(RescanOptions{}); err != nil {
		t.Fatalf("Rescan: %v", err)
	}

	after := readBookCacheFileOrFail(t, libRoot)
	if _, ok := after.Books["onda1skb"]; ok {
		t.Error("Rescan exported the book cache; it must not write to the shelf")
	}
	if after.Timestamp != before.Timestamp {
		t.Errorf("exported cache timestamp changed from %d to %d; Rescan must not write",
			before.Timestamp, after.Timestamp)
	}
}

func readBookCacheFileOrFail(t *testing.T, libRoot string) BookCacheFile {
	t.Helper()

	filePath := path.Join(libRoot, appFolder, bookCacheFilePrefix+testWriterID+bookCacheFileSuffix)
	cache, err := readBookCacheFileAt(filePath)
	if err != nil {
		t.Fatalf("read exported book cache: %v", err)
	}
	return *cache
}

func TestRescanRefusesASecondWalkAndNamesTheRunningOne(t *testing.T) {
	s := newTestShelf(t, &ShelfConf{LibRoot: t.TempDir(), LockMode: "none"})

	// Claimed directly rather than by racing two Rescan calls: the refusal is
	// what is under test, and a race that has to be won to observe it would be
	// flaky in exactly the direction that hides a regression.
	runningID, _ := s.beginRescan()
	if runningID == "" {
		t.Fatal("beginRescan did not claim a free shelf")
	}

	result, err := s.Rescan(RescanOptions{})
	if !errors.Is(err, ErrRescanInProgress) {
		t.Fatalf("Rescan error = %v, want ErrRescanInProgress", err)
	}
	if result.ID != runningID {
		t.Errorf("refused result ID = %q, want the running scan's ID %q", result.ID, runningID)
	}
	if result.BookCount != 0 || result.FolderCount != 0 {
		t.Error("a refused rescan reported counts it never walked for")
	}

	s.endRescan()
	if _, err := s.Rescan(RescanOptions{}); err != nil {
		t.Fatalf("Rescan after the running one ended: %v", err)
	}
}
