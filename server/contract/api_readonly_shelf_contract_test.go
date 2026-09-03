package contract_test

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

// A shelf opened with read_only serves reads normally.
func TestAPIReadOnlyShelfServesReadsContract(t *testing.T) {
	env := newAPITestEnv(t, withReadOnlyShelf())

	assertStatus(t, env.get(booksURL()), http.StatusOK)
	assertStatus(t, env.get(shelfURL("folders")), http.StatusOK)
	assertStatus(t, env.get(shelfURL("trash", "books")), http.StatusOK)

	// A rescan walks the shelf and rebuilds the in-memory cache without writing
	// anything, so it is a read even though it is a POST.
	assertStatus(t, env.post(shelfURL("scans"), nil), http.StatusOK)
}

// Every write against a read-only shelf is refused with 409, including the ones
// that would otherwise queue a background chain and answer 202 — a caller that
// got 202 would have to read a task report to learn the work never happened.
func TestAPIReadOnlyShelfRefusesWritesContract(t *testing.T) {
	env := newAPITestEnv(t, withReadOnlyShelf())

	tests := []struct {
		name string
		url  string
		body string
	}{
		{name: "book batch", url: shelfURL("book-batches"),
			body: `{"operation":"trash","book_ids":["book-0001"]}`},
		{name: "content stat refresh", url: shelfURL("content-stat-refreshes")},
		{name: "source fingerprints", url: shelfURL("source-fingerprints")},
		{name: "empty trash", url: shelfURL("trash", "empty")},
		{name: "book cache export", url: bookCacheExportURL()},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var body io.Reader
			if tc.body != "" {
				body = strings.NewReader(tc.body)
			}

			rec := env.post(tc.url, body)

			// 409 is also how an endpoint reports a chain already in flight, and
			// that answer carries the chain's ID. Nothing was queued here, so the
			// two must be told apart by body shape rather than by status: a
			// client that reads taskchain_id off this refusal gets nothing and
			// polls it.
			assertErrorEnvelope(t, rec, http.StatusConflict, "SHELF_READ_ONLY",
				"shelf is opened read-only; this PlainShelf instance cannot modify it")
			if strings.Contains(rec.Body.String(), "taskchain_id") {
				t.Errorf("body = %s, want a refusal rather than a queued chain", rec.Body.String())
			}
		})
	}

	// The synchronous write path reaches the shelf itself, so this 409 arrives
	// from fsutil.ErrReadOnly through the error table rather than from the
	// handler's own gate.
	rec := postBookImport(t, env, bookUpload("book.txt", plainTextContentType, "text"))
	assertErrorEnvelope(t, rec, http.StatusConflict, "SHELF_READ_ONLY",
		"shelf is opened read-only; this PlainShelf instance cannot modify it")
}
