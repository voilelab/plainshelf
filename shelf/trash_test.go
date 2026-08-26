package shelf

import (
	"bytes"
	"encoding/json"
	"errors"
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

// A shelf written by a pre-rename build keeps its trash under the hidden
// .trash/ path. Opened writable it is migrated onto trash/; opened read-only it
// cannot be, so the reader must fall back to the legacy path instead of
// silently reporting an empty trash. "Read-only" has to mean the trash cannot
// be changed, not that it vanishes.
func TestReadOnlyShelfListsLegacyTrash(t *testing.T) {
	// The writable open of an identical fixture is the baseline: whatever it
	// lists after migrating, the read-only open must list without migrating.
	writableLib := path.Join(t.TempDir(), "writable")
	seedLegacyTrashedBook(t, writableLib, "legacy-book", "Legacy")
	writable := newTestShelf(t, &ShelfConf{LibRoot: writableLib})
	wantIDs, err := writable.ListTrashedBookIDs()
	if err != nil {
		t.Fatalf("writable ListTrashedBookIDs: %v", err)
	}
	wantBooks, err := writable.ListTrashedBooks()
	if err != nil {
		t.Fatalf("writable ListTrashedBooks: %v", err)
	}

	readOnlyLib := path.Join(t.TempDir(), "readonly")
	seedLegacyTrashedBook(t, readOnlyLib, "legacy-book", "Legacy")

	before := treeSnapshot(t, readOnlyLib)
	denyWrites(t, readOnlyLib)

	s := newTestShelf(t, &ShelfConf{LibRoot: readOnlyLib, ReadOnly: true})

	gotIDs, err := s.ListTrashedBookIDs()
	if err != nil {
		t.Fatalf("read-only ListTrashedBookIDs: %v", err)
	}
	if !slices.Equal(gotIDs, wantIDs) {
		t.Errorf("read-only ListTrashedBookIDs() = %v, want %v (same as writable)", gotIDs, wantIDs)
	}

	gotBooks, err := s.ListTrashedBooks()
	if err != nil {
		t.Fatalf("read-only ListTrashedBooks: %v", err)
	}
	if len(gotBooks) != len(wantBooks) {
		t.Fatalf("read-only ListTrashedBooks returned %d books, want %d", len(gotBooks), len(wantBooks))
	}
	for i := range gotBooks {
		if gotBooks[i].ID != wantBooks[i].ID || gotBooks[i].Title != wantBooks[i].Title {
			t.Errorf("book %d = {%q, %q}, want {%q, %q}", i, gotBooks[i].ID, gotBooks[i].Title, wantBooks[i].ID, wantBooks[i].Title)
		}
	}

	// Nothing was written: no rename onto trash/, no MkdirAll, the legacy path
	// is exactly where it was found.
	if diff := snapshotDiff(before, treeSnapshot(t, readOnlyLib)); diff != "" {
		t.Errorf("read-only open of a legacy-trash shelf wrote to it:\n%s", diff)
	}
	if _, err := os.Stat(path.Join(readOnlyLib, legacyTrashFolder)); err != nil {
		t.Errorf("legacy trash directory should be untouched: %v", err)
	}
	if _, err := os.Stat(path.Join(readOnlyLib, trashFolder)); !os.IsNotExist(err) {
		t.Errorf("read-only open created trash/: %v", err)
	}
}

// The common read-only case is an already-migrated shelf: trash/ exists, so the
// resolver settles on it and the fallback never disturbs it. Guards against a
// fallback that would read the wrong path or write to a shelf it may not.
func TestReadOnlyShelfListsMigratedTrash(t *testing.T) {
	tmpLib := path.Join(t.TempDir(), "shelf_test")

	s := newTestShelf(t, &ShelfConf{LibRoot: tmpLib})
	want := trashBook(t, s, "Trashed")
	if err := s.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	before := treeSnapshot(t, tmpLib)
	denyWrites(t, tmpLib)

	ro := newTestShelf(t, &ShelfConf{LibRoot: tmpLib, ReadOnly: true})
	ids, err := ro.ListTrashedBookIDs()
	if err != nil {
		t.Fatalf("ListTrashedBookIDs: %v", err)
	}
	if !slices.Equal(ids, []string{want}) {
		t.Errorf("ListTrashedBookIDs() = %v, want [%s]", ids, want)
	}

	if diff := snapshotDiff(before, treeSnapshot(t, tmpLib)); diff != "" {
		t.Errorf("read-only open of a migrated shelf wrote to it:\n%s", diff)
	}
}

// A trash.json edited by hand into something unparseable must not hide the book
// from the trash, nor block restoring it. Without the metadata the book cannot
// go back to its original folder, so it lands at the top level of books/.
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

// trashMetaPath returns the on-disk path of a trashed book's trash.json.
func trashMetaPath(libRoot, bookID string) string {
	return path.Join(libRoot, trashBooksFolder, bookID+bookExtension, trashMetaFile)
}

// rewriteTrashMeta replaces a trash.json with the given JSON object, returning
// the bytes now on disk so a caller can prove a refused operation left them
// alone.
func rewriteTrashMeta(t *testing.T, metaPath string, meta map[string]any) []byte {
	t.Helper()

	payload, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		t.Fatalf("marshal trash.json: %v", err)
	}
	if err := os.WriteFile(metaPath, payload, 0o644); err != nil {
		t.Fatalf("write trash.json: %v", err)
	}
	return payload
}

// readTrashMetaJSON decodes a trash.json straight from disk, so assertions see
// the file rather than the struct the shelf holds in memory.
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

// A trash.json written before versioning existed is read as v1, and reading it
// never rewrites the file: the version reaches disk only on the next write.
func TestTrashMetaWithoutSchemaVersionIsReadAsV1AndNotRewritten(t *testing.T) {
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

	// The unversioned record is still honored, so the book goes back to the
	// folder it came from rather than to the top level.
	if err := s.RestoreTrashedBook(bookID); err != nil {
		t.Fatalf("RestoreTrashedBook: %v", err)
	}
	if _, err := os.Stat(path.Join(tmpLib, booksFolder, "origin", bookID+bookExtension)); err != nil {
		t.Errorf("restored book is not in its original layer: %v", err)
	}
}

// A trash.json written by a newer build stays listed but must not be modified,
// and each refusal must happen before anything on disk moves.
func TestTrashOperationsRefuseNewerSchemaVersion(t *testing.T) {
	tmpLib := path.Join(t.TempDir(), "shelf_test")
	s := newTestShelf(t, &ShelfConf{LibRoot: tmpLib})

	bookID := trashBook(t, s, "Future Trash")
	metaPath := trashMetaPath(tmpLib, bookID)

	future := readTrashMetaJSON(t, metaPath)
	future["schema_version"] = TrashMetaSchemaVersion + 1
	future["restore_note"] = "written by a newer build"
	bumped := rewriteTrashMeta(t, metaPath, future)

	// Reading still works.
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

	// Nothing moved, nothing was rewritten.
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

// A book carried out of the trash by hand keeps the trash.json it was deleted
// with. Trashing it again must be refused before the move, so the book is not
// left sitting in trash/ under a record this build cannot write.
func TestMoveBookToTrashRefusesNewerTrashMetaOnActiveBook(t *testing.T) {
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
