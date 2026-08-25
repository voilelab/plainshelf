package shelf

import (
	"errors"
	"io"
	"os"
	"path"
	"strings"
	"sync"
	"testing"

	"github.com/voilelab/plainshelf/shelf/bookpkg"
)

var errInitFailed = errors.New("init failed")

// bookPkgEntries lists the book packages directly under dir, tolerating a dir
// that was never created.
func bookPkgEntries(t *testing.T, dir string) []string {
	t.Helper()

	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		t.Fatalf("ReadDir(%q): %v", dir, err)
	}

	var found []string
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), bookExtension) {
			found = append(found, entry.Name())
		}
	}
	return found
}

// A book whose initialization fails must leave nothing behind. Previously the
// book folder was renamed into place first and the source was created after, so
// a failure stranded a sourceless book in the library forever.
func TestShelfNewBookWithRollsBackOnInitFailure(t *testing.T) {
	tmpLib := path.Join(t.TempDir(), "shelf_test")
	shelf := newTestShelf(t, &ShelfConf{LibRoot: tmpLib})

	_, err := shelf.NewBookWith(FolderPath{"rollback"}, "Doomed Book", func(book *Book) error {
		if _, err := book.NewSource(nil); err != nil {
			t.Fatalf("NewSource on staged book: %v", err)
		}
		return errInitFailed
	})
	if !errors.Is(err, errInitFailed) {
		t.Fatalf("NewBookWith error = %v, want %v", err, errInitFailed)
	}

	if found := bookPkgEntries(t, path.Join(tmpLib, booksFolder, "rollback")); len(found) != 0 {
		t.Errorf("books left under the layer after a failed init: %v", found)
	}

	staged, err := os.ReadDir(path.Join(tmpLib, appFolder, appTmpFolder))
	if err != nil {
		t.Fatalf("ReadDir app tmp: %v", err)
	}
	if len(staged) != 0 {
		names := make([]string, 0, len(staged))
		for _, entry := range staged {
			names = append(names, entry.Name())
		}
		t.Errorf("staging folder not cleaned after a failed init: %v", names)
	}

	books, err := shelf.ListBooks()
	if err != nil {
		t.Fatalf("ListBooks: %v", err)
	}
	for _, book := range books {
		if book.Title() == "Doomed Book" {
			t.Errorf("failed book %q is visible in the library", book.Title())
		}
	}
}

// Everything the initializer writes must survive the move out of staging.
func TestShelfNewBookWithPersistsInitWrites(t *testing.T) {
	tmpLib := path.Join(t.TempDir(), "shelf_test")
	shelf := newTestShelf(t, &ShelfConf{LibRoot: tmpLib})

	const content = "staged content\n"

	book, err := shelf.NewBookWith(FolderPath{"staged"}, "Staged Book", func(book *Book) error {
		source, err := book.NewSource(strings.NewReader(content))
		if err != nil {
			return err
		}
		if err := book.SetCurrentSource(source.ID()); err != nil {
			return err
		}

		meta := book.GetMeta()
		meta.Language = "en"
		meta.Format = "txt"
		return book.SetMeta(meta)
	})
	if err != nil {
		t.Fatalf("NewBookWith: %v", err)
	}

	// Re-read through the shelf so the assertions run against what is on disk,
	// not the instance the initializer mutated.
	reopened, err := shelf.GetBook(book.ID())
	if err != nil {
		t.Fatalf("GetBook: %v", err)
	}

	meta := reopened.GetMeta()
	if meta.Language != "en" {
		t.Errorf("language = %q, want %q", meta.Language, "en")
	}
	if meta.Format != "txt" {
		t.Errorf("format = %q, want %q", meta.Format, "txt")
	}
	if meta.CurrentSource == "" {
		t.Fatal("current source is empty, want the source created during init")
	}

	source, err := reopened.GetSource(meta.CurrentSource)
	if err != nil {
		t.Fatalf("GetSource(%q): %v", meta.CurrentSource, err)
	}
	reader, err := source.Open()
	if err != nil {
		t.Fatalf("source.Open: %v", err)
	}
	defer reader.Close()

	got, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read source: %v", err)
	}
	if string(got) != content {
		t.Errorf("source content = %q, want %q", string(got), content)
	}
}

