package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/voilelab/plainshelf/internal/readingprogress"
)

func seedStore(t *testing.T, store *readingprogress.Store, mutate func(readingprogress.Document) readingprogress.Document) {
	t.Helper()
	if _, err := store.Mutate(mutate); err != nil {
		t.Fatalf("seed store: %v", err)
	}
}

func readerOffset(t *testing.T, store *readingprogress.Store, shelfKey, bookID string) int64 {
	t.Helper()
	doc, _, err := store.Read()
	if err != nil {
		t.Fatalf("read store: %v", err)
	}
	return doc.Shelves[shelfKey][bookID].Offset
}

func TestProjectProgressFoldsAndPersists(t *testing.T) {
	store := readingprogress.NewStore(filepath.Join(t.TempDir(), "reading_progress.json"))
	seedStore(t, store, func(doc readingprogress.Document) readingprogress.Document {
		out := doc.Clone()
		out.Shelves[readingprogress.ReaderShelfID] = map[string]readingprogress.Entry{"book-a": {Offset: 120, At: 1000}}
		return out
	})

	resolve := func(bookID string) (string, bool) {
		if bookID == "book-a" {
			return "real-shelf", true
		}
		return "", false
	}

	text, err := projectStoredReaderProgress(store, resolve)
	if err != nil {
		t.Fatalf("project: %v", err)
	}

	got := readingprogress.Parse(text)
	if v := got.Shelves["real-shelf"]["book-a"].Offset; v != 120 {
		t.Fatalf("returned real-shelf offset = %d, want 120", v)
	}

	if v := readerOffset(t, store, "real-shelf", "book-a"); v != 120 {
		t.Fatalf("persisted real-shelf offset = %d, want 120", v)
	}
	if v := readerOffset(t, store, readingprogress.ReaderShelfID, "book-a"); v != 120 {
		t.Fatalf("reader entry offset = %d, want 120 (must be kept)", v)
	}
}

func TestProjectProgressUnresolvedStays(t *testing.T) {
	store := readingprogress.NewStore(filepath.Join(t.TempDir(), "reading_progress.json"))
	seedStore(t, store, func(doc readingprogress.Document) readingprogress.Document {
		out := doc.Clone()
		out.Shelves[readingprogress.ReaderShelfID] = map[string]readingprogress.Entry{"book-loose": {Offset: 30, At: 1000}}
		return out
	})

	unresolved := func(string) (string, bool) { return "", false }
	if _, err := projectStoredReaderProgress(store, unresolved); err != nil {
		t.Fatalf("project: %v", err)
	}

	doc, _, err := store.Read()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(doc.Shelves) != 1 {
		t.Fatalf("unresolved book must not create a real-shelf entry; shelves = %v", doc.Shelves)
	}
	if v := doc.Shelves[readingprogress.ReaderShelfID]["book-loose"].Offset; v != 30 {
		t.Fatalf("loose reader entry = %d, want 30", v)
	}
}

func TestWriteProgressPreservesNewerEntry(t *testing.T) {
	store := readingprogress.NewStore(filepath.Join(t.TempDir(), "reading_progress.json"))
	seedStore(t, store, func(doc readingprogress.Document) readingprogress.Document {
		out := doc.Clone()

		out.Shelves[readingprogress.ReaderShelfID] = map[string]readingprogress.Entry{"book-a": {Offset: 999, At: 2000}}
		out.Shelves["real-shelf"] = map[string]readingprogress.Entry{"book-a": {Offset: 200, At: 1000}}
		return out
	})

	app := &DesktopApp{readingProgressSync: store}

	if err := app.WriteReadingProgress(
		`{"version":2,"shelves":{"real-shelf":{"book-a":{"offset":260,"at":3000}},"book":{"book-a":{"offset":1,"at":500}}}}`,
	); err != nil {
		t.Fatalf("write: %v", err)
	}

	if v := readerOffset(t, store, "real-shelf", "book-a"); v != 260 {
		t.Fatalf("desktop real-shelf offset = %d, want 260 (newer desktop write wins)", v)
	}
	if v := readerOffset(t, store, readingprogress.ReaderShelfID, "book-a"); v != 999 {
		t.Fatalf("reader namespace offset = %d, want 999 (stale desktop copy must not clobber it)", v)
	}
}

func TestShelfIDReservesReaderNamespace(t *testing.T) {
	if id := generateDesktopShelfID("book", map[string]bool{}); id == readingprogress.ReaderShelfID {
		t.Fatalf("shelf id %q collides with the reader namespace", id)
	}
	if id := generateDesktopShelfID("Book", map[string]bool{}); id == readingprogress.ReaderShelfID {
		t.Fatalf("shelf id %q collides with the reader namespace", id)
	}

	if id := generateDesktopShelfID("Comics", map[string]bool{}); id != "comics" {
		t.Fatalf("unexpected shelf id for Comics: %q", id)
	}
}

func TestWriteProgressRejectsCorruptJSON(t *testing.T) {
	store := readingprogress.NewStore(filepath.Join(t.TempDir(), "reading_progress.json"))
	seedStore(t, store, func(doc readingprogress.Document) readingprogress.Document {
		out := doc.Clone()
		out.Shelves["real-shelf"] = map[string]readingprogress.Entry{"book-a": {Offset: 50, At: 1000}}
		return out
	})

	app := &DesktopApp{readingProgressSync: store}
	if err := app.WriteReadingProgress("{not json"); err == nil {
		t.Fatalf("expected corrupt JSON to be rejected")
	}
	if v := readerOffset(t, store, "real-shelf", "book-a"); v != 50 {
		t.Fatalf("stored progress must survive a rejected write, got %d", v)
	}
}

func TestReadProgressSkipsRealReaderShelf(t *testing.T) {
	desktopApp := startedDesktopAppWithShelves(t, readingprogress.ReaderShelfID, "other")
	other, ok := desktopApp.app.ShelfManager().GetShelf("other")
	if !ok {
		t.Fatal("shelf other not found")
	}
	book, err := other.NewBook(nil, "Dune")
	if err != nil {
		t.Fatalf("NewBook: %v", err)
	}

	desktopApp.readingProgressPath = filepath.Join(t.TempDir(), "reading_progress.json")
	desktopApp.readingProgressSync = readingprogress.NewStore(desktopApp.readingProgressPath)
	seedStore(t, desktopApp.readingProgressSync, func(doc readingprogress.Document) readingprogress.Document {
		out := doc.Clone()
		out.Shelves[readingprogress.ReaderShelfID] = map[string]readingprogress.Entry{book.ID(): {Offset: 42, At: 1000}}
		return out
	})

	text, err := desktopApp.ReadReadingProgress()
	if err != nil {
		t.Fatalf("ReadReadingProgress: %v", err)
	}
	if _, projected := readingprogress.Parse(text).Shelves["other"][book.ID()]; projected {
		t.Errorf("progress under the real %q shelf was projected onto other: %s", readingprogress.ReaderShelfID, text)
	}
	if _, persisted := readingprogress.Parse(mustReadFile(t, desktopApp.readingProgressPath)).Shelves["other"][book.ID()]; persisted {
		t.Error("the projection was written to disk")
	}
}

func mustReadFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}
