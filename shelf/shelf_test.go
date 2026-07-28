package shelf

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"os"
	"path"
	"testing"
	"time"
)

func TestShelfNewShelf(t *testing.T) {
	_ = newTestShelf(t, &ShelfConf{LibRoot: path.Join("testdata", "lib")})
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
	_ = newTestShelf(t, &ShelfConf{LibRoot: tmpLib})

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

func TestShelfRuntimeStateAndScanInterval(t *testing.T) {
	shelf := newTestShelf(t, &ShelfConf{
		LibRoot:           t.TempDir(),
		LockMode:          "none",
		ScanInterval:      "2m",
		BookCheckInterval: "3m",
	})

	if !shelf.IsReady() {
		t.Fatal("shelf is not ready after WaitReady")
	}
	if shelf.InitErr() != nil {
		t.Fatalf("InitErr = %v, want nil", shelf.InitErr())
	}
	if shelf.ReadingStats() == nil {
		t.Fatal("ReadingStats returned nil")
	}

	if err := shelf.SetScanInterval("invalid"); err == nil {
		t.Fatal("SetScanInterval accepted an invalid duration")
	}
	if err := shelf.SetScanInterval(""); err != nil {
		t.Fatalf("SetScanInterval(default): %v", err)
	}
	shelf.bookCache.RLock()
	interval := shelf.bookCache.scanInterval
	bookCheckInterval := shelf.bookCache.bookCheckInterval
	shelf.bookCache.RUnlock()
	if interval != time.Minute {
		t.Fatalf("default scan interval = %v, want %v", interval, time.Minute)
	}
	if bookCheckInterval != 3*time.Minute {
		t.Fatalf("book check interval = %v, want %v", bookCheckInterval, 3*time.Minute)
	}
}

func TestShelfWaitReadyCancellationAndInitializingReads(t *testing.T) {
	shelf := &Shelf{readyCh: make(chan struct{})}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := shelf.WaitReady(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("WaitReady error = %v, want context.Canceled", err)
	}
	if _, err := shelf.ListBooks(); !errors.Is(err, ErrShelfInitializing) {
		t.Fatalf("ListBooks error = %v, want ErrShelfInitializing", err)
	}
	if _, err := shelf.GetBook("book"); !errors.Is(err, ErrShelfInitializing) {
		t.Fatalf("GetBook error = %v, want ErrShelfInitializing", err)
	}
	if _, err := shelf.GetBooksByLayer(nil); !errors.Is(err, ErrShelfInitializing) {
		t.Fatalf("GetBooksByLayer error = %v, want ErrShelfInitializing", err)
	}
}

func TestShelfDeleteLayer(t *testing.T) {
	shelf := newTestShelf(t, &ShelfConf{LibRoot: t.TempDir(), LockMode: "none"})

	if err := shelf.NewLayer(Layers{"empty"}); err != nil {
		t.Fatalf("NewLayer(empty): %v", err)
	}
	if err := shelf.DeleteLayer(Layers{"empty"}); err != nil {
		t.Fatalf("DeleteLayer(empty): %v", err)
	}
	if _, err := shelf.dbRoot.Stat(path.Join(booksFolder, "empty")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("deleted layer stat error = %v, want os.ErrNotExist", err)
	}

	if _, err := shelf.NewBook(Layers{"occupied"}, "Resident"); err != nil {
		t.Fatalf("NewBook(occupied): %v", err)
	}
	if err := shelf.DeleteLayer(Layers{"occupied"}); err == nil {
		t.Fatal("DeleteLayer removed a non-empty layer")
	}
	if _, err := shelf.dbRoot.Stat(path.Join(booksFolder, "occupied")); err != nil {
		t.Fatalf("non-empty layer disappeared: %v", err)
	}

	if err := shelf.DeleteLayer(Layers{"missing"}); err == nil {
		t.Fatal("DeleteLayer accepted a missing layer")
	}
}

func TestShelfListBooks(t *testing.T) {
	shelf := newTestShelf(t, &ShelfConf{LibRoot: path.Join("testdata", "lib")})

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
	shelf := newTestShelf(t, &ShelfConf{LibRoot: path.Join("testdata", "lib")})

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
	shelf := newTestShelf(t, &ShelfConf{LibRoot: path.Join("testdata", "lib")})

	_, err := shelf.GetBook("nonexistent")
	if err == nil {
		t.Fatal("Expected error when getting non-existent book, but got nil")
	}
}

func TestShelfGetAllLayers(t *testing.T) {
	shelf := newTestShelf(t, &ShelfConf{LibRoot: path.Join("testdata", "lib")})

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
	shelf := newTestShelf(t, &ShelfConf{LibRoot: path.Join("testdata", "lib")})

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
	shelf := newTestShelf(t, &ShelfConf{LibRoot: tmpLib})

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
	shelf := newTestShelf(t, &ShelfConf{LibRoot: tmpLib})

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

func TestShelfTrashLifecycle(t *testing.T) {
	tmpLib := path.Join(t.TempDir(), "shelf_test")
	shelf := newTestShelf(t, &ShelfConf{LibRoot: tmpLib})

	book, err := shelf.NewBook([]string{"new", "layer"}, "Trash Me")
	if err != nil {
		t.Fatalf("Failed to create new book: %v", err)
	}

	if err := shelf.DeleteBook(book.ID()); err != nil {
		t.Fatalf("Failed to move book to trash: %v", err)
	}

	books, err := shelf.ListBooks()
	if err != nil {
		t.Fatalf("Failed to list books after trash: %v", err)
	}
	if len(books) != 0 {
		t.Fatalf("Expected no active books after trash, got %d", len(books))
	}

	trashed, err := shelf.ListTrashedBooks()
	if err != nil {
		t.Fatalf("Failed to list trashed books: %v", err)
	}
	if len(trashed) != 1 {
		t.Fatalf("Expected 1 trashed book, got %d", len(trashed))
	}
	if trashed[0].ID != book.ID() {
		t.Fatalf("Trashed book ID = %s, want %s", trashed[0].ID, book.ID())
	}
	if got := trashed[0].OriginalLayer.String(); got != "new/layer" {
		t.Fatalf("Trashed original layer = %s, want new/layer", got)
	}

	if err := shelf.RestoreTrashedBook(book.ID()); err != nil {
		t.Fatalf("Failed to restore trashed book: %v", err)
	}

	restored, err := shelf.GetBook(book.ID())
	if err != nil {
		t.Fatalf("Failed to get restored book: %v", err)
	}
	if got := restored.Layers().String(); got != "new/layer" {
		t.Fatalf("Restored layer = %s, want new/layer", got)
	}

	if err := shelf.DeleteBook(book.ID()); err != nil {
		t.Fatalf("Failed to trash book a second time: %v", err)
	}
	if err := shelf.DeleteTrashedBook(book.ID()); err != nil {
		t.Fatalf("Failed to permanently delete trashed book: %v", err)
	}
	if err := shelf.RestoreTrashedBook(book.ID()); !errors.Is(err, ErrTrashedBookNotFound) {
		t.Fatalf("Restore deleted trashed book error = %v, want ErrTrashedBookNotFound", err)
	}
}

func TestShelfRestoreTrashResolvesFolderCollision(t *testing.T) {
	shelf := newTestShelf(t, &ShelfConf{LibRoot: t.TempDir(), LockMode: "none"})

	original, err := shelf.NewBook(Layers{"fiction"}, "Same Title")
	if err != nil {
		t.Fatalf("NewBook(original): %v", err)
	}
	originalPath := original.FolderPath()
	if err := shelf.DeleteBook(original.ID()); err != nil {
		t.Fatalf("DeleteBook(original): %v", err)
	}

	replacement, err := shelf.NewBook(Layers{"fiction"}, "Same Title")
	if err != nil {
		t.Fatalf("NewBook(replacement): %v", err)
	}
	if replacement.FolderPath() != originalPath {
		t.Fatalf("replacement path = %q, want original path %q", replacement.FolderPath(), originalPath)
	}

	if err := shelf.RestoreTrashedBook(original.ID()); err != nil {
		t.Fatalf("RestoreTrashedBook: %v", err)
	}
	restored, err := shelf.GetBook(original.ID())
	if err != nil {
		t.Fatalf("GetBook(restored): %v", err)
	}
	if restored.FolderPath() == replacement.FolderPath() {
		t.Fatalf("restored book reused occupied path %q", restored.FolderPath())
	}
	if got := restored.Layers().String(); got != "fiction" {
		t.Fatalf("restored layer = %q, want fiction", got)
	}
}

func TestShelfMoveBook(t *testing.T) {
	tmpLib := path.Join(t.TempDir(), "shelf_test")
	shelf := newTestShelf(t, &ShelfConf{LibRoot: tmpLib, ScanInterval: "0s"})

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

func TestShelfRenameLayer(t *testing.T) {
	tmpLib := path.Join(t.TempDir(), "shelf_test")
	shelf := newTestShelf(t, &ShelfConf{LibRoot: tmpLib, ScanInterval: "0s"})

	book, err := shelf.NewBook([]string{"oldlayer"}, "Test Book")
	if err != nil {
		t.Fatalf("Failed to create book: %v", err)
	}

	if err := shelf.RenameLayer([]string{"oldlayer"}, []string{"newlayer"}); err != nil {
		t.Fatalf("RenameLayer failed: %v", err)
	}

	booksInOld, err := shelf.GetBooksByLayer([]string{"oldlayer"})
	if err != nil {
		t.Fatalf("GetBooksByLayer(oldlayer) failed: %v", err)
	}
	if len(booksInOld) != 0 {
		t.Errorf("Expected 0 books in oldlayer after rename, got %d", len(booksInOld))
	}

	booksInNew, err := shelf.GetBooksByLayer([]string{"newlayer"})
	if err != nil {
		t.Fatalf("GetBooksByLayer(newlayer) failed: %v", err)
	}
	if len(booksInNew) != 1 {
		t.Fatalf("Expected 1 book in newlayer after rename, got %d", len(booksInNew))
	}
	if booksInNew[0].ID() != book.ID() {
		t.Errorf("Expected book ID %q in newlayer, got %q", book.ID(), booksInNew[0].ID())
	}
}

func TestShelfRenameLayerNested(t *testing.T) {
	tmpLib := path.Join(t.TempDir(), "shelf_test")
	shelf := newTestShelf(t, &ShelfConf{LibRoot: tmpLib, ScanInterval: "0s"})

	book, err := shelf.NewBook([]string{"parent", "child"}, "Nested Book")
	if err != nil {
		t.Fatalf("Failed to create book: %v", err)
	}

	if err := shelf.RenameLayer([]string{"parent", "child"}, []string{"parent", "renamed"}); err != nil {
		t.Fatalf("RenameLayer failed: %v", err)
	}

	booksInNew, err := shelf.GetBooksByLayer([]string{"parent", "renamed"})
	if err != nil {
		t.Fatalf("GetBooksByLayer failed: %v", err)
	}
	if len(booksInNew) != 1 {
		t.Fatalf("Expected 1 book in parent/renamed, got %d", len(booksInNew))
	}
	if booksInNew[0].ID() != book.ID() {
		t.Errorf("Expected book ID %q in parent/renamed, got %q", book.ID(), booksInNew[0].ID())
	}
}

func TestShelfRenameLayerAcrossParentsRejected(t *testing.T) {
	tmpLib := path.Join(t.TempDir(), "shelf_test")
	shelf := newTestShelf(t, &ShelfConf{LibRoot: tmpLib, ScanInterval: "0s"})

	book, err := shelf.NewBook([]string{"alpha", "beta"}, "Cross Book")
	if err != nil {
		t.Fatalf("Failed to create book: %v", err)
	}

	if err := shelf.RenameLayer([]string{"alpha", "beta"}, []string{"gamma", "delta"}); err == nil {
		t.Fatal("Expected RenameLayer to reject moving across parents")
	}

	booksInOld, err := shelf.GetBooksByLayer([]string{"alpha", "beta"})
	if err != nil {
		t.Fatalf("GetBooksByLayer failed: %v", err)
	}
	if len(booksInOld) != 1 || booksInOld[0].ID() != book.ID() {
		t.Fatalf("Expected book to remain in alpha/beta, got %#v", booksInOld)
	}
}

func TestShelfMoveLayerUnderExistingLayer(t *testing.T) {
	tmpLib := path.Join(t.TempDir(), "shelf_test")
	shelf := newTestShelf(t, &ShelfConf{LibRoot: tmpLib, ScanInterval: "0s"})

	book, err := shelf.NewBook([]string{"alpha", "beta"}, "Move Layer Book")
	if err != nil {
		t.Fatalf("Failed to create book: %v", err)
	}
	if err := shelf.NewLayer([]string{"gamma"}); err != nil {
		t.Fatalf("Failed to create target layer: %v", err)
	}

	if err := shelf.MoveLayer([]string{"alpha", "beta"}, []string{"gamma"}); err != nil {
		t.Fatalf("MoveLayer failed: %v", err)
	}

	booksInNew, err := shelf.GetBooksByLayer([]string{"gamma", "beta"})
	if err != nil {
		t.Fatalf("GetBooksByLayer failed: %v", err)
	}
	if len(booksInNew) != 1 || booksInNew[0].ID() != book.ID() {
		t.Fatalf("Expected book in gamma/beta, got %#v", booksInNew)
	}
}

func TestShelfMoveLayerRequiresExistingTarget(t *testing.T) {
	tmpLib := path.Join(t.TempDir(), "shelf_test")
	shelf := newTestShelf(t, &ShelfConf{LibRoot: tmpLib})

	if err := shelf.NewLayer([]string{"alpha"}); err != nil {
		t.Fatalf("Failed to create layer: %v", err)
	}

	if err := shelf.MoveLayer([]string{"alpha"}, []string{"missing"}); err == nil {
		t.Fatal("Expected MoveLayer to reject missing target layer")
	}
}

func TestShelfRenameLayerOldNotExist(t *testing.T) {
	tmpLib := path.Join(t.TempDir(), "shelf_test")
	shelf := newTestShelf(t, &ShelfConf{LibRoot: tmpLib})

	err := shelf.RenameLayer([]string{"nonexistent"}, []string{"anything"})
	if err == nil {
		t.Fatal("Expected error when renaming non-existent layer, got nil")
	}
}

func TestShelfRenameLayerNewAlreadyExists(t *testing.T) {
	tmpLib := path.Join(t.TempDir(), "shelf_test")
	shelf := newTestShelf(t, &ShelfConf{LibRoot: tmpLib})

	if err := shelf.NewLayer([]string{"layerA"}); err != nil {
		t.Fatalf("Failed to create layerA: %v", err)
	}
	if err := shelf.NewLayer([]string{"layerB"}); err != nil {
		t.Fatalf("Failed to create layerB: %v", err)
	}

	err := shelf.RenameLayer([]string{"layerA"}, []string{"layerB"})
	if err == nil {
		t.Fatal("Expected error when renaming to an already-existing layer, got nil")
	}
}

func TestShelfRenameLayerInvalidOldName(t *testing.T) {
	tmpLib := path.Join(t.TempDir(), "shelf_test")
	shelf := newTestShelf(t, &ShelfConf{LibRoot: tmpLib})

	err := shelf.RenameLayer([]string{"bad/name"}, []string{"newname"})
	if err == nil {
		t.Fatal("Expected error for invalid old layer name, got nil")
	}
}

func TestShelfRenameLayerInvalidNewName(t *testing.T) {
	tmpLib := path.Join(t.TempDir(), "shelf_test")
	shelf := newTestShelf(t, &ShelfConf{LibRoot: tmpLib})

	if err := shelf.NewLayer([]string{"validlayer"}); err != nil {
		t.Fatalf("Failed to create layer: %v", err)
	}

	err := shelf.RenameLayer([]string{"validlayer"}, []string{"bad/name"})
	if err == nil {
		t.Fatal("Expected error for invalid new layer name, got nil")
	}
}

func TestShelfGetBookRefreshesWhenBookMetaChangesOnDisk(t *testing.T) {
	tmpLib := path.Join(t.TempDir(), "lib")

	err := os.CopyFS(tmpLib, os.DirFS("testdata/lib"))
	if err != nil {
		t.Fatalf("Failed to copy test library: %v", err)
	}

	shelf := newTestShelf(t, &ShelfConf{LibRoot: tmpLib})

	book, err := shelf.GetBook("book-a82m")
	if err != nil {
		t.Fatalf("Failed to get book before disk update: %v", err)
	}
	if got := book.Title(); got != "Book Title" {
		t.Fatalf("Expected initial title %q, got %q", "Book Title", got)
	}

	metaPath := path.Join(tmpLib, booksFolder, "default", "test", "book-a82m.bookpkg", BookMetaFile)
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

	shelf := newTestShelf(t, &ShelfConf{LibRoot: tmpLib, ScanInterval: "0s"})

	books, err := shelf.ListBooks()
	if err != nil {
		t.Fatalf("Failed to list books before updates: %v", err)
	}
	if len(books) != 2 {
		t.Fatalf("Expected 2 books before updates, got %d", len(books))
	}

	metaPath := path.Join(tmpLib, booksFolder, "default", "test", "book-a82m.bookpkg", BookMetaFile)
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

	newBookPath := path.Join(tmpLib, booksFolder, "default", "test", "book-new.bookpkg")
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

// TestListBooksKeepsBookWithFutureSchemaVersion pins the decision that a book
// written by a newer PlainShelf build stays visible instead of disappearing.
// If openBook were ever changed to reject an unsupported schema version, the
// book would vanish from listings, be evicted from the cache, 404 from the API,
// and become impossible to restore from trash — and every other test would
// still pass. This is the regression guard for that.
func TestListBooksKeepsBookWithFutureSchemaVersion(t *testing.T) {
	tmpLib := path.Join(t.TempDir(), "lib")

	if err := os.CopyFS(tmpLib, os.DirFS("testdata/lib")); err != nil {
		t.Fatalf("Failed to copy test library: %v", err)
	}

	futureMeta, err := os.ReadFile(path.Join("testdata", "schema", "v2-future", BookMetaFile))
	if err != nil {
		t.Fatalf("Failed to read future fixture: %v", err)
	}

	futureBookPath := path.Join(tmpLib, booksFolder, "future-c3.bookpkg")
	if err := os.MkdirAll(futureBookPath, 0o755); err != nil {
		t.Fatalf("Failed to create future book directory: %v", err)
	}
	if err := os.WriteFile(path.Join(futureBookPath, BookMetaFile), futureMeta, 0o644); err != nil {
		t.Fatalf("Failed to write future book meta: %v", err)
	}

	shelf := newTestShelf(t, &ShelfConf{LibRoot: tmpLib, ScanInterval: "0s"})

	books, err := shelf.ListBooks()
	if err != nil {
		t.Fatalf("Failed to list books: %v", err)
	}

	found := false
	for _, b := range books {
		if b.ID() == "schema-v2-c3" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("Expected a book with a future schema version to remain listed, got %d books", len(books))
	}

	book, err := shelf.GetBook("schema-v2-c3")
	if err != nil {
		t.Fatalf("Expected GetBook to succeed for a future schema version, got: %v", err)
	}
	if got := book.GetMeta().SchemaVersion; got != 2 {
		t.Errorf("Expected SchemaVersion 2, got %d", got)
	}
}