// While the initializer runs, the book must not yet exist at its final path.
// The final path is checked directly rather than through ListBooks, because the
// exclusive shelf lock is still held here.
func TestShelfNewBookWithBookNotVisibleDuringInit(t *testing.T) {
	tmpLib := path.Join(t.TempDir(), "shelf_test")
	shelf := newTestShelf(t, &ShelfConf{LibRoot: tmpLib})

	_, err := shelf.NewBookWith(FolderPath{"hidden"}, "Hidden Book", func(book *Book) error {
		if found := bookPkgEntries(t, path.Join(tmpLib, booksFolder, "hidden")); len(found) != 0 {
			t.Errorf("book is already visible under the layer during init: %v", found)
		}
		if !strings.HasPrefix(book.PackagePath(), path.Join(appFolder, appTmpFolder)) {
			t.Errorf("staged book folder = %q, want it under the app temp folder", book.PackagePath())
		}
		return nil
	})
	if err != nil {
		t.Fatalf("NewBookWith: %v", err)
	}

	if found := bookPkgEntries(t, path.Join(tmpLib, booksFolder, "hidden")); len(found) != 1 {
		t.Errorf("books under the layer after init = %v, want exactly one", found)
	}
}

// Concurrent metadata writes to one book used to race on a shared
// "book.json.tmp": one writer's rename would consume the file the other was
// still staging. Each write now goes through its own temp file.
//
// Each goroutine gets its own Book handle: sharing one instance would exercise
// the unsynchronized in-memory meta rather than the on-disk temp file, which is
// what this test is about.
func TestBookSetMetaConcurrentWritersDoNotCollide(t *testing.T) {
	tmpLib := path.Join(t.TempDir(), "shelf_test")
	shelf := newTestShelf(t, &ShelfConf{LibRoot: tmpLib})

	created, err := shelf.NewBook(FolderPath{"concurrent"}, "Concurrent Book")
	if err != nil {
		t.Fatalf("NewBook: %v", err)
	}
	bookPath := created.PackagePath()

	const writers = 12

	books := make([]*Book, writers)
	for i := range books {
		book, err := bookpkg.Open(shelf.dbRoot, newLoggerForTest(), bookPath)
		if err != nil {
			t.Fatalf("Open %d: %v", i, err)
		}
		books[i] = book
	}

	var wg sync.WaitGroup
	errs := make([]error, writers)
	for i := range writers {
		wg.Go(func() {

			meta := books[i].GetMeta()
			meta.Comments = strings.Repeat("x", i+1)
			errs[i] = books[i].SetMeta(meta)
		})
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Errorf("writer %d: %v", i, err)
		}
	}

	// The surviving book.json must be one complete write, not a mix of several.
	reopened, err := bookpkg.Open(shelf.dbRoot, newLoggerForTest(), bookPath)
	if err != nil {
		t.Fatalf("Open after writes: %v", err)
	}
	if got := reopened.GetMeta().Comments; len(got) < 1 || len(got) > writers || strings.Trim(got, "x") != "" {
		t.Errorf("comments = %q, want a whole run of 1..%d 'x' characters", got, writers)
	}

	assertNoTempFiles(t, path.Join(tmpLib, bookPath))
}

// Two books that agree on everything the old MD5 seed derived an ID from - the
// folders and the title - must still get different IDs. The collision probe that
// used to make that true only sees what this process already has in its cache,
// so a shared shelf or a book copied in with a file manager slipped past it.
func TestShelfNewBookIDsAreRandomNotDerived(t *testing.T) {
	tmpLib := path.Join(t.TempDir(), "shelf_test")
	shelf := newTestShelf(t, &ShelfConf{LibRoot: tmpLib})

	first, err := shelf.NewBook(FolderPath{"same"}, "Same Title")
	if err != nil {
		t.Fatalf("NewBook: %v", err)
	}
	second, err := shelf.NewBook(FolderPath{"same"}, "Same Title")
	if err != nil {
		t.Fatalf("NewBook: %v", err)
	}

	if first.ID() == second.ID() {
		t.Fatalf("two books with the same layers and title share the ID %q", first.ID())
	}

	for _, id := range []string{first.ID(), second.ID()} {
		if err := validateBookID(id); err != nil {
			t.Errorf("generated ID %q is not usable as one: %v", id, err)
		}
		// This build writes a canonical, lowercase v4 UUID: 36 characters in the
		// 8-4-4-4-12 hex layout, version nibble 4, variant 10xx (8, 9, a, or b).
		if len(id) != 36 {
			t.Errorf("generated ID %q has length %d, want 36", id, len(id))
		}
		if strings.Trim(id, "0123456789abcdef-") != "" {
			t.Errorf("generated ID %q contains characters outside the UUID alphabet", id)
		}
		if id[14] != '4' {
			t.Errorf("generated ID %q is not a version 4 UUID (version nibble %q)", id, id[14])
		}
		if !strings.ContainsRune("89ab", rune(id[19])) {
			t.Errorf("generated ID %q has a non-RFC-4122 variant (variant nibble %q)", id, id[19])
		}
	}
}

