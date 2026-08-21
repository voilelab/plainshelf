package contract_test

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/voilelab/plainshelf/server"
)

/*
Reader mode is what the standalone reading binary (cmd/plainshelf-read) runs as.
It is not "read-only with a nicer name": read-only refuses the requests that ask
for a write, while reader mode decides which routes exist at all. These tests
pin both halves of that — the reading surface it does serve, and the library
surface it does not — because the frontend gates its pages on the mode this
reports, and a route quietly moved between the two groups would strand a page on
a 404.
*/

// seedReaderShelf fills a shelf with a writable server and returns the one book
// it created, so the reader-mode tests below have something real to read. The
// seeding app is closed with the subtest, before any reader app opens the shelf.
func seedReaderShelf(t *testing.T, libRoot string) server.Book {
	t.Helper()

	var seeded server.Book
	t.Run("seed", func(t *testing.T) {
		env := newAPITestEnv(t, withLibRoot(libRoot))
		seeded = importTextBook(t, env, "Reader Book", "Fiction", "reader.txt", "Some content.")
	})
	return seeded
}

// A reader server says so, and says it is read-only, without being configured
// read-only: the mode implies it.
func TestAPIReaderModeReportsItselfContract(t *testing.T) {
	env := newAPITestEnv(t, withReaderMode())

	mode := getJSON[modeResponse](t, env, "/api/mode")
	if mode.Mode != "reader" {
		t.Errorf("mode = %q, want %q", mode.Mode, "reader")
	}
	if !mode.ReadOnly {
		t.Error("read_only = false, want reader mode to imply it")
	}
}

// An ordinary server names its mode rather than leaving the field to be
// inferred from absence.
func TestAPIFullModeReportsItselfContract(t *testing.T) {
	env := newAPITestEnv(t)

	mode := getJSON[modeResponse](t, env, "/api/mode")
	if mode.Mode != "full" {
		t.Errorf("mode = %q, want %q", mode.Mode, "full")
	}
	if mode.ReadOnly {
		t.Error("read_only = true, want a plain server to be writable")
	}
}

// Everything a reading client asks for is served. This is the whole API surface
// of the reader binary; adding to it is a deliberate act, and this list is where
// that shows up.
func TestAPIReaderModeServesTheReadingRoutesContract(t *testing.T) {
	libRoot := t.TempDir()
	seeded := seedReaderShelf(t, libRoot)

	env := newAPITestEnv(t, withLibRoot(libRoot), withReaderMode())

	bookID := seeded.Meta.ID
	sourceID := env.currentSourceID(t, bookID)

	for _, url := range []string{
		"/health",
		"/api/mode",
		"/api/version",
		"/api/shelves",
		shelfURL("status"),
		booksURL(),
		bookURL(bookID),
		bookURL(bookID, "content"),
		bookURL(bookID, "sources"),
		sourceURL(bookID, sourceID),
		sourceURL(bookID, sourceID, "content"),
		shelfURL("layers"),
	} {
		t.Run(url, func(t *testing.T) {
			assertStatus(t, env.get(url), http.StatusOK)
		})
	}

	// The cover and the asset routes are mounted too. The seeded book has
	// neither, so what is asserted is that the request reaches the handler and
	// is answered as a missing file rather than as a missing route: the mux
	// answers an unmounted /api path with Go's own "404 page not found".
	for _, url := range []string{
		bookURL(bookID, "cover"),
		assetURL(bookID, sourceID, "missing.png"),
	} {
		t.Run(url, func(t *testing.T) {
			rec := env.get(url)
			assertStatus(t, rec, http.StatusNotFound)
			if body := rec.Body.String(); body == "404 page not found\n" {
				t.Errorf("body = %q, want the handler's own answer rather than an unmounted route", body)
			}
		})
	}
}

