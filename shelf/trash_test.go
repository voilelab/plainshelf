package shelf

import (
	"os"
	"path"
	"slices"
	"testing"
)

// trashBook creates a book and moves it to the trash, returning its ID.
func trashBook(t *testing.T, s *Shelf, title string) string {
	t.Helper()

	book, err := s.NewBook(nil, title)
	if err != nil {
		t.Fatalf("NewBook(%q): %v", title, err)
	}
	if err := s.MoveBookToTrash(book.ID()); err != nil {
		t.Fatalf("MoveBookToTrash(%q): %v", book.ID(), err)
	}
	return book.ID()
}

func TestListTrashedBookIDsReturnsEmptyWhenTrashIsEmpty(t *testing.T) {
	s := newTestShelf(t, &ShelfConf{LibRoot: path.Join(t.TempDir(), "shelf_test")})

	ids, err := s.ListTrashedBookIDs()
	if err != nil {
		t.Fatalf("ListTrashedBookIDs: %v", err)
	}
	if len(ids) != 0 {
		t.Errorf("Expected no IDs, got %v", ids)
	}
}

func TestListTrashedBookIDsReturnsEveryTrashedBook(t *testing.T) {
	s := newTestShelf(t, &ShelfConf{LibRoot: path.Join(t.TempDir(), "shelf_test")})

	want := []string{trashBook(t, s, "First"), trashBook(t, s, "Second")}
	slices.Sort(want)

	ids, err := s.ListTrashedBookIDs()
	if err != nil {
		t.Fatalf("ListTrashedBookIDs: %v", err)
	}
	if !slices.Equal(ids, want) {
		t.Errorf("ListTrashedBookIDs() = %v, want %v", ids, want)
	}
}

// A book whose metadata cannot be read is skipped by ListTrashedBooks, so it
// never reaches the UI. Emptying the trash must still delete it instead of
// leaving an invisible directory behind.
func TestListTrashedBookIDsIncludesUnreadableBooks(t *testing.T) {
	tmpLib := path.Join(t.TempDir(), "shelf_test")
	s := newTestShelf(t, &ShelfConf{LibRoot: tmpLib})

	healthy := trashBook(t, s, "Healthy")
	broken := trashBook(t, s, "Broken")

	metaPath := path.Join(tmpLib, trashBooksFolder, broken+bookExtension, BookMetaFile)
	if err := os.WriteFile(metaPath, []byte("not json"), 0o600); err != nil {
		t.Fatalf("corrupt book meta: %v", err)
	}

	listed, err := s.ListTrashedBooks()
	if err != nil {
		t.Fatalf("ListTrashedBooks: %v", err)
	}
	if len(listed) != 1 || listed[0].ID != healthy {
		t.Fatalf("Expected ListTrashedBooks to skip the unreadable book, got %d entries", len(listed))
	}

	want := []string{healthy, broken}
	slices.Sort(want)

	ids, err := s.ListTrashedBookIDs()
	if err != nil {
		t.Fatalf("ListTrashedBookIDs: %v", err)
	}
	if !slices.Equal(ids, want) {
		t.Errorf("ListTrashedBookIDs() = %v, want %v including the unreadable book", ids, want)
	}
}
