package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/voilelab/plainshelf/internal/readingprogress"
	"github.com/voilelab/plainshelf/reader/readerapi"
	"github.com/voilelab/plainshelf/shelf/bookpkg"
)

const (
	testBookID   = "dune1234"
	testSourceID = "20260315-a1"
)

// writeBookPackage builds a minimal .bookpkg the reader can open, using the same
// shelf structs the server writes so a renamed field breaks the build here rather
// than at runtime.
func writeBookPackage(t *testing.T, bookID string) string {
	t.Helper()

	dir := filepath.Join(t.TempDir(), "book.bookpkg")
	sourceDir := filepath.Join(dir, bookpkg.SourcesFolder, testSourceID)
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatalf("creating package: %v", err)
	}

	writeJSON(t, filepath.Join(dir, bookpkg.BookMetaFile), bookpkg.BookMeta{
		SchemaVersion: bookpkg.BookMetaSchemaVersion,
		ID:            bookID,
		Title:         "Dune",
		Authors:       []string{"Frank Herbert"},
		Format:        "md",
		CurrentSource: testSourceID,
	})
	writeJSON(t, filepath.Join(sourceDir, bookpkg.SourceMetaFile), bookpkg.SourceMeta{
		SchemaVersion: bookpkg.SourceMetaSchemaVersion,
		ID:            testSourceID,
		Format:        "md",
	})
	if err := os.WriteFile(filepath.Join(sourceDir, bookpkg.SourceFile), []byte("# Chapter 1\n"), 0o644); err != nil {
		t.Fatalf("writing source: %v", err)
	}

	return dir
}

func writeJSON(t *testing.T, path string, value any) {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("encoding %s: %v", path, err)
	}
	if err := os.WriteFile(path, encoded, 0o644); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
}

// newReaderAppWithBook builds a reader app with the given active shelf id, an
// open book, and progress persisted to an isolated temp file.
func newReaderAppWithBook(t *testing.T, shelfID string) *ReaderApp {
	t.Helper()

	library := readerapi.NewLibrary(nil)
	t.Cleanup(func() { library.Close() }) //nolint:errcheck // test cleanup
	if _, err := library.Open(writeBookPackage(t, testBookID)); err != nil {
		t.Fatalf("Open: %v", err)
	}

	if shelfID == "" {
		shelfID = readerapi.ShelfID
	}
	return &ReaderApp{
		library:       library,
		shelfID:       shelfID,
		progressStore: readingprogress.NewStore(filepath.Join(t.TempDir(), "reading_progress.json")),
	}
}

func progressDoc(t *testing.T, shelfID, bookID string, offset, at int64) string {
	t.Helper()
	doc := readingprogress.New()
	doc.Shelves[shelfID] = map[string]readingprogress.Entry{bookID: {Offset: offset, At: at}}
	text, err := readingprogress.Serialize(doc)
	if err != nil {
		t.Fatalf("Serialize: %v", err)
	}
	return text
}

// diskDoc reads the persisted document straight off the store's path so a test
// can assert which shelf namespace the write actually landed under.
func diskDoc(t *testing.T, app *ReaderApp) readingprogress.Document {
	t.Helper()
	doc, _, err := app.progressStore.Read()
	if err != nil {
		t.Fatalf("store read: %v", err)
	}
	return doc
}

// When launched with -shelf, the reader keys progress under the real shelf id, so
// the desktop library sees it at shelves.<realShelfID>.<bookID> with no
// projection, and a read returns exactly that namespace.
func TestReaderProgressUsesRealShelfID(t *testing.T) {
	const realShelf = "shelf-real"
	app := newReaderAppWithBook(t, realShelf)

	if err := app.WriteReadingProgress(progressDoc(t, realShelf, testBookID, 512, 1000)); err != nil {
		t.Fatalf("WriteReadingProgress: %v", err)
	}

	disk := diskDoc(t, app)
	if got := disk.Shelves[realShelf][testBookID]; got != (readingprogress.Entry{Offset: 512, At: 1000}) {
		t.Errorf("shelves[%q][%q] = %+v, want {512 1000}", realShelf, testBookID, got)
	}
	if _, ok := disk.Shelves[readerapi.ShelfID]; ok {
		t.Errorf("progress must not land under the synthetic shelf %q: %v", readerapi.ShelfID, disk.Shelves)
	}

	got, err := app.ReadReadingProgress()
	if err != nil {
		t.Fatalf("ReadReadingProgress: %v", err)
	}
	if want := progressDoc(t, realShelf, testBookID, 512, 1000); got != want {
		t.Errorf("ReadReadingProgress = %q, want %q", got, want)
	}
}

