package crosscut_test

import (
	"io"
	"io/fs"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/voilelab/plainshelf/server/contract/apitest"

	"github.com/voilelab/plainshelf/server"
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
That the setting reaches ShelfConf, which is what turns those writes off as
well, is pinned in server/app_readonly_test.go: it asks nothing of the router,
so it is not a contract test. What is pinned here is the part that needs the
router — a read-only server serving a whole round of reads without touching the
shelf, and the per-shelf setting's HTTP side.
*/

// readOnlyBookCacheWait is how long TestReadOnlyServerLeavesTheShelfUntouched
// lets the export timer run, and readOnlyBookCacheInterval is how often it
// ticks while it does. Four ticks, because one is what the assertion needs and
// the rest is margin for a loaded runner.
const (
	readOnlyBookCacheInterval = "100ms"
	readOnlyBookCacheWait     = 400 * time.Millisecond
)

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
		// The export timer is the one write with no request behind it, so the
		// read-only run has to outlive at least one tick for the snapshot below
		// to mean anything. There is nothing to poll for — the assertion is that
		// nothing appears — so the interval is shortened and the wait is a small
		// multiple of it rather than a fixed two seconds.
		env := apitest.New(t,
			apitest.WithLibRoot(libRoot),
			apitest.WithReadOnlyServer(),
			apitest.WithBookCacheInterval(readOnlyBookCacheInterval),
		)

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

		time.Sleep(readOnlyBookCacheWait)
	})

	assertTreeUnchanged(t, before, snapshotTree(t, libRoot))
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
