package main

import (
	"encoding/json/v2"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/voilelab/plainshelf/internal/readingprogress"
	"github.com/voilelab/plainshelf/reader/readerapi"
	"github.com/voilelab/plainshelf/shelf/bookpkg"
)

const (
	testBookID   = "dune1234"
	testSourceID = "20260315-a1"
)

func newTestApp(t *testing.T) (*ReaderApp, *fakeNative) {
	t.Helper()
	native := &fakeNative{}
	app := &ReaderApp{
		native:        native,
		library:       readerapi.NewLibrary(nil),
		progressStore: readingprogress.NewStore(filepath.Join(t.TempDir(), "reading_progress.json")),
		closeTimeout:  time.Second,
		windows:       map[uint]*readerWindow{},
	}
	t.Cleanup(func() { app.Shutdown(t.Context()) })
	return app, native
}

func launchBook(t *testing.T, app *ReaderApp, shelfID string) uint {
	t.Helper()
	return launchBookAt(t, app, shelfID, testBookID, noLaunchSection)
}

func launchBookAt(t *testing.T, app *ReaderApp, shelfID, bookID string, section int) uint {
	t.Helper()
	win := app.Launch(launchArgs{bookPath: writeBookPackage(t, bookID), shelfID: shelfID, section: section})
	if got := app.bootConfig(win).BookID; got != bookID {
		t.Fatalf("launched window shows %q, want %q", got, bookID)
	}
	return win
}

func wantBoot(t *testing.T, app *ReaderApp, win uint, shelfID, bookID string, section int) {
	t.Helper()
	boot := app.bootConfig(win)
	gotSection := noLaunchSection
	if boot.Section != nil {
		gotSection = *boot.Section
	}
	if boot.ShelfID != shelfID || boot.BookID != bookID || gotSection != section {
		t.Fatalf("window %d boot = {%s %s %d}, want {%s %s %d}",
			win, boot.ShelfID, boot.BookID, gotSection, shelfID, bookID, section)
	}
}

func TestMenuOpenAddsWindow(t *testing.T) {
	app, native := newTestApp(t)
	first := launchBookAt(t, app, "launch-shelf", testBookID, 3)
	const nextBook = "nextbook"
	native.dir = writeBookPackage(t, nextBook)

	if id, err := app.openBookPackageIn(first); err != nil || id != nextBook {
		t.Fatalf("openBookPackageIn = %q, %v", id, err)
	}
	if len(native.created) != 2 || len(native.reloads) != 0 {
		t.Fatalf("windows %v, reloads %v; want a second window and no reload", native.created, native.reloads)
	}
	second := native.created[1]
	wantBoot(t, app, first, "launch-shelf", testBookID, 3)
	wantBoot(t, app, second, readerapi.ShelfID, nextBook, noLaunchSection)
}

func TestFailedOpenKeepsWindowState(t *testing.T) {
	app, native := newTestApp(t)
	win := launchBookAt(t, app, "launch-shelf", testBookID, 3)
	native.dir = t.TempDir()

	if _, err := app.openBookPackageIn(win); err == nil {
		t.Fatal("opened a directory without book metadata")
	}
	wantBoot(t, app, win, "launch-shelf", testBookID, 3)
	if len(native.created) != 1 {
		t.Fatalf("windows = %v, want only the original", native.created)
	}
}

func TestTwoWindowsKeepOwnBoot(t *testing.T) {
	app, native := newTestApp(t)
	desktop := launchBookAt(t, app, "shelf-real", testBookID, 2)

	if err := app.OpenFinderFile(writeBookPackage(t, "finderbook")); err != nil {
		t.Fatalf("OpenFinderFile: %v", err)
	}
	if len(native.created) != 2 {
		t.Fatalf("windows = %v, want two", native.created)
	}
	finder := native.created[1]

	wantBoot(t, app, desktop, "shelf-real", testBookID, 2)
	wantBoot(t, app, finder, readerapi.ShelfID, "finderbook", noLaunchSection)
	if !app.library.Has(testBookID) || !app.library.Has("finderbook") {
		t.Fatal("library does not hold both books")
	}
}

