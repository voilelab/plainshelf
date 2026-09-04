package contract_test

import (
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/voilelab/plainshelf/server/contract/apitest"

	"github.com/voilelab/plainshelf/server"
	"github.com/voilelab/plainshelf/shelf"
)

/*
Read-only comes in two scopes: the app-wide `read_only` an operator writes next
to `shelves`, and the per-shelf `read_only` that covers one shelf. They differ
in reach, not in what they forbid on the shelf they cover, so both are pinned
here.

The app-wide one is not only an HTTP gate. Refusing the requests that ask for a
write is not the same as not writing: a shelf writes on its own account too — it
creates its folders, clears app/tmp/, takes the lock file and exports the book
cache on a timer — and none of that has a request behind it for the gate to see.
The first half of this file pins that the setting reaches ShelfConf, which is
what turns those writes off as well; the second half pins the HTTP side of the
per-shelf setting.
*/

// fileState is what a snapshot records about one path in the shelf. Modification
// time is the assertion the acceptance test is really about; size and directory
// flag are carried so a rewrite that lands in the same second still shows up.
type fileState struct {
	isDir   bool
	size    int64
	modTime time.Time
}

// snapshotTree records every path under root, so two snapshots can be compared
// to prove a run of the server touched nothing.
func snapshotTree(t *testing.T, root string) map[string]fileState {
	t.Helper()

	states := map[string]fileState{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		states[rel] = fileState{isDir: entry.IsDir(), size: info.Size(), modTime: info.ModTime()}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	return states
}

// assertTreeUnchanged reports every path that was added, removed or written to.
func assertTreeUnchanged(t *testing.T, before, after map[string]fileState) {
	t.Helper()

	for rel, was := range before {
		is, ok := after[rel]
		if !ok {
			t.Errorf("%s was removed", rel)
			continue
		}
		if is.modTime != was.modTime || is.size != was.size || is.isDir != was.isDir {
			t.Errorf("%s changed: %+v then %+v", rel, was, is)
		}
	}
	for rel := range after {
		if _, ok := before[rel]; !ok {
			t.Errorf("%s was created", rel)
		}
	}
}

// A read-only server opens every shelf read-only, including one added after
// startup through the desktop "add shelf" flow.
func TestReadOnlyServerOpensEveryShelfReadOnly(t *testing.T) {
	env := apitest.New(t, apitest.WithReadOnlyServer())

	shelfData, ok := env.App.ShelfManager().GetShelf(apitest.DefaultShelfID)
	if !ok {
		t.Fatalf("%s missing", apitest.DefaultShelfID)
	}
	if !shelfData.ReadOnly() {
		t.Error("configured shelf ReadOnly() = false, want the app-wide read_only to reach it")
	}

	addedRoot := t.TempDir()
	if err := env.App.AddShelf(shelf.ShelfConfWithID{
		ID:        "added_later",
		Name:      "Added Later",
		ShelfConf: shelf.ShelfConf{LibRoot: addedRoot},
	}); err != nil {
		t.Fatalf("AddShelf: %v", err)
	}

	added, ok := env.App.ShelfManager().GetShelf("added_later")
	if !ok {
		t.Fatal("added_later missing from the shelf manager")
	}
	if !added.ReadOnly() {
		t.Error("added shelf ReadOnly() = false, want the app-wide read_only to reach it too")
	}

	// The writer ID is what enables the exported book cache, and exporting is a
	// write. A read-only server must not hand one out, so the export is refused
	// as read-only rather than reported as unconfigured.
	if _, err := added.ExportBookCache(); err == nil {
		t.Error("ExportBookCache on a read-only server succeeded, want a refusal")
	}
}

// The acceptance case: a read-only server serves a whole round of reads — list
// the books, read a book's content, rescan — and leaves the shelf byte for byte
// and mtime for mtime as it found it.
func TestReadOnlyServerLeavesTheShelfUntouched(t *testing.T) {
	libRoot := t.TempDir()

	// Seed the shelf with a writable server, in a subtest so its app is closed —
	// and has flushed its own exported cache — before the snapshot is taken.
	t.Run("seed", func(t *testing.T) {
		env := apitest.New(t, apitest.WithLibRoot(libRoot))
		apitest.ImportTextBook(t, env, "Untouched Book", "Fiction", "untouched.txt", "Some content.")

		// Force the export rather than waiting out the interval, so the file the
		// read-only run must not rewrite or prune is already on disk.
		apitest.AssertStatus(t, env.Post(apitest.BookCacheExportURL(), nil), http.StatusOK)
	})

	before := snapshotTree(t, libRoot)

	t.Run("read", func(t *testing.T) {
		env := apitest.New(t, apitest.WithLibRoot(libRoot), apitest.WithReadOnlyServer())

		rec := env.Get(apitest.BooksURL())
		apitest.AssertStatus(t, rec, http.StatusOK)
		books := apitest.DecodeJSON[[]server.Book](t, rec)
		if len(books) != 1 {
			t.Fatalf("listed %d books, want the seeded one", len(books))
		}

		apitest.AssertStatus(t, env.Get(apitest.BookURL(books[0].Meta.ID)), http.StatusOK)
		apitest.AssertStatus(t, env.Get(apitest.BookURL(books[0].Meta.ID, "content")), http.StatusOK)

		// A rescan is a read despite its method, so read-only mode lets it
		// through. It is the one POST that does, and it must stay one.
		apitest.AssertStatus(t, env.Post(apitest.ShelfURL("scans"), nil), http.StatusOK)

		// The book cache export runs on a timer as well as on demand; give the
		// interval configured for contract tests time to come round.
		time.Sleep(2 * time.Second)
	})

	assertTreeUnchanged(t, before, snapshotTree(t, libRoot))
}

// A read-only server must not create the shelf either — the same promise
// ShelfConf.ReadOnly makes, reached through the app-wide setting.
func TestReadOnlyServerDoesNotCreateTheShelf(t *testing.T) {
	libRoot := filepath.Join(t.TempDir(), "missing-shelf")

	app, err := server.NewApp(apitest.AppConf(t, apitest.WithLibRoot(libRoot), apitest.WithReadOnlyServer()))
	if err == nil {
		app.Close()
		t.Fatal("NewApp succeeded on a missing lib_root, want a failure rather than a created shelf")
	}
	if _, statErr := os.Stat(libRoot); !os.IsNotExist(statErr) {
		t.Errorf("stat %s = %v, want the path still missing", libRoot, statErr)
	}
}

// A shelf opened with read_only serves reads normally.
func TestAPIReadOnlyShelfServesReadsContract(t *testing.T) {
	env := apitest.New(t, apitest.WithReadOnlyShelf())

	apitest.AssertStatus(t, env.Get(apitest.BooksURL()), http.StatusOK)
	apitest.AssertStatus(t, env.Get(apitest.ShelfURL("folders")), http.StatusOK)
	apitest.AssertStatus(t, env.Get(apitest.ShelfURL("trash", "books")), http.StatusOK)

	// A rescan walks the shelf and rebuilds the in-memory cache without writing
	// anything, so it is a read even though it is a POST.
	apitest.AssertStatus(t, env.Post(apitest.ShelfURL("scans"), nil), http.StatusOK)
}

// Every write against a read-only shelf is refused with 409, including the ones
// that would otherwise queue a background chain and answer 202 — a caller that
// got 202 would have to read a task report to learn the work never happened.
func TestAPIReadOnlyShelfRefusesWritesContract(t *testing.T) {
	env := apitest.New(t, apitest.WithReadOnlyShelf())

	tests := []struct {
		name string
		url  string
		body string
	}{
		{name: "book batch", url: apitest.ShelfURL("book-batches"),
			body: `{"operation":"trash","book_ids":["book-0001"]}`},
		{name: "content stat refresh", url: apitest.ShelfURL("content-stat-refreshes")},
		{name: "source fingerprints", url: apitest.ShelfURL("source-fingerprints")},
		{name: "empty trash", url: apitest.ShelfURL("trash", "empty")},
		{name: "book cache export", url: apitest.BookCacheExportURL()},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var body io.Reader
			if tc.body != "" {
				body = strings.NewReader(tc.body)
			}

			rec := env.Post(tc.url, body)

			// 409 is also how an endpoint reports a chain already in flight, and
			// that answer carries the chain's ID. Nothing was queued here, so the
			// two must be told apart by body shape rather than by status: a
			// client that reads taskchain_id off this refusal gets nothing and
			// polls it.
			apitest.AssertErrorEnvelope(t, rec, http.StatusConflict, "SHELF_READ_ONLY",
				"shelf is opened read-only; this PlainShelf instance cannot modify it")
			if strings.Contains(rec.Body.String(), "taskchain_id") {
				t.Errorf("body = %s, want a refusal rather than a queued chain", rec.Body.String())
			}
		})
	}

	// The synchronous write path reaches the shelf itself, so this 409 arrives
	// from fsutil.ErrReadOnly through the error table rather than from the
	// handler's own gate.
	rec := apitest.PostBookImport(t, env, apitest.BookUpload("book.txt", apitest.PlainTextContentType, "text"))
	apitest.AssertErrorEnvelope(t, rec, http.StatusConflict, "SHELF_READ_ONLY",
		"shelf is opened read-only; this PlainShelf instance cannot modify it")
}
