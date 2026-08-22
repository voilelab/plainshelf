package main

import (
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
	return doc.Shelves[shelfKey][bookID]
}

// A reader progress entry is projected onto the real shelf that holds the book,
// and the projection is persisted so a second read is a no-op.
func TestProjectStoredReaderProgress_FoldsAndPersists(t *testing.T) {
	store := readingprogress.NewStore(filepath.Join(t.TempDir(), "reading_progress.json"))
	seedStore(t, store, func(doc readingprogress.Document) readingprogress.Document {
		out := doc.Clone()
		out.Shelves[readingprogress.ReaderShelfID] = map[string]int64{"book-a": 120}
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
	if v := got.Shelves["real-shelf"]["book-a"]; v != 120 {
		t.Fatalf("returned real-shelf offset = %d, want 120", v)
	}
	// Persisted, and the reader namespace is kept.
	if v := readerOffset(t, store, "real-shelf", "book-a"); v != 120 {
		t.Fatalf("persisted real-shelf offset = %d, want 120", v)
	}
	if v := readerOffset(t, store, readingprogress.ReaderShelfID, "book-a"); v != 120 {
		t.Fatalf("reader entry offset = %d, want 120 (must be kept)", v)
	}
}

// An unresolved book (no shelf holds it) is not projected and stays under the
// reader namespace; the read returns the stored document untouched.
func TestProjectStoredReaderProgress_UnresolvedStays(t *testing.T) {
	store := readingprogress.NewStore(filepath.Join(t.TempDir(), "reading_progress.json"))
	seedStore(t, store, func(doc readingprogress.Document) readingprogress.Document {
		out := doc.Clone()
		out.Shelves[readingprogress.ReaderShelfID] = map[string]int64{"book-loose": 30}
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
	if v := doc.Shelves[readingprogress.ReaderShelfID]["book-loose"]; v != 30 {
		t.Fatalf("loose reader entry = %d, want 30", v)
	}
}

// A desktop write preserves the reader's namespace even when the desktop's
// in-memory copy of it is stale.
func TestWriteReadingProgress_PreservesReaderNamespace(t *testing.T) {
	store := readingprogress.NewStore(filepath.Join(t.TempDir(), "reading_progress.json"))
	seedStore(t, store, func(doc readingprogress.Document) readingprogress.Document {
		out := doc.Clone()
		out.Shelves[readingprogress.ReaderShelfID] = map[string]int64{"book-a": 999}
		return out
	})

	app := &DesktopApp{readingProgressSync: store}
	// A desktop-side write that advances a real shelf and carries a stale reader
	// entry it happened to read earlier.
	if err := app.WriteReadingProgress(`{"version":1,"shelves":{"real-shelf":{"book-a":40},"book":{"book-a":1}}}`); err != nil {
		t.Fatalf("write: %v", err)
	}

	if v := readerOffset(t, store, "real-shelf", "book-a"); v != 40 {
		t.Fatalf("desktop real-shelf offset = %d, want 40", v)
	}
	if v := readerOffset(t, store, readingprogress.ReaderShelfID, "book-a"); v != 999 {
		t.Fatalf("reader namespace offset = %d, want 999 (must be preserved)", v)
	}
}

// A corrupt write is rejected rather than lenient-parsed into an empty document
// (which would wipe stored progress).
func TestWriteReadingProgress_RejectsCorruptJSON(t *testing.T) {
	store := readingprogress.NewStore(filepath.Join(t.TempDir(), "reading_progress.json"))
	seedStore(t, store, func(doc readingprogress.Document) readingprogress.Document {
		out := doc.Clone()
		out.Shelves["real-shelf"] = map[string]int64{"book-a": 50}
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
