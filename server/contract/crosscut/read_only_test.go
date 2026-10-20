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

const (
	readOnlyBookCacheInterval = 100 * time.Millisecond
	readOnlyBookCacheWait     = 4 * readOnlyBookCacheInterval
)

type fileState struct {
	isDir   bool
	size    int64
	modTime time.Time
}

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

	t.Run("seed", func(t *testing.T) {
		env := apitest.New(t, apitest.WithLibRoot(libRoot))
		apitest.ImportTextBook(t, env, "Untouched Book", "Fiction", "untouched.txt", "Some content.")

		apitest.AssertStatus(t, env.Post(apitest.BookCacheExportURL(), nil), http.StatusOK)
	})

	before := snapshotTree(t, libRoot)

	t.Run("read", func(t *testing.T) {

		env := apitest.New(t,
			apitest.WithLibRoot(libRoot),
			apitest.WithReadOnlyServer(),
			apitest.WithBookCacheInterval(readOnlyBookCacheInterval.String()),
		)

		rec := env.Get(apitest.BooksURL())
		apitest.AssertStatus(t, rec, http.StatusOK)
		books := apitest.DecodeJSON[[]server.Book](t, rec)
		if len(books) != 1 {
			t.Fatalf("listed %d books, want the seeded one", len(books))
		}

		apitest.AssertStatus(t, env.Get(apitest.BookURL(books[0].Meta.ID)), http.StatusOK)
		apitest.AssertStatus(t, env.Get(apitest.BookURL(books[0].Meta.ID, "content")), http.StatusOK)

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

			apitest.AssertErrorEnvelope(t, rec, http.StatusConflict, "SHELF_READ_ONLY",
				"shelf is opened read-only; this PlainShelf instance cannot modify it")
			if strings.Contains(rec.Body.String(), "taskchain_id") {
				t.Errorf("body = %s, want a refusal rather than a queued chain", rec.Body.String())
			}
		})
	}

	rec := apitest.PostBookImport(t, env, apitest.BookUpload("book.txt", apitest.PlainTextContentType, "text"))
	apitest.AssertErrorEnvelope(t, rec, http.StatusConflict, "SHELF_READ_ONLY",
		"shelf is opened read-only; this PlainShelf instance cannot modify it")
}
