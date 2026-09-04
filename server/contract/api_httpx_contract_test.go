package contract_test

import (
	"net/http"
	"strings"
	"testing"
)

// Every shelf-scoped route resolves its shelf through the same helper, so an
// unknown ID must be one answer rather than one per endpoint. Reads and the
// routes that would otherwise queue background work are listed together: a
// caller that got 202 for a shelf that does not exist would poll a chain that
// never ran.
func TestAPIUnknownShelfIsRejectedConsistentlyContract(t *testing.T) {
	env := New(t)

	requests := []struct {
		method string
		path   string
	}{
		{http.MethodGet, ShelfIDURL("missing_shelf", "books")},
		{http.MethodGet, ShelfIDURL("missing_shelf", "status")},
		{http.MethodGet, ShelfIDURL("missing_shelf", "folders")},
		{http.MethodGet, ShelfIDURL("missing_shelf", "trash", "books")},
		{http.MethodGet, ShelfIDURL("missing_shelf", "books", "duplicate")},
		{http.MethodGet, ShelfIDURL("missing_shelf", "books", "similar")},
		{http.MethodGet, ShelfIDURL("missing_shelf", "fingerprints", "status")},
		{http.MethodPost, ShelfIDURL("missing_shelf", "scans")},
		{http.MethodPost, ShelfIDURL("missing_shelf", "book-cache-exports")},
		{http.MethodPost, ShelfIDURL("missing_shelf", "content-stat-refreshes")},
		{http.MethodPost, ShelfIDURL("missing_shelf", "source-fingerprints")},
		{http.MethodPost, ShelfIDURL("missing_shelf", "trash", "empty")},
	}

	for _, tc := range requests {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			rec := env.Request(tc.method, tc.path, nil)

			AssertStatus(t, rec, http.StatusNotFound)
			if got := strings.TrimSpace(rec.Body.String()); got != "shelf not found" {
				t.Fatalf("body = %q, want %q", got, "shelf not found")
			}
		})
	}
}

func TestAPIJSONResponsesShareOneShapeContract(t *testing.T) {
	env := New(t)

	paths := []string{
		"/api/mode",
		"/api/version",
		"/api/shelves",
		BooksURL(),
		"/api/setting/epub_import_strategy",
	}

	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			rec := env.Get(path)

			AssertStatus(t, rec, http.StatusOK)
			AssertJSONContentType(t, rec)

			// A client reading the body line-wise would block without the newline.
			if body := rec.Body.String(); !strings.HasSuffix(body, "\n") {
				t.Fatalf("body = %q, want a trailing newline", body)
			}
		})
	}
}