// The ID is opaque: nothing it was once derived from may move it. Reading
// progress, bookmarks, and every device-local document are keyed on it, so a
// recomputation would silently orphan all of them.
func TestShelfBookIDSurvivesTitleFolderAndTrashRoundTrip(t *testing.T) {
	tmpLib := path.Join(t.TempDir(), "shelf_test")
	shelf := newTestShelf(t, &ShelfConf{LibRoot: tmpLib})

	book, err := shelf.NewBook(FolderPath{"origin"}, "Original Title")
	if err != nil {
		t.Fatalf("NewBook: %v", err)
	}
	bookID := book.ID()

	meta := book.GetMeta()
	meta.Title = "Renamed Title"
	if err := book.SetMeta(meta); err != nil {
		t.Fatalf("SetMeta: %v", err)
	}
	if book.ID() != bookID {
		t.Fatalf("renaming the title changed the ID: %q -> %q", bookID, book.ID())
	}

	moved, err := shelf.MoveBook(bookID, FolderPath{"elsewhere"})
	if err != nil {
		t.Fatalf("MoveBook: %v", err)
	}
	if moved.ID() != bookID {
		t.Fatalf("moving the book changed the ID: %q -> %q", bookID, moved.ID())
	}

	if err := shelf.MoveBookToTrash(bookID); err != nil {
		t.Fatalf("MoveBookToTrash: %v", err)
	}
	if err := shelf.RestoreTrashedBook(bookID); err != nil {
		t.Fatalf("RestoreTrashedBook: %v", err)
	}

	restored, err := shelf.GetBook(bookID)
	if err != nil {
		t.Fatalf("GetBook after restore: %v", err)
	}
	if restored.ID() != bookID {
		t.Fatalf("the trash round trip changed the ID: %q -> %q", bookID, restored.ID())
	}
}

// A shelf written by an older build keeps its 8-character hex IDs untouched, and
// a book created next to them gets a random one. Both forms have to work at once
// - nothing migrates the old ones.
func TestShelfLegacyAndRandomBookIDsCoexist(t *testing.T) {
	tmpLib := path.Join(t.TempDir(), "shelf_test")
	legacyID := "a1b2c3d4"

	legacyDir := path.Join(tmpLib, booksFolder, "legacy", legacyID+bookExtension)
	if err := os.MkdirAll(legacyDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	legacyMeta := `{"schema_version":1,"id":"` + legacyID + `","title":"Legacy Book"}`
	if err := os.WriteFile(path.Join(legacyDir, BookMetaFile), []byte(legacyMeta), 0o644); err != nil {
		t.Fatalf("WriteFile book.json: %v", err)
	}

	shelf := newTestShelf(t, &ShelfConf{LibRoot: tmpLib})

	legacy, err := shelf.GetBook(legacyID)
	if err != nil {
		t.Fatalf("GetBook(%q): %v", legacyID, err)
	}
	if legacy.ID() != legacyID {
		t.Fatalf("legacy ID was rewritten: %q -> %q", legacyID, legacy.ID())
	}

	fresh, err := shelf.NewBook(FolderPath{"legacy"}, "Legacy Book")
	if err != nil {
		t.Fatalf("NewBook: %v", err)
	}
	if fresh.ID() == legacyID {
		t.Fatalf("a new book reused the legacy ID %q", legacyID)
	}

	// Both are addressable, and writing through one leaves the other alone.
	freshMeta := fresh.GetMeta()
	freshMeta.Title = "Fresh Book"
	if err := fresh.SetMeta(freshMeta); err != nil {
		t.Fatalf("SetMeta on the new book: %v", err)
	}
	reopened, err := shelf.GetBook(legacyID)
	if err != nil {
		t.Fatalf("GetBook(%q) after writing the new book: %v", legacyID, err)
	}
	if reopened.Title() != "Legacy Book" {
		t.Errorf("legacy book title = %q, want %q", reopened.Title(), "Legacy Book")
	}
}
