package shelf

import (
	"errors"
	"os"
	"path"
	"slices"
	"strings"
	"testing"
)

func breakFile(t *testing.T, filePath, raw string) {
	t.Helper()

	if err := os.WriteFile(filePath, []byte(raw), 0o644); err != nil {
		t.Fatalf("Failed to write %s: %v", filePath, err)
	}
}

func TestRescanSkipsMalformedBooks(t *testing.T) {
	tmpLib := path.Join(t.TempDir(), "shelf_test")
	shelf := newTestShelf(t, &ShelfConf{LibRoot: tmpLib})

	broken, err := shelf.NewBook(FolderPath{}, "Broken")
	if err != nil {
		t.Fatalf("NewBook: %v", err)
	}
	intact, err := shelf.NewBook(FolderPath{}, "Intact")
	if err != nil {
		t.Fatalf("NewBook: %v", err)
	}
	intactID := intact.ID()

	breakFile(t, path.Join(tmpLib, broken.PackagePath(), BookMetaFile),
		`{"schema_version": 1, "title": "Broken", "title": "Broken"}`)

	shelf.markBookCacheTreeDirty()
	if _, err := shelf.RescanUnthrottled(); err != nil {
		t.Fatalf("RescanUnthrottled: %v", err)
	}

	books, err := shelf.ListBooks()
	if err != nil {
		t.Fatalf("ListBooks: %v", err)
	}

	ids := make([]string, 0, len(books))
	for _, book := range books {
		ids = append(ids, book.ID())
	}
	if !slices.Contains(ids, intactID) {
		t.Errorf("listed IDs = %q, want the intact book %q still listed", ids, intactID)
	}
	if slices.Contains(ids, broken.ID()) {
		t.Errorf("listed IDs = %q, want the unreadable book %q left out", ids, broken.ID())
	}
}

func TestTrashedBookMalformedMetaNamesFile(t *testing.T) {
	tmpLib := path.Join(t.TempDir(), "shelf_test")
	shelf := newTestShelf(t, &ShelfConf{LibRoot: tmpLib})

	book, err := shelf.NewBook(FolderPath{}, "Doomed")
	if err != nil {
		t.Fatalf("NewBook: %v", err)
	}
	bookID := book.ID()
	if err := shelf.MoveBookToTrash(bookID); err != nil {
		t.Fatalf("MoveBookToTrash: %v", err)
	}

	trashedMeta := path.Join(tmpLib, trashBooksFolder, bookID+bookExtension, BookMetaFile)
	breakFile(t, trashedMeta, `{"id": "`+bookID+`", "title": "A", "title": "B"}`)

	err = shelf.RestoreTrashedBook(bookID)
	if err == nil {
		t.Fatal("Expected restoring a book with a malformed book.json to fail, got no error")
	}
	if !errors.Is(err, ErrMalformedMetadata) {
		t.Fatalf("errors.Is(err, ErrMalformedMetadata) = false, err = %v", err)
	}

	var malformed *MalformedMetadataError
	if !errors.As(err, &malformed) {
		t.Fatalf("errors.As(err, *MalformedMetadataError) = false, err = %v", err)
	}
	if !strings.Contains(malformed.File, BookMetaFile) {
		t.Errorf("File = %q, want the book.json that could not be read", malformed.File)
	}
	if got := err.Error(); !strings.Contains(got, `"title"`) {
		t.Errorf("error does not name the duplicated member: %s", got)
	}
}
