package shelf

import (
	"log"
	"os"
	"path"
	"testing"
)

func TestShelfNewShelf(t *testing.T) {
	shelf, err := OpenLocalShelf(path.Join("testdata", "lib"))
	if err != nil {
		t.Fatalf("Failed to initialize Shelf: %v", err)
	}
	defer shelf.Close()
}

func TestShelfMakeStructure(t *testing.T) {
	tmpLib := path.Join(t.TempDir(), "shelf_test")
	shelf, err := OpenLocalShelf(tmpLib)
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
	shelf, err := OpenLocalShelf(path.Join("testdata", "lib"))
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
	shelf, err := OpenLocalShelf(path.Join("testdata", "lib"))
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
	shelf, err := OpenLocalShelf(path.Join("testdata", "lib"))
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
	shelf, err := OpenLocalShelf(path.Join("testdata", "lib"))
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
	shelf, err := OpenLocalShelf(path.Join("testdata", "lib"))
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
	shelf, err := OpenLocalShelf(tmpLib)
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
	shelf, err := OpenLocalShelf(tmpLib)
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
	shelf, err := OpenLocalShelf(tmpLib)
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
