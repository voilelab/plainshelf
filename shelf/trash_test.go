package shelf

import (
	"fmt"
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

// seedLegacyTrashedBook writes a book directory under the pre-rename hidden
// trash path, the way an older build would have left it.
func seedLegacyTrashedBook(t *testing.T, libRoot, bookID, title string) {
	t.Helper()

	bookPath := path.Join(libRoot, legacyTrashBooksFolder, bookID+bookExtension)
	if err := os.MkdirAll(bookPath, 0o755); err != nil {
		t.Fatalf("MkdirAll(%q): %v", bookPath, err)
	}
	meta := fmt.Sprintf(`{"id":%q,"title":%q}`, bookID, title)
	if err := os.WriteFile(path.Join(bookPath, BookMetaFile), []byte(meta), 0o644); err != nil {
		t.Fatalf("write book.json: %v", err)
	}
	if err := os.WriteFile(path.Join(bookPath, trashMetaFile), []byte(`{"delete_reason":"user"}`), 0o644); err != nil {
		t.Fatalf("write trash.json: %v", err)
	}
}

func TestOpenShelfRenamesLegacyTrash(t *testing.T) {
	tmpLib := path.Join(t.TempDir(), "shelf_test")
	seedLegacyTrashedBook(t, tmpLib, "legacy-book", "Legacy")

	s := newTestShelf(t, &ShelfConf{LibRoot: tmpLib})

	ids, err := s.ListTrashedBookIDs()
	if err != nil {
		t.Fatalf("ListTrashedBookIDs: %v", err)
	}
	if !slices.Equal(ids, []string{"legacy-book"}) {
		t.Errorf("ListTrashedBookIDs() = %v, want [legacy-book]", ids)
	}

	if _, err := os.Stat(path.Join(tmpLib, legacyTrashFolder)); !os.IsNotExist(err) {
		t.Errorf("legacy trash directory still present: %v", err)
	}
}

// A shelf opened by an older build again after the rename ends up with both
// directories. Every book must survive the merge.
func TestOpenShelfMergesLegacyTrashIntoRenamedTrash(t *testing.T) {
	tmpLib := path.Join(t.TempDir(), "shelf_test")

	s := newTestShelf(t, &ShelfConf{LibRoot: tmpLib})
	current := trashBook(t, s, "Current")
	if err := s.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	seedLegacyTrashedBook(t, tmpLib, "legacy-book", "Legacy")

	reopened := newTestShelf(t, &ShelfConf{LibRoot: tmpLib})
	ids, err := reopened.ListTrashedBookIDs()
	if err != nil {
		t.Fatalf("ListTrashedBookIDs: %v", err)
	}
	want := []string{current, "legacy-book"}
	slices.Sort(want)
	if !slices.Equal(ids, want) {
		t.Errorf("ListTrashedBookIDs() = %v, want %v", ids, want)
	}

	if _, err := os.Stat(path.Join(tmpLib, legacyTrashFolder)); !os.IsNotExist(err) {
		t.Errorf("legacy trash directory still present: %v", err)
	}
}

// Two entries for the same book ID cannot share a folder name. Neither may be
// dropped, so the legacy one is kept beside the current one.
func TestOpenShelfKeepsBothWhenLegacyTrashCollides(t *testing.T) {
	tmpLib := path.Join(t.TempDir(), "shelf_test")

	s := newTestShelf(t, &ShelfConf{LibRoot: tmpLib})
	bookID := trashBook(t, s, "Current")
	if err := s.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	seedLegacyTrashedBook(t, tmpLib, bookID, "Legacy")

	newTestShelf(t, &ShelfConf{LibRoot: tmpLib})

	for _, folder := range []string{bookID + bookExtension, bookID + "-1" + bookExtension} {
		if _, err := os.Stat(path.Join(tmpLib, trashBooksFolder, folder)); err != nil {
			t.Errorf("expected %q under the trash: %v", folder, err)
		}
	}
	if _, err := os.Stat(path.Join(tmpLib, legacyTrashFolder)); !os.IsNotExist(err) {
		t.Errorf("legacy trash directory still present: %v", err)
	}
}

// A whole-directory rename carries anything else the user kept there along
// with the books, so nothing under the legacy path is left behind.
func TestOpenShelfRenameCarriesUnknownLegacyTrashContent(t *testing.T) {
	tmpLib := path.Join(t.TempDir(), "shelf_test")
	seedLegacyTrashedBook(t, tmpLib, "legacy-book", "Legacy")

	if err := os.WriteFile(path.Join(tmpLib, legacyTrashBooksFolder, "notes.txt"), []byte("mine"), 0o644); err != nil {
		t.Fatalf("write stray file: %v", err)
	}

	newTestShelf(t, &ShelfConf{LibRoot: tmpLib})

	if _, err := os.Stat(path.Join(tmpLib, trashBooksFolder, "notes.txt")); err != nil {
		t.Errorf("stray file did not survive the rename: %v", err)
	}
}

// When the two directories are merged the books move one by one, and a file
// the user put there by hand is not one of them. It is never deleted, so the
// directory holding it stays as well.
func TestOpenShelfKeepsUnknownLegacyTrashContentOnMerge(t *testing.T) {
	tmpLib := path.Join(t.TempDir(), "shelf_test")

	s := newTestShelf(t, &ShelfConf{LibRoot: tmpLib})
	current := trashBook(t, s, "Current")
	if err := s.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	seedLegacyTrashedBook(t, tmpLib, "legacy-book", "Legacy")
	strayPath := path.Join(tmpLib, legacyTrashBooksFolder, "notes.txt")
	if err := os.WriteFile(strayPath, []byte("mine"), 0o644); err != nil {
		t.Fatalf("write stray file: %v", err)
	}

	reopened := newTestShelf(t, &ShelfConf{LibRoot: tmpLib})
	ids, err := reopened.ListTrashedBookIDs()
	if err != nil {
		t.Fatalf("ListTrashedBookIDs: %v", err)
	}
	want := []string{current, "legacy-book"}
	slices.Sort(want)
	if !slices.Equal(ids, want) {
		t.Errorf("ListTrashedBookIDs() = %v, want %v", ids, want)
	}
	if _, err := os.Stat(strayPath); err != nil {
		t.Errorf("stray file was removed: %v", err)
	}
}
