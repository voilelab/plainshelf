package main

import (
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"testing"
	"time"

	"github.com/voilelab/plainshelf/internal/readingclose"
	"github.com/voilelab/plainshelf/internal/readingprogress"
)

type readerWindowEnv struct {
	app    *DesktopApp
	native *fakeNative
	store  *readingprogress.Store
	bookA  string
	bookB  string
}

func newReaderWindowEnv(t *testing.T) readerWindowEnv {
	t.Helper()
	app := startedDesktopAppWithShelf(t, "main")
	native := &fakeNative{}
	app.native = native
	store := readingprogress.NewStore(filepath.Join(t.TempDir(), "reading_progress.json"))
	app.progressStager = readingclose.NewStager(store, time.Second)

	main := mustShelf(t, app, "main")
	a, err := main.NewBook(nil, "Dune")
	if err != nil {
		t.Fatalf("NewBook: %v", err)
	}
	b, err := main.NewBook(nil, "Emma")
	if err != nil {
		t.Fatalf("NewBook: %v", err)
	}
	return readerWindowEnv{app: app, native: native, store: store, bookA: a.ID(), bookB: b.ID()}
}

func (e readerWindowEnv) open(t *testing.T, bookID string, section int) readerWindow {
	t.Helper()
	if err := e.app.OpenReaderWindow("main", bookID, section); err != nil {
		t.Fatalf("OpenReaderWindow: %v", err)
	}
	return e.native.windows[len(e.native.windows)-1]
}

func TestReaderWindowTargetsReaderRoute(t *testing.T) {
	env := newReaderWindowEnv(t)

	w := env.open(t, env.bookA, 3)

	parsed, err := url.Parse(w.url)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Path != "/reader/"+env.bookA {
		t.Fatalf("path = %q, want the reader route for %s", parsed.Path, env.bookA)
	}
	if got := parsed.Query(); got.Get("shelf") != "main" || got.Get("section") != "3" {
		t.Fatalf("query = %v, want shelf=main section=3", got)
	}
	if w.title != "Dune" {
		t.Fatalf("title = %q, want the book title", w.title)
	}
}

func TestReaderWindowOmitsNegativeSection(t *testing.T) {
	env := newReaderWindowEnv(t)

	w := env.open(t, env.bookA, -1)

	parsed, _ := url.Parse(w.url)
	if parsed.Query().Has("section") {
		t.Fatalf("url = %q, want no section", w.url)
	}
}

func TestReaderWindowNameIsPerBook(t *testing.T) {
	env := newReaderWindowEnv(t)

	first := env.open(t, env.bookA, 1)
	again := env.open(t, env.bookA, 5)
	other := env.open(t, env.bookB, -1)

	if first.name != again.name {
		t.Fatalf("same book got names %q and %q; the second open must focus the first window", first.name, again.name)
	}
	if first.name == other.name {
		t.Fatalf("two books share window name %q", first.name)
	}
}

func TestReaderWindowReuseSavesItsBook(t *testing.T) {
	env := newReaderWindowEnv(t)
	again := env.open(t, env.bookA, 2)
	env.app.StageReadingProgress("main", env.bookA, 420, 1000)
	env.app.StageReadingProgress("main", env.bookB, 77, 1000)

	again.onReuse()

	if got := diskOffset(t, env.store, "main", env.bookA); got != 420 {
		t.Fatalf("reused book offset = %d, want 420 before its window reloads", got)
	}
	if got := diskOffset(t, env.store, "main", env.bookB); got != 0 {
		t.Fatalf("other book offset = %d, want it left staged", got)
	}
}

func TestReaderWindowRejectsUnknownBook(t *testing.T) {
	env := newReaderWindowEnv(t)

	if err := env.app.OpenReaderWindow("main", "missing", -1); err == nil {
		t.Fatal("OpenReaderWindow accepted a missing book")
	}
	if err := env.app.OpenReaderWindow("nope", env.bookA, -1); err == nil {
		t.Fatal("OpenReaderWindow accepted a missing shelf")
	}
	if len(env.native.windows) != 0 {
		t.Fatalf("opened %d windows for invalid targets", len(env.native.windows))
	}
}

func TestReaderWindowCloseKeepsAppRunning(t *testing.T) {
	env := newReaderWindowEnv(t)
	a := env.open(t, env.bookA, -1)
	env.open(t, env.bookB, -1)
	env.app.StageReadingProgress("main", env.bookA, 420, 1000)
	env.app.StageReadingProgress("main", env.bookB, 77, 1000)

	a.onClose()

	if got := diskOffset(t, env.store, "main", env.bookA); got != 420 {
		t.Fatalf("closed window's offset = %d, want 420", got)
	}
	if got := diskOffset(t, env.store, "main", env.bookB); got != 0 {
		t.Fatalf("open window's offset = %d, want it left staged", got)
	}
	if env.native.quits != 0 {
		t.Fatalf("closing a reader window quit the app %d times", env.native.quits)
	}
	if !slices.Contains(env.native.events, readingDataChangedEvent) {
		t.Fatalf("events = %v, want %q so the main window refreshes", env.native.events, readingDataChangedEvent)
	}
	if _, err := env.app.ListShelfDetails(); err != nil {
		t.Fatalf("backend unusable after a reader window closed: %v", err)
	}
}

func TestMainWindowCloseQuitsWithReaders(t *testing.T) {
	env := newReaderWindowEnv(t)
	env.open(t, env.bookA, -1)
	env.open(t, env.bookB, -1)
	env.app.StageReadingProgress("main", env.bookA, 420, 1000)
	env.app.StageReadingProgress("main", env.bookB, 77, 1000)

	env.app.closeMainWindow()

	if env.native.quits != 1 {
		t.Fatalf("quits = %d, want 1 so no reader window outlives the main window", env.native.quits)
	}
	if got := diskOffset(t, env.store, "main", env.bookA); got != 420 {
		t.Fatalf("book A offset = %d, want 420", got)
	}
	if got := diskOffset(t, env.store, "main", env.bookB); got != 77 {
		t.Fatalf("book B offset = %d, want 77", got)
	}
}

func TestReadingDataEventMatchesUI(t *testing.T) {
	source, err := os.ReadFile(filepath.Join("..", "frontend", "src", "api", "desktop.ts"))
	if err != nil {
		t.Fatal(err)
	}
	match := regexp.MustCompile(`(?m)^const READING_DATA_CHANGED_EVENT = '([^']+)';$`).FindSubmatch(source)
	if len(match) != 2 {
		t.Fatal("frontend reading-data event declaration not found")
	}
	if got := string(match[1]); got != readingDataChangedEvent {
		t.Fatalf("frontend event = %q, desktop event = %q", got, readingDataChangedEvent)
	}
}