// A desktop write under another real shelf, and standalone reader progress under
// the synthetic shelf, must both survive a real-shelf reader write: it touches
// only its own book's entry.
func TestReaderRealShelfWritePreservesOtherNamespaces(t *testing.T) {
	const realShelf = "shelf-real"
	app := newReaderAppWithBook(t, realShelf)

	seed := readingprogress.New()
	seed.Shelves["shelf-other"] = map[string]readingprogress.Entry{"other-book": {Offset: 30, At: 500}}
	seed.Shelves[readerapi.ShelfID] = map[string]readingprogress.Entry{"loose-book": {Offset: 40, At: 500}}
	if _, err := app.progressStore.Mutate(func(readingprogress.Document) readingprogress.Document { return seed }); err != nil {
		t.Fatalf("seeding store: %v", err)
	}

	if err := app.WriteReadingProgress(progressDoc(t, realShelf, testBookID, 99, 1000)); err != nil {
		t.Fatalf("WriteReadingProgress: %v", err)
	}

	disk := diskDoc(t, app)
	if got := disk.Shelves[realShelf][testBookID].Offset; got != 99 {
		t.Errorf("shelves[%q][%q] = %d, want 99", realShelf, testBookID, got)
	}
	if got := disk.Shelves["shelf-other"]["other-book"].Offset; got != 30 {
		t.Errorf("desktop entry clobbered: shelves[shelf-other][other-book] = %d, want 30", got)
	}
	if got := disk.Shelves[readerapi.ShelfID]["loose-book"].Offset; got != 40 {
		t.Errorf("synthetic-shelf entry clobbered: shelves[%q][loose-book] = %d, want 40", readerapi.ShelfID, got)
	}
}

// Standalone (no -shelf) is unchanged: progress stays under the synthetic shelf
// the desktop side projects, so #451's reader -> desktop sync keeps working.
func TestReaderProgressDefaultsToSyntheticShelf(t *testing.T) {
	app := newReaderAppWithBook(t, "")
	if app.shelfID != readerapi.ShelfID {
		t.Fatalf("default shelfID = %q, want %q", app.shelfID, readerapi.ShelfID)
	}

	if err := app.WriteReadingProgress(progressDoc(t, readerapi.ShelfID, testBookID, 256, 1000)); err != nil {
		t.Fatalf("WriteReadingProgress: %v", err)
	}

	disk := diskDoc(t, app)
	if got := disk.Shelves[readerapi.ShelfID][testBookID].Offset; got != 256 {
		t.Errorf("shelves[%q][%q] = %d, want 256", readerapi.ShelfID, testBookID, got)
	}
}

// NewReaderApp falls back to the synthetic shelf id for an empty or whitespace
// -shelf, so a stray flag can never make the reader report an empty active shelf.
func TestNewReaderAppShelfIDFallback(t *testing.T) {
	for _, shelfID := range []string{"", "   "} {
		if got := NewReaderApp(nil, shelfID).shelfID; got != readerapi.ShelfID {
			t.Errorf("NewReaderApp shelfID for %q = %q, want %q", shelfID, got, readerapi.ShelfID)
		}
	}
	if got := NewReaderApp(nil, "shelf-real").shelfID; got != "shelf-real" {
		t.Errorf("NewReaderApp shelfID = %q, want shelf-real", got)
	}
}
