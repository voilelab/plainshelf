package shelf

import (
	"bytes"
	"io"
	"os"
	"path"
	"testing"
	"time"

	"github.com/voilelab/plainshelf/internal/fsutil"
)

func TestGetBook(t *testing.T) {
	testdataRoot, err := os.OpenRoot("testdata")
	if err != nil {
		t.Fatalf("Failed to open testdata root: %v", err)
	}
	defer testdataRoot.Close()

	rootFS := fsutil.NewRootFS(testdataRoot)
	book, err := openBook(rootFS, newLoggerForTest(), "book-a82m")
	if err != nil {
		t.Fatalf("Failed to open book: %v", err)
	}

	expectedTitle := "Book Title"
	if book.Title() != expectedTitle {
		t.Errorf("Expected book title '%s', got '%s'", expectedTitle, book.Title())
	}

	_, err = openBook(rootFS, newLoggerForTest(), "nonexistent-book")
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
	book, err := openBook(rootFS, newLoggerForTest(), "book-a82m")
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

func TestGetSource(t *testing.T) {
	testdataRoot, err := os.OpenRoot("testdata")
	if err != nil {
		t.Fatalf("Failed to open testdata root: %v", err)
	}
	defer testdataRoot.Close()

	rootFS := fsutil.NewRootFS(testdataRoot)
	book, err := openBook(rootFS, newLoggerForTest(), "book-a82m")
	if err != nil {
		t.Fatalf("Failed to open book: %v", err)
	}

	source, err := book.GetSource("20260315-a1")
	if err != nil {
		t.Fatalf("Failed to get source: %v", err)
	}

	expectedSourceID := "20260315-a1"
	if source.ID() != expectedSourceID {
		t.Errorf("Expected source ID '%s', got '%s'", expectedSourceID, source.ID())
	}

	_, err = book.GetSource("nonexistent-source")
	if err == nil {
		t.Fatalf("Expected error when getting nonexistent source, but got none")
	}
}

func TestGetCurrentSource(t *testing.T) {
	testdataRoot, err := os.OpenRoot("testdata")
	if err != nil {
		t.Fatalf("Failed to open testdata root: %v", err)
	}
	defer testdataRoot.Close()

	rootFS := fsutil.NewRootFS(testdataRoot)
	book, err := openBook(rootFS, newLoggerForTest(), "book-a82m")
	if err != nil {
		t.Fatalf("Failed to open book: %v", err)
	}

	currentSourceID := book.CurrentSource()
	expectedSourceID := "20260315-a1"
	if currentSourceID != expectedSourceID {
		t.Errorf("Expected current source ID '%s', got '%s'", expectedSourceID, currentSourceID)
	}
}

func TestListSources(t *testing.T) {
	testdataRoot, err := os.OpenRoot("testdata")
	if err != nil {
		t.Fatalf("Failed to open testdata root: %v", err)
	}
	defer testdataRoot.Close()

	rootFS := fsutil.NewRootFS(testdataRoot)
	book, err := openBook(rootFS, newLoggerForTest(), "book-a82m")
	if err != nil {
		t.Fatalf("Failed to open book: %v", err)
	}

	sources, err := book.ListSource()
	if err != nil {
		t.Fatalf("Failed to list sources: %v", err)
	}

	if len(sources) != 1 {
		t.Fatalf("Expected 1 source, got %d", len(sources))
	}

	expectedSourceID := "20260315-a1"
	if sources[0].ID() != expectedSourceID {
		t.Errorf("Expected source ID '%s', got '%s'", expectedSourceID, sources[0].ID())
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
	book, err := createBook(rootFS, newLoggerForTest(), bookID, bookID, title)
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
	book, err := createBook(rootFS, newLoggerForTest(), bookID, bookID, title)
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

func TestNewSource(t *testing.T) {
	tmpLib := path.Join(t.TempDir())
	tmpRoot, err := os.OpenRoot(tmpLib)
	if err != nil {
		t.Fatalf("Failed to open temporary root: %v", err)
	}
	defer tmpRoot.Close()

	bookID := "test-book-a38j"
	title := "Test Book"

	rootFS := fsutil.NewRootFS(tmpRoot)
	book, err := createBook(rootFS, newLoggerForTest(), bookID, bookID, title)
	if err != nil {
		t.Fatalf("Failed to create new book: %v", err)
	}

	srcText := "This is the content of the source."
	source, err := book.NewSource(bytes.NewReader([]byte(srcText)))
	if err != nil {
		t.Fatalf("Failed to create new source: %v", err)
	}

	retrievedSource, err := book.GetSource(source.ID())
	if err != nil {
		t.Fatalf("Failed to get source: %v", err)
	}

	getSrc, err := retrievedSource.Open()
	if err != nil {
		t.Fatalf("Failed to open source: %v", err)
	}

	retrievedSrcData, err := io.ReadAll(getSrc)
	if err != nil {
		t.Fatalf("Failed to read source data: %v", err)
	}

	if string(retrievedSrcData) != srcText {
		t.Errorf("Expected retrieved source to match original source, got '%s'", string(retrievedSrcData))
	}
}

func TestSetCurrentSource(t *testing.T) {
	tmpLib := path.Join(t.TempDir())
	tmpRoot, err := os.OpenRoot(tmpLib)
	if err != nil {
		t.Fatalf("Failed to open temporary root: %v", err)
	}
	defer tmpRoot.Close()

	bookID := "test-book-a38j"
	title := "Test Book"

	rootFS := fsutil.NewRootFS(tmpRoot)
	book, err := createBook(rootFS, newLoggerForTest(), bookID, bookID, title)
	if err != nil {
		t.Fatalf("Failed to create new book: %v", err)
	}

	srcText := "This is the content of the source."
	source, err := book.NewSource(bytes.NewReader([]byte(srcText)))
	if err != nil {
		t.Fatalf("Failed to create new source: %v", err)
	}

	err = book.SetCurrentSource(source.ID())
	if err != nil {
		t.Fatalf("Failed to set current source: %v", err)
	}

	if book.CurrentSource() != source.ID() {
		t.Errorf("Expected current source ID to be '%s', got '%s'", source.ID(), book.CurrentSource())
	}

	srcText2 := "This is the content of the second source."
	source2, err := book.NewSource(bytes.NewReader([]byte(srcText2)))
	if err != nil {
		t.Fatalf("Failed to create second source: %v", err)
	}

	err = book.SetCurrentSource(source2.ID())
	if err != nil {
		t.Fatalf("Failed to set current source: %v", err)
	}

	if book.CurrentSource() != source2.ID() {
		t.Errorf("Expected current source ID to be '%s', got '%s'", source2.ID(), book.CurrentSource())
	}

	// Set current source back to the first source
	err = book.SetCurrentSource(source.ID())
	if err != nil {
		t.Fatalf("Failed to set current source: %v", err)
	}

	if book.CurrentSource() != source.ID() {
		t.Errorf("Expected current source ID to be '%s', got '%s'", source.ID(), book.CurrentSource())
	}
}

func TestSetMetaMarksOtherInstanceStale(t *testing.T) {
	tmpLib := path.Join(t.TempDir())
	tmpRoot, err := os.OpenRoot(tmpLib)
	if err != nil {
		t.Fatalf("Failed to open temporary root: %v", err)
	}
	defer tmpRoot.Close()

	bookID := "test-book-a38j"
	title := "Test Book"

	rootFS := fsutil.NewRootFS(tmpRoot)
	book1, err := createBook(rootFS, newLoggerForTest(), bookID, bookID, title)
	if err != nil {
		t.Fatalf("Failed to create new book: %v", err)
	}

	book2, err := openBook(rootFS, newLoggerForTest(), bookID)
	if err != nil {
		t.Fatalf("Failed to open second book instance: %v", err)
	}

	if book1.IsStale() {
		t.Fatalf("Expected first instance to be fresh initially")
	}
	if book2.IsStale() {
		t.Fatalf("Expected second instance to be fresh initially")
	}

	meta := book1.GetMeta()
	meta.Comments = "updated by book1"

	// Ensure filesystem mtime has advanced on platforms with coarse timestamp precision.
	time.Sleep(10 * time.Millisecond)

	err = book1.SetMeta(meta)
	if err != nil {
		t.Fatalf("Failed to set book meta from first instance: %v", err)
	}

	if book1.IsStale() {
		t.Fatalf("Expected first instance to remain fresh after SetMeta")
	}
	if !book2.IsStale() {
		t.Fatalf("Expected second instance to become stale after first instance updates meta")
	}
}

