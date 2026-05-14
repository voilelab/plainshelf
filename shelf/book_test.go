package shelf

import (
	"bytes"
	"io"
	"os"
	"path"
	"testing"

	"github.com/voilelab/plainshelf/internal/fsutil"
)

func TestGetBook(t *testing.T) {
	testdataRoot, err := os.OpenRoot("testdata")
	if err != nil {
		t.Fatalf("Failed to open testdata root: %v", err)
	}
	defer testdataRoot.Close()

	rootFS := fsutil.NewRootFS(testdataRoot)
	book, err := openBook(rootFS, "book-a82m")
	if err != nil {
		t.Fatalf("Failed to open book: %v", err)
	}

	expectedTitle := "Book Title"
	if book.Title() != expectedTitle {
		t.Errorf("Expected book title '%s', got '%s'", expectedTitle, book.Title())
	}

	_, err = openBook(rootFS, "nonexistent-book")
	if err == nil {
		t.Fatalf("Expected error when getting nonexistent book, but got none")
	}
}

func TestGetBookCover(t *testing.T) {
	testdataRoot, err := os.OpenRoot("testdata")
	if err != nil {
		t.Fatalf("Failed to open testdata root: %v", err)
	}
	defer testdataRoot.Close()

	rootFS := fsutil.NewRootFS(testdataRoot)
	book, err := openBook(rootFS, "book-a82m")
	if err != nil {
		t.Fatalf("Failed to open book: %v", err)
	}

	coverData, _, err := book.OpenCover()
	if err != nil {
		t.Fatalf("Failed to get book cover: %v", err)
	}

	expectedCoverData := []byte{0x89, 0x50, 0x4E, 0x47} // PNG file signature
	if len(coverData) < 4 || !bytes.Equal(coverData[:4], expectedCoverData) {
		t.Errorf("Expected cover data to start with PNG signature, got %v", coverData[:4])
	}
}

func TestGetSnapshot(t *testing.T) {
	testdataRoot, err := os.OpenRoot("testdata")
	if err != nil {
		t.Fatalf("Failed to open testdata root: %v", err)
	}
	defer testdataRoot.Close()

	rootFS := fsutil.NewRootFS(testdataRoot)
	book, err := openBook(rootFS, "book-a82m")
	if err != nil {
		t.Fatalf("Failed to open book: %v", err)
	}

	snapshot, err := book.GetSnapshot("20260315-a1")
	if err != nil {
		t.Fatalf("Failed to get snapshot: %v", err)
	}

	expectedSnapshotID := "20260315-a1"
	if snapshot.ID() != expectedSnapshotID {
		t.Errorf("Expected snapshot ID '%s', got '%s'", expectedSnapshotID, snapshot.ID())
	}

	_, err = book.GetSnapshot("nonexistent-snapshot")
	if err == nil {
		t.Fatalf("Expected error when getting nonexistent snapshot, but got none")
	}
}

func TestGetCurrentSnapshot(t *testing.T) {
	testdataRoot, err := os.OpenRoot("testdata")
	if err != nil {
		t.Fatalf("Failed to open testdata root: %v", err)
	}
	defer testdataRoot.Close()

	rootFS := fsutil.NewRootFS(testdataRoot)
	book, err := openBook(rootFS, "book-a82m")
	if err != nil {
		t.Fatalf("Failed to open book: %v", err)
	}

	currentSnapshotID := book.CurrentSnapshot()
	expectedSnapshotID := "20260315-a1"
	if currentSnapshotID != expectedSnapshotID {
		t.Errorf("Expected current snapshot ID '%s', got '%s'", expectedSnapshotID, currentSnapshotID)
	}
}

func TestListSnapshots(t *testing.T) {
	testdataRoot, err := os.OpenRoot("testdata")
	if err != nil {
		t.Fatalf("Failed to open testdata root: %v", err)
	}
	defer testdataRoot.Close()

	rootFS := fsutil.NewRootFS(testdataRoot)
	book, err := openBook(rootFS, "book-a82m")
	if err != nil {
		t.Fatalf("Failed to open book: %v", err)
	}

	snapshots, err := book.ListSnapshot()
	if err != nil {
		t.Fatalf("Failed to list snapshots: %v", err)
	}

	if len(snapshots) != 1 {
		t.Fatalf("Expected 1 snapshot, got %d", len(snapshots))
	}

	expectedSnapshotID := "20260315-a1"
	if snapshots[0].ID() != expectedSnapshotID {
		t.Errorf("Expected snapshot ID '%s', got '%s'", expectedSnapshotID, snapshots[0].ID())
	}
}

func TestNewBook(t *testing.T) {
	tmpLib := path.Join(t.TempDir())
	tmpRoot, err := os.OpenRoot(tmpLib)
	if err != nil {
		t.Fatalf("Failed to open temporary root: %v", err)
	}
	defer tmpRoot.Close()

	bookID := "test-book-a38j"
	title := "Test Book"

	rootFS := fsutil.NewRootFS(tmpRoot)
	book, err := createBook(rootFS, bookID, bookID, title)
	if err != nil {
		t.Fatalf("Failed to create new book: %v", err)
	}

	if book.ID() != bookID {
		t.Errorf("Expected book ID '%s', got '%s'", bookID, book.ID())
	}

	if book.Title() != title {
		t.Errorf("Expected book title '%s', got '%s'", title, book.Title())
	}

	// Check if the book folder was created
	bookPath := path.Join(tmpLib, bookID)
	if _, err := os.Open(bookPath); err != nil {
		t.Fatalf("Expected book folder to be created, but got error: %v", err)
	}
}