// Running a library is not reading one. None of these is a write, and reader
// mode still does not serve them.
func TestAPIReaderModeMountsNoLibraryRoutesContract(t *testing.T) {
	libRoot := t.TempDir()
	seeded := seedReaderShelf(t, libRoot)

	env := newAPITestEnv(t, withLibRoot(libRoot), withReaderMode())

	for _, url := range []string{
		"/api/logs",
		"/api/logs/some-log/content",
		"/api/setting/cover_to_jpg",
		"/api/setting/epub_import_strategy",
		"/api/taskchains/some-chain",
		shelfURL("trash", "books"),
	} {
		t.Run(url, func(t *testing.T) {
			rec := env.get(url)
			assertStatus(t, rec, http.StatusNotFound)

			// The unknown-API-path fallback answers, not the SPA index: a page
			// that asked for one of these must see a 404, not an HTML document
			// to parse as JSON.
			if body := rec.Body.String(); body != "404 page not found\n" {
				t.Errorf("body = %q, want the unmounted-route answer", body)
			}
		})
	}

	// The duplicate scan is asserted apart from the list above because it is not
	// answered by the unmounted-path fallback: /books/duplicate also matches the
	// /books/{book_id} pattern reader mode does mount, so the book handler
	// answers it — as a book that does not exist, which is the point.
	duplicates := env.get(shelfURL("books", "duplicate"))
	assertStatus(t, duplicates, http.StatusNotFound)

	// None of this is about the routes being broken: the same requests on a full
	// server are served.
	full := newAPITestEnv(t, withLibRoot(libRoot))
	assertStatus(t, full.get(shelfURL("trash", "books")), http.StatusOK)
	assertStatus(t, full.get(shelfURL("books", "duplicate")), http.StatusOK)
	assertStatus(t, full.get(bookURL(seeded.Meta.ID)), http.StatusOK)
}

// Writes are refused before routing, by the read-only gate reader mode implies,
// so the answer is a refusal rather than a missing route.
func TestAPIReaderModeRefusesWritesContract(t *testing.T) {
	libRoot := t.TempDir()
	seeded := seedReaderShelf(t, libRoot)

	env := newAPITestEnv(t, withLibRoot(libRoot), withReaderMode())

	assertStatus(t, env.patch(bookURL(seeded.Meta.ID), nil), http.StatusForbidden)
	assertStatus(t, env.delete(bookURL(seeded.Meta.ID)), http.StatusForbidden)
	assertStatus(t, env.post(shelfURL("layers", "NewLayer"), nil), http.StatusForbidden)
	assertStatus(t, env.post(shelfURL("trash", "empty"), nil), http.StatusForbidden)
}

// The acceptance case: a reader serves a round of reads and leaves the shelf
// byte for byte and mtime for mtime as it found it — the same promise the
// read-only server makes, reached through the mode instead of the flag.
func TestReaderModeLeavesTheShelfUntouched(t *testing.T) {
	libRoot := t.TempDir()
	seeded := seedReaderShelf(t, libRoot)

	before := snapshotTree(t, libRoot)

	t.Run("read", func(t *testing.T) {
		env := newAPITestEnv(t, withLibRoot(libRoot), withReaderMode())

		assertStatus(t, env.get(booksURL()), http.StatusOK)
		assertStatus(t, env.get(bookURL(seeded.Meta.ID)), http.StatusOK)
		assertStatus(t, env.get(bookURL(seeded.Meta.ID, "content")), http.StatusOK)
		assertStatus(t, env.get(shelfURL("layers")), http.StatusOK)
	})

	assertTreeUnchanged(t, before, snapshotTree(t, libRoot))
}

// A reader has nowhere of its own to write either: the settings store is kept in
// memory, so the configured store path is never created.
func TestReaderModeKeepsItsStoreInMemory(t *testing.T) {
	storePath := filepath.Join(t.TempDir(), "store")

	app, err := server.NewApp(apiAppConf(t, withStorePath(storePath), withReaderMode()))
	if err != nil {
		t.Fatalf("NewApp: %v", err)
	}
	t.Cleanup(func() {
		if err := app.Close(); err != nil {
			t.Fatalf("Close app: %v", err)
		}
	})

	if _, err := os.Stat(storePath); !os.IsNotExist(err) {
		t.Errorf("stat %s = %v, want the store path never created", storePath, err)
	}
}

// An unknown mode is refused at startup rather than silently serving the full
// API, which is what a typo in a config file would otherwise get.
func TestUnknownServerModeIsRefused(t *testing.T) {
	conf := apiAppConf(t)
	conf.Mode = server.ServerMode("readonly")

	app, err := server.NewApp(conf)
	if err == nil {
		app.Close()
		t.Fatal("NewApp succeeded on an unknown mode, want a failure")
	}
}