func TestBootForReadsWindowHeader(t *testing.T) {
	app, _ := newTestApp(t)
	launchBook(t, app, "")
	second := launchBookAt(t, app, "shelf-real", "nextbook", 1)

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set(windowIDHeader, strconv.FormatUint(uint64(second), 10))
	if boot := app.BootFor(request); boot.BookID != "nextbook" || boot.ShelfID != "shelf-real" {
		t.Fatalf("BootFor(window %d) = %+v, want nextbook on shelf-real", second, boot)
	}

	unknown := httptest.NewRequest(http.MethodGet, "/", nil)
	unknown.Header.Set(windowIDHeader, "99")
	if boot := app.BootFor(unknown); boot.BookID != "" || boot.ShelfID != readerapi.ShelfID || boot.Section != nil {
		t.Fatalf("BootFor(unknown) = %+v, want an empty standalone boot", boot)
	}
}

func TestLaunchShelfIDFallback(t *testing.T) {
	app, _ := newTestApp(t)
	for shelfID, want := range map[string]string{"": readerapi.ShelfID, "   ": readerapi.ShelfID, "shelf-real": "shelf-real"} {
		win := app.Launch(launchArgs{shelfID: shelfID, section: noLaunchSection})
		if got := app.bootConfig(win).ShelfID; got != want {
			t.Errorf("Launch shelfID for %q = %q, want %q", shelfID, got, want)
		}
	}
}

func TestBootConfigSection(t *testing.T) {
	app, _ := newTestApp(t)
	for _, section := range []int{noLaunchSection, 0, 3} {
		win := app.Launch(launchArgs{shelfID: "shelf-real", section: section})
		wantBoot(t, app, win, "shelf-real", "", section)
	}
}

func TestLaunchBadBookOpensEmptyWindow(t *testing.T) {
	app, native := newTestApp(t)
	win := app.Launch(launchArgs{bookPath: t.TempDir(), section: noLaunchSection})

	wantBoot(t, app, win, readerapi.ShelfID, "", noLaunchSection)
	if len(native.created) != 1 {
		t.Fatalf("windows = %v, want one", native.created)
	}
}

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

func diskDoc(t *testing.T, app *ReaderApp) readingprogress.Document {
	t.Helper()
	doc, _, err := app.progressStore.Read()
	if err != nil {
		t.Fatalf("store read: %v", err)
	}
	return doc
}

func TestReaderProgressUsesRealShelfID(t *testing.T) {
	const realShelf = "shelf-real"
	app, _ := newTestApp(t)
	win := launchBook(t, app, realShelf)

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

	got, err := app.ReadReadingProgress(windowContext(t, win))
	if err != nil {
		t.Fatalf("ReadReadingProgress: %v", err)
	}
	if want := progressDoc(t, realShelf, testBookID, 512, 1000); got != want {
		t.Errorf("ReadReadingProgress = %q, want %q", got, want)
	}
}

func TestReadProgressPerWindowShelf(t *testing.T) {
	app, _ := newTestApp(t)
	desktop := launchBook(t, app, "shelf-real")
	finder := launchBookAt(t, app, "", "finderbook", noLaunchSection)
	for _, doc := range []string{
		progressDoc(t, "shelf-real", testBookID, 10, 1000),
		progressDoc(t, readerapi.ShelfID, "finderbook", 20, 1000),
	} {
		if err := app.WriteReadingProgress(doc); err != nil {
			t.Fatal(err)
		}
	}

	for win, want := range map[uint]string{
		desktop: progressDoc(t, "shelf-real", testBookID, 10, 1000),
		finder:  progressDoc(t, readerapi.ShelfID, "finderbook", 20, 1000),
	} {
		got, err := app.readProgressIn(win)
		if err != nil || got != want {
			t.Errorf("readProgressIn(%d) = %q, %v; want %q", win, got, err, want)
		}
	}
}

func TestRealShelfWriteKeepsOtherNamespaces(t *testing.T) {
	const realShelf = "shelf-real"
	app, _ := newTestApp(t)
	launchBook(t, app, realShelf)

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

func TestWindowOfReadsBindingContext(t *testing.T) {
	if got := windowOf(windowContext(t, 7)); got != 7 {
		t.Errorf("windowOf = %d, want 7", got)
	}
	if got := windowOf(t.Context()); got != 0 {
		t.Errorf("windowOf without a window = %d, want 0", got)
	}
}