func TestSetCover(t *testing.T) {
	tmpLib := path.Join(t.TempDir())
	tmpRoot, err := os.OpenRoot(tmpLib)
	if err != nil {
		t.Fatalf("Failed to open temporary root: %v", err)
	}
	defer tmpRoot.Close()

	bookID := "test-book-a38j"
	title := "Test Book"

	rootFS := fsutil.NewRootFS(tmpRoot)
	book, err := createBook(rootFS, bookID, bookID, title)
	if err != nil {
		t.Fatalf("Failed to create new book: %v", err)
	}

	coverData := []byte{0x89, 0x50, 0x4E, 0x47} // PNG file signature
	err = book.SetCover(coverData, ".png")
	if err != nil {
		t.Fatalf("Failed to set book cover: %v", err)
	}

	retrievedCoverData, _, err := book.OpenCover()
	if err != nil {
		t.Fatalf("Failed to get book cover: %v", err)
	}

	if !bytes.Equal(retrievedCoverData, coverData) {
		t.Errorf("Expected retrieved cover data to match set cover data, got %v", retrievedCoverData)
	}

	// Check if the cover file was created
	coverPath := path.Join(tmpLib, bookID, "cover.png")
	if _, err := os.Open(coverPath); err != nil {
		t.Fatalf("Expected cover file to be created, but got error: %v", err)
	}
}

func TestNewSnapshot(t *testing.T) {
	tmpLib := path.Join(t.TempDir())
	tmpRoot, err := os.OpenRoot(tmpLib)
	if err != nil {
		t.Fatalf("Failed to open temporary root: %v", err)
	}
	defer tmpRoot.Close()

	bookID := "test-book-a38j"
	title := "Test Book"

	rootFS := fsutil.NewRootFS(tmpRoot)
	book, err := createBook(rootFS, bookID, bookID, title)
	if err != nil {
		t.Fatalf("Failed to create new book: %v", err)
	}

	srcText := "This is the content of the snapshot."
	snapshot, err := book.NewSnapshot(bytes.NewReader([]byte(srcText)))
	if err != nil {
		t.Fatalf("Failed to create new snapshot: %v", err)
	}

	retrievedSnapshot, err := book.GetSnapshot(snapshot.ID())
	if err != nil {
		t.Fatalf("Failed to get snapshot: %v", err)
	}

	getSrc, err := retrievedSnapshot.OpenSource()
	if err != nil {
		t.Fatalf("Failed to open snapshot source: %v", err)
	}

	retrievedSrcData, err := io.ReadAll(getSrc)
	if err != nil {
		t.Fatalf("Failed to read snapshot source data: %v", err)
	}

	if string(retrievedSrcData) != srcText {
		t.Errorf("Expected retrieved snapshot source to match original source, got '%s'", string(retrievedSrcData))
	}
}

func TestSetCurrentSnapshot(t *testing.T) {
	tmpLib := path.Join(t.TempDir())
	tmpRoot, err := os.OpenRoot(tmpLib)
	if err != nil {
		t.Fatalf("Failed to open temporary root: %v", err)
	}
	defer tmpRoot.Close()

	bookID := "test-book-a38j"
	title := "Test Book"

	rootFS := fsutil.NewRootFS(tmpRoot)
	book, err := createBook(rootFS, bookID, bookID, title)
	if err != nil {
		t.Fatalf("Failed to create new book: %v", err)
	}

	srcText := "This is the content of the snapshot."
	snapshot, err := book.NewSnapshot(bytes.NewReader([]byte(srcText)))
	if err != nil {
		t.Fatalf("Failed to create new snapshot: %v", err)
	}

	err = book.SetCurrentSnapshot(snapshot.ID())
	if err != nil {
		t.Fatalf("Failed to set current snapshot: %v", err)
	}

	if book.CurrentSnapshot() != snapshot.ID() {
		t.Errorf("Expected current snapshot ID to be '%s', got '%s'", snapshot.ID(), book.CurrentSnapshot())
	}

	srcText2 := "This is the content of the second snapshot."
	snapshot2, err := book.NewSnapshot(bytes.NewReader([]byte(srcText2)))
	if err != nil {
		t.Fatalf("Failed to create second snapshot: %v", err)
	}

	err = book.SetCurrentSnapshot(snapshot2.ID())
	if err != nil {
		t.Fatalf("Failed to set current snapshot: %v", err)
	}

	if book.CurrentSnapshot() != snapshot2.ID() {
		t.Errorf("Expected current snapshot ID to be '%s', got '%s'", snapshot2.ID(), book.CurrentSnapshot())
	}

	// Set current snapshot back to the first snapshot
	err = book.SetCurrentSnapshot(snapshot.ID())
	if err != nil {
		t.Fatalf("Failed to set current snapshot: %v", err)
	}

	if book.CurrentSnapshot() != snapshot.ID() {
		t.Errorf("Expected current snapshot ID to be '%s', got '%s'", snapshot.ID(), book.CurrentSnapshot())
	}
}
