package shelf

import (
	"bytes"
	"encoding/json/v2"
	"errors"
	"fmt"
	"os"
	"path"
	"slices"
	"strings"
	"testing"

	"github.com/voilelab/plainshelf/internal/jsonopt"
)

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

func TestTrashedIDsEmptyTrash(t *testing.T) {
	s := newTestShelf(t, &ShelfConf{LibRoot: path.Join(t.TempDir(), "shelf_test")})

	ids, err := s.ListTrashedBookIDs()
	if err != nil {
		t.Fatalf("ListTrashedBookIDs: %v", err)
	}
	if len(ids) != 0 {
		t.Errorf("Expected no IDs, got %v", ids)
	}
}

func TestTrashedIDsListsEveryBook(t *testing.T) {
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

func TestTrashedIDsIncludeUnreadable(t *testing.T) {
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

func TestLegacyTrashMergesIntoTrash(t *testing.T) {
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

func TestLegacyTrashCollisionKeepsBoth(t *testing.T) {
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

func TestLegacyTrashRenameKeepsUnknownFiles(t *testing.T) {
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

func TestLegacyTrashMergeKeepsUnknown(t *testing.T) {
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

func TestTrashToleratesUnreadableTrashMeta(t *testing.T) {
	tmpLib := path.Join(t.TempDir(), "shelf_test")
	s := newTestShelf(t, &ShelfConf{LibRoot: tmpLib})

	book, err := s.NewBook(FolderPath{"layer"}, "Corrupted")
	if err != nil {
		t.Fatalf("NewBook: %v", err)
	}
	bookID := book.ID()
	if err := s.MoveBookToTrash(bookID); err != nil {
		t.Fatalf("MoveBookToTrash: %v", err)
	}

	metaPath := path.Join(tmpLib, trashBooksFolder, bookID+bookExtension, trashMetaFile)
	if err := os.WriteFile(metaPath, []byte("{not json"), 0o644); err != nil {
		t.Fatalf("corrupt trash.json: %v", err)
	}

	items, err := s.ListTrashedBooks()
	if err != nil {
		t.Fatalf("ListTrashedBooks: %v", err)
	}
	if len(items) != 1 || items[0].ID != bookID {
		t.Fatalf("ListTrashedBooks() = %+v, want the book with ID %q", items, bookID)
	}

	if err := s.RestoreTrashedBook(bookID); err != nil {
		t.Fatalf("RestoreTrashedBook: %v", err)
	}
	if _, err := os.Stat(path.Join(tmpLib, booksFolder, bookID+bookExtension)); err != nil {
		t.Errorf("restored book is not at the top level of books/: %v", err)
	}
}

func trashMetaPath(libRoot, bookID string) string {
	return path.Join(libRoot, trashBooksFolder, bookID+bookExtension, trashMetaFile)
}

func rewriteTrashMeta(t *testing.T, metaPath string, meta map[string]any) []byte {
	t.Helper()

	payload, err := json.Marshal(meta, jsonopt.Disk())
	if err != nil {
		t.Fatalf("marshal trash.json: %v", err)
	}
	if err := os.WriteFile(metaPath, payload, 0o644); err != nil {
		t.Fatalf("write trash.json: %v", err)
	}
	return payload
}

func readTrashMetaJSON(t *testing.T, metaPath string) map[string]any {
	t.Helper()

	raw, err := os.ReadFile(metaPath)
	if err != nil {
		t.Fatalf("read trash.json: %v", err)
	}
	var meta map[string]any
	if err := json.Unmarshal(raw, &meta); err != nil {
		t.Fatalf("unmarshal trash.json: %v", err)
	}
	return meta
}

func TestMoveBookToTrashStampsSchemaVersion(t *testing.T) {
	tmpLib := path.Join(t.TempDir(), "shelf_test")
	s := newTestShelf(t, &ShelfConf{LibRoot: tmpLib})

	bookID := trashBook(t, s, "Versioned")

	meta := readTrashMetaJSON(t, trashMetaPath(tmpLib, bookID))
	version, ok := meta["schema_version"].(float64)
	if !ok {
		t.Fatalf("trash.json has no schema_version: %#v", meta)
	}
	if version != float64(TrashMetaSchemaVersion) {
		t.Errorf("schema_version = %v, want %d", version, TrashMetaSchemaVersion)
	}
}

func TestTrashWritesFolderNamesLiterally(t *testing.T) {
	tmpLib := path.Join(t.TempDir(), "shelf_test")
	s := newTestShelf(t, &ShelfConf{LibRoot: tmpLib})

	const folder = "R&D <drafts>"
	book, err := s.NewBook(FolderPath{folder}, "Report")
	if err != nil {
		t.Fatalf("NewBook: %v", err)
	}
	bookID := book.ID()
	if err := s.MoveBookToTrash(bookID); err != nil {
		t.Fatalf("MoveBookToTrash: %v", err)
	}

	raw, err := os.ReadFile(trashMetaPath(tmpLib, bookID))
	if err != nil {
		t.Fatalf("read trash.json: %v", err)
	}
	if !strings.Contains(string(raw), folder) {
		t.Errorf("trash.json does not carry %q literally:\n%s", folder, raw)
	}
	for _, escape := range []string{`\u0026`, `\u003c`, `\u003e`} {
		if strings.Contains(string(raw), escape) {
			t.Errorf("trash.json still escapes %s:\n%s", escape, raw)
		}
	}

	meta := readTrashMetaJSON(t, trashMetaPath(tmpLib, bookID))
	original, ok := meta["original_folder"].([]any)
	if !ok || len(original) != 1 || original[0] != folder {
		t.Errorf("trash.json original_folder = %#v, want [%q]", meta["original_folder"], folder)
	}
}

func TestLegacyTrashMetaReadWithoutRewrite(t *testing.T) {
	tmpLib := path.Join(t.TempDir(), "shelf_test")
	s := newTestShelf(t, &ShelfConf{LibRoot: tmpLib})

	bookID := trashBook(t, s, "Legacy Trash")
	metaPath := trashMetaPath(tmpLib, bookID)

	legacy := rewriteTrashMeta(t, metaPath, map[string]any{
		"original_path":   path.Join(booksFolder, "origin", bookID+bookExtension),
		"original_folder": []string{"origin"},
		"delete_reason":   "user",
	})

	items, err := s.ListTrashedBooks()
	if err != nil {
		t.Fatalf("ListTrashedBooks: %v", err)
	}
	if len(items) != 1 || items[0].ID != bookID {
		t.Fatalf("ListTrashedBooks() = %+v, want the book with ID %q", items, bookID)
	}

	after, err := os.ReadFile(metaPath)
	if err != nil {
		t.Fatalf("re-read trash.json: %v", err)
	}
	if !bytes.Equal(legacy, after) {
		t.Fatalf("listing the trash rewrote trash.json, got:\n%s", after)
	}

	if err := s.RestoreTrashedBook(bookID); err != nil {
		t.Fatalf("RestoreTrashedBook: %v", err)
	}
	if _, err := os.Stat(path.Join(tmpLib, booksFolder, "origin", bookID+bookExtension)); err != nil {
		t.Errorf("restored book is not in its original layer: %v", err)
	}
}

func TestTrashRefusesNewerSchema(t *testing.T) {
	tmpLib := path.Join(t.TempDir(), "shelf_test")
	s := newTestShelf(t, &ShelfConf{LibRoot: tmpLib})

	bookID := trashBook(t, s, "Future Trash")
	metaPath := trashMetaPath(tmpLib, bookID)

	future := readTrashMetaJSON(t, metaPath)
	future["schema_version"] = TrashMetaSchemaVersion + 1
	future["restore_note"] = "written by a newer build"
	bumped := rewriteTrashMeta(t, metaPath, future)

	items, err := s.ListTrashedBooks()
	if err != nil {
		t.Fatalf("ListTrashedBooks: %v", err)
	}
	if len(items) != 1 || items[0].ID != bookID {
		t.Fatalf("ListTrashedBooks() = %+v, want the book with ID %q", items, bookID)
	}

	if err := s.RestoreTrashedBook(bookID); !errors.Is(err, ErrUnsupportedTrashSchemaVersion) {
		t.Fatalf("RestoreTrashedBook error = %v, want ErrUnsupportedTrashSchemaVersion", err)
	}
	if err := s.DeleteTrashedBook(bookID); !errors.Is(err, ErrUnsupportedTrashSchemaVersion) {
		t.Fatalf("DeleteTrashedBook error = %v, want ErrUnsupportedTrashSchemaVersion", err)
	}

	if _, err := os.Stat(path.Join(tmpLib, trashBooksFolder, bookID+bookExtension)); err != nil {
		t.Fatalf("refused operations must leave the book in the trash: %v", err)
	}
	after, err := os.ReadFile(metaPath)
	if err != nil {
		t.Fatalf("re-read trash.json: %v", err)
	}
	if !bytes.Equal(bumped, after) {
		t.Fatalf("refused operations must leave trash.json untouched, got:\n%s", after)
	}
}

func TestMoveBookRefusesFutureTrashMeta(t *testing.T) {
	tmpLib := path.Join(t.TempDir(), "shelf_test")
	s := newTestShelf(t, &ShelfConf{LibRoot: tmpLib})

	book, err := s.NewBook(FolderPath{"layer"}, "Carried Back")
	if err != nil {
		t.Fatalf("NewBook: %v", err)
	}
	bookID := book.ID()

	activePath := path.Join(tmpLib, book.PackagePath())
	rewriteTrashMeta(t, path.Join(activePath, trashMetaFile), map[string]any{
		"schema_version": TrashMetaSchemaVersion + 1,
	})

	if err := s.MoveBookToTrash(bookID); !errors.Is(err, ErrUnsupportedTrashSchemaVersion) {
		t.Fatalf("MoveBookToTrash error = %v, want ErrUnsupportedTrashSchemaVersion", err)
	}

	if _, err := os.Stat(activePath); err != nil {
		t.Fatalf("refused trashing must leave the book where it was: %v", err)
	}
	if _, err := os.Stat(path.Join(tmpLib, trashBooksFolder, bookID+bookExtension)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("refused trashing must not leave a book in the trash, stat err = %v", err)
	}
}
