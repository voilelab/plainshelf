package shelf

import (
	"encoding/json"
	"log"
	"os"
	"path"
	"testing"
	"time"
)

func TestShelfNewShelf(t *testing.T) {
	shelf, err := NewShelf(&ShelfConf{LibRoot: path.Join("testdata", "lib")})
	if err != nil {
		t.Fatalf("Failed to initialize Shelf: %v", err)
	}
	defer shelf.Close()
}

func TestOpenLocalShelfReturnsOpenRootError(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := path.Join(tmpDir, "not-a-directory")
	if err := os.WriteFile(filePath, []byte("not a shelf"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	shelf, err := NewShelf(&ShelfConf{LibRoot: filePath})
	if err == nil {
		if shelf != nil {
			_ = shelf.Close()
		}
		t.Fatal("Expected error when opening a regular file as a shelf root, got nil")
	}
}

func TestShelfMakeStructure(t *testing.T) {
	tmpLib := path.Join(t.TempDir(), "shelf_test")
	shelf, err := NewShelf(&ShelfConf{LibRoot: tmpLib})
	if err != nil {
		t.Fatalf("Failed to initialize Shelf: %v", err)
	}
	defer shelf.Close()

	// Check if the Books folder was created
	booksPath := path.Join(tmpLib, booksFolder)
	if _, err := os.Open(booksPath); err != nil {
		t.Fatalf("Expected Books folder to be created, but got error: %v", err)
	}

	// Check if the AppTmp folder was created
	appTmpPath := path.Join(tmpLib, appFolder, appTmpFolder)
	if _, err := os.Open(appTmpPath); err != nil {
		t.Fatalf("Expected AppTmp folder to be created, but got error: %v", err)
	}
}

func TestShelfListBooks(t *testing.T) {
	shelf, err := NewShelf(&ShelfConf{LibRoot: path.Join("testdata", "lib")})
	if err != nil {
		t.Fatalf("Failed to initialize Shelf: %v", err)
	}
	defer shelf.Close()

	books, err := shelf.ListBooks()
	if err != nil {
		t.Fatalf("Failed to list books: %v", err)
	}

	if len(books) != 2 {
		t.Fatalf("Expected 2 books, got %d", len(books))
	}

	expectedTitle := "Book Title"
	if books[0].Title() != expectedTitle {
		t.Errorf("Expected book title '%s', got '%s'", expectedTitle, books[0].Title())
	}
}

func TestShelfGetBook(t *testing.T) {
	shelf, err := NewShelf(&ShelfConf{LibRoot: path.Join("testdata", "lib")})
	if err != nil {
		t.Fatalf("Failed to initialize Shelf: %v", err)
	}
	defer shelf.Close()

	book, err := shelf.GetBook("book-a82m")
	if err != nil {
		t.Fatalf("Failed to get book: %v", err)
	}

	expectedTitle := "Book Title"
	if book.Title() != expectedTitle {
		t.Errorf("Expected book title '%s', got '%s'", expectedTitle, book.Title())
	}
}

func TestShelfGetBookNotFound(t *testing.T) {
	shelf, err := NewShelf(&ShelfConf{LibRoot: path.Join("testdata", "lib")})
	if err != nil {
		t.Fatalf("Failed to initialize Shelf: %v", err)
	}
	defer shelf.Close()

	_, err = shelf.GetBook("nonexistent")
	if err == nil {
		t.Fatal("Expected error when getting non-existent book, but got nil")
	}
}

func TestShelfGetAllLayers(t *testing.T) {
	shelf, err := NewShelf(&ShelfConf{LibRoot: path.Join("testdata", "lib")})
	if err != nil {
		t.Fatalf("Failed to initialize Shelf: %v", err)
	}
	defer shelf.Close()

	layers, err := shelf.GetAllLayers()
	if err != nil {
		t.Fatalf("Failed to get all layers: %v", err)
	}

	expectedLayers := []Layers{{""}, {"default"}, {"default", "test"}, {"empty"}}
	log.Println("Expected layers:", expectedLayers)
	log.Println("Actual layers:", layers)
	if len(layers) != len(expectedLayers) {
		t.Fatalf("Expected %d layers, got %d", len(expectedLayers), len(layers))
	}

	for i, layer := range expectedLayers {
		if layers[i].String() != layer.String() {
			t.Errorf("Expected layer '%s', got '%s'", layer.String(), layers[i].String())
		}
	}
}

func TestShelfGetBookByLayer(t *testing.T) {
	shelf, err := NewShelf(&ShelfConf{LibRoot: path.Join("testdata", "lib")})
	if err != nil {
		t.Fatalf("Failed to initialize Shelf: %v", err)
	}
	defer shelf.Close()

	books, err := shelf.GetBooksByLayer([]string{"default", "test"})
	if err != nil {
		t.Fatalf("Failed to get book by layer: %v", err)
	}

	if len(books) != 1 {
		t.Fatalf("Expected 1 book in layer 'default/test', got %d", len(books))
	}

	expectedTitle := "Book Title"
	if books[0].Title() != expectedTitle {
		t.Errorf("Expected book title '%s', got '%s'", expectedTitle, books[0].Title())
	}
}

func TestShelfNewBook(t *testing.T) {
	tmpLib := path.Join(t.TempDir(), "shelf_test")
	shelf, err := NewShelf(&ShelfConf{LibRoot: tmpLib})
	if err != nil {
		t.Fatalf("Failed to initialize Shelf: %v", err)
	}
	defer shelf.Close()

	book, err := shelf.NewBook([]string{"new", "layer"}, "New Book")
	if err != nil {
		t.Fatalf("Failed to create new book: %v", err)
	}

	if book.Title() != "New Book" {
		t.Errorf("Expected book title 'New Book', got '%s'", book.Title())
	}
}

func TestShelfDeleteBook(t *testing.T) {
	tmpLib := path.Join(t.TempDir(), "shelf_test")
	shelf, err := NewShelf(&ShelfConf{LibRoot: tmpLib})
	if err != nil {
		t.Fatalf("Failed to initialize Shelf: %v", err)
	}
	defer shelf.Close()

	book, err := shelf.NewBook([]string{"new", "layer"}, "New Book")
	if err != nil {
		t.Fatalf("Failed to create new book: %v", err)
	}

	err = shelf.DeleteBook(book.ID())
	if err != nil {
		t.Fatalf("Failed to delete book: %v", err)
	}

	_, err = shelf.GetBook(book.ID())
	if err == nil {
		t.Fatal("Expected error when getting deleted book, but got nil")
	}
}

func TestShelfMoveBook(t *testing.T) {
	tmpLib := path.Join(t.TempDir(), "shelf_test")
	shelf, err := NewShelf(&ShelfConf{LibRoot: tmpLib, ScanInterval: "0s"})
	if err != nil {
		t.Fatalf("Failed to initialize Shelf: %v", err)
	}
	defer shelf.Close()

	book, err := shelf.NewBook([]string{"layer1"}, "Book to Move")
	if err != nil {
		t.Fatalf("Failed to create new book: %v", err)
	}

	movedBook, err := shelf.MoveBook(book.ID(), []string{"layer2"})
	if err != nil {
		t.Fatalf("Failed to move book: %v", err)
	}

	if movedBook.Title() != "Book to Move" {
		t.Errorf("Expected book title 'Book to Move', got '%s'", movedBook.Title())
	}

	booksInLayer1, err := shelf.GetBooksByLayer([]string{"layer1"})
	if err != nil {
		t.Fatalf("Failed to get books in layer1: %v", err)
	}
	if len(booksInLayer1) != 0 {
		t.Errorf("Expected 0 books in layer1 after move, got %d", len(booksInLayer1))
	}

	booksInLayer2, err := shelf.GetBooksByLayer([]string{"layer2"})
	if err != nil {
		t.Fatalf("Failed to get books in layer2: %v", err)
	}
	if len(booksInLayer2) != 1 {
		t.Errorf("Expected 1 book in layer2 after move, got %d", len(booksInLayer2))
	}
	if booksInLayer2[0].ID() != book.ID() {
		t.Errorf("Expected moved book ID '%s', got '%s'", book.ID(), booksInLayer2[0].ID())
	}
}

func TestShelfGetBookRefreshesWhenBookMetaChangesOnDisk(t *testing.T) {
	tmpLib := path.Join(t.TempDir(), "lib")

	err := os.CopyFS(tmpLib, os.DirFS("testdata/lib"))
	if err != nil {
		t.Fatalf("Failed to copy test library: %v", err)
	}

	shelf, err := NewShelf(&ShelfConf{LibRoot: tmpLib})
	if err != nil {
		t.Fatalf("Failed to initialize Shelf: %v", err)
	}
	defer shelf.Close()

	book, err := shelf.GetBook("book-a82m")
	if err != nil {
		t.Fatalf("Failed to get book before disk update: %v", err)
	}
	if got := book.Title(); got != "Book Title" {
		t.Fatalf("Expected initial title %q, got %q", "Book Title", got)
	}

	metaPath := path.Join(tmpLib, booksFolder, "default", "test", "book-a82m.novl", BookMetaFile)
	metaBytes, err := os.ReadFile(metaPath)
	if err != nil {
		t.Fatalf("Failed to read book meta: %v", err)
	}

	var meta BookMeta
	if err := json.Unmarshal(metaBytes, &meta); err != nil {
		t.Fatalf("Failed to unmarshal book meta: %v", err)
	}
	meta.Title = "Book Title Updated On Disk"

	updatedMetaBytes, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal updated book meta: %v", err)
	}

	time.Sleep(time.Until(time.Now().Truncate(time.Second).Add(time.Second)))
	if err := os.WriteFile(metaPath, updatedMetaBytes, 0o644); err != nil {
		t.Fatalf("Failed to write updated book meta: %v", err)
	}

	refreshedBook, err := shelf.GetBook("book-a82m")
	if err != nil {
		t.Fatalf("Failed to get book after disk update: %v", err)
	}
	if got := refreshedBook.Title(); got != "Book Title Updated On Disk" {
		t.Fatalf("Expected refreshed title %q, got %q", "Book Title Updated On Disk", got)
	}
}

func TestShelfListBooksRefreshesStaleMetaAndDiscoversNewBookOnCacheMiss(t *testing.T) {
	tmpLib := path.Join(t.TempDir(), "lib")

	err := os.CopyFS(tmpLib, os.DirFS("testdata/lib"))
	if err != nil {
		t.Fatalf("Failed to copy test library: %v", err)
	}

	shelf, err := NewShelf(&ShelfConf{LibRoot: tmpLib, ScanInterval: "0s"})
	if err != nil {
		t.Fatalf("Failed to initialize Shelf: %v", err)
	}
	defer shelf.Close()

	books, err := shelf.ListBooks()
	if err != nil {
		t.Fatalf("Failed to list books before updates: %v", err)
	}
	if len(books) != 2 {
		t.Fatalf("Expected 2 books before updates, got %d", len(books))
	}

	metaPath := path.Join(tmpLib, booksFolder, "default", "test", "book-a82m.novl", BookMetaFile)
	metaBytes, err := os.ReadFile(metaPath)
	if err != nil {
		t.Fatalf("Failed to read existing book meta: %v", err)
	}
	var meta BookMeta
	if err := json.Unmarshal(metaBytes, &meta); err != nil {
		t.Fatalf("Failed to unmarshal existing book meta: %v", err)
	}
	meta.Title = "List Refresh Title"
	updatedMetaBytes, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal existing book meta: %v", err)
	}
	time.Sleep(time.Until(time.Now().Truncate(time.Second).Add(time.Second)))
	if err := os.WriteFile(metaPath, updatedMetaBytes, 0o644); err != nil {
		t.Fatalf("Failed to write existing book meta: %v", err)
	}

	newBookPath := path.Join(tmpLib, booksFolder, "default", "test", "book-new.novl")
	if err := os.MkdirAll(newBookPath, 0o755); err != nil {
		t.Fatalf("Failed to create new book directory: %v", err)
	}

	newMeta := BookMeta{ID: "book-new", Title: "Brand New Book", Language: "en"}
	newMetaBytes, err := json.MarshalIndent(newMeta, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal new book meta: %v", err)
	}
	if err := os.WriteFile(path.Join(newBookPath, BookMetaFile), newMetaBytes, 0o644); err != nil {
		t.Fatalf("Failed to write new book meta: %v", err)
	}

	books, err = shelf.ListBooks()
	if err != nil {
		t.Fatalf("Failed to list books after updates: %v", err)
	}

	seenUpdated := false
	for _, b := range books {
		if b.ID() == "book-a82m" && b.Title() == "List Refresh Title" {
			seenUpdated = true
			break
		}
	}
	if !seenUpdated {
		t.Fatalf("Expected ListBooks to include refreshed metadata for book-a82m")
	}

	newBook, err := shelf.GetBook("book-new")
	if err != nil {
		t.Fatalf("Expected GetBook to discover cache-miss book after directory appears: %v", err)
	}
	if newBook.Title() != "Brand New Book" {
		t.Fatalf("Expected new book title %q, got %q", "Brand New Book", newBook.Title())
	}
}
