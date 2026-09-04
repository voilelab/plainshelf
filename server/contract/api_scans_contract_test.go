package contract_test

import (
	"encoding/json/v2"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/voilelab/plainshelf/server"
	"github.com/voilelab/plainshelf/shelf"
)

func ScansURL() string {
	return ShelfURL("scans")
}

// writeBookIntoShelf drops a book package straight into books/, the way copying
// a folder in from outside PlainShelf does. Nothing tells the server about it,
// so only a walk of the tree can find it.
func writeBookIntoShelf(t *testing.T, libRoot, dirName, bookID, title string) {
	t.Helper()

	bookDir := filepath.Join(libRoot, "books", dirName+".bookpkg")
	if err := os.MkdirAll(bookDir, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	data, err := json.Marshal(map[string]any{
		"schema_version": shelf.BookMetaSchemaVersion,
		"id":             bookID,
		"title":          title,
	})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if err := os.WriteFile(filepath.Join(bookDir, shelf.BookMetaFile), data, 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}

func TestAPIRescanShelfContract(t *testing.T) {
	env := New(t)
	ImportTextBook(t, env, "Already Here", "Fiction", "here.txt", "Some content.")

	// Added the way a user does when they ask why the book has not appeared:
	// from outside PlainShelf, with no request to tell the server about it.
	writeBookIntoShelf(t, env.LibRoot, "dropped-in", "drop1nkb", "Dropped In")

	rec := env.Post(ScansURL(), nil)
	AssertStatus(t, rec, http.StatusOK)
	AssertJSONContentType(t, rec)

	scan := DecodeJSON[server.ScanResponse](t, rec)
	if scan.ScanID == "" {
		t.Error("scan_id is empty")
	}
	if scan.ScannedAt <= 0 {
		t.Errorf("scanned_at = %d, want the Unix time the walk began", scan.ScannedAt)
	}
	if scan.BookCount != 2 {
		t.Errorf("book_count = %d, want 2", scan.BookCount)
	}
	// "/" and "Fiction".
	if scan.FolderCount != 2 {
		t.Errorf("folder_count = %d, want 2", scan.FolderCount)
	}

	// The counts are only worth reporting if the listing agrees with them.
	books := GetJSON[[]server.Book](t, env, BooksURL())
	if len(books) != 2 {
		t.Fatalf("books after the rescan = %d, want the externally added book to be listed", len(books))
	}
}

// A rescan reads the shelf and writes nothing to it, so read-only mode has no
// reason to refuse it — unlike every other POST, which AssertMutationGated
// pins as refused.
func TestAPIRescanShelfIsAllowedInReadOnlyModeContract(t *testing.T) {
	env := New(t)
	env.SetReadOnly(t, true)

	rec := env.Post(ScansURL(), nil)
	AssertStatus(t, rec, http.StatusOK)
}

// The token gate draws the same exemption: a rescan is a read, so protect_read
// governs it rather than its method. Under the shipped defaults -- local_token
// with protect_read off -- the docs promise reading needs no token, and the
// "refresh the book list" button is a read the user can see.
func TestAPIRescanShelfNeedsNoTokenWithoutProtectReadContract(t *testing.T) {
	env := New(t)

	rec := env.DoRaw(httptest.NewRequest(http.MethodPost, ScansURL(), nil))
	AssertStatus(t, rec, http.StatusOK)
}

// With protect_read on, reads need a token and the rescan is one of them.
func TestAPIRescanShelfRequiresTheTokenUnderProtectReadContract(t *testing.T) {
	security := LocalTokenSecurity()
	security.ProtectRead = true
	env := New(t, WithSecurity(security))

	rec := env.DoRaw(httptest.NewRequest(http.MethodPost, ScansURL(), nil))
	AssertStatus(t, rec, http.StatusUnauthorized)

	rec = env.Post(ScansURL(), nil)
	AssertStatus(t, rec, http.StatusOK)
}

// Dropping the token requirement does not drop the CSRF one: local_token is
// documented as a CSRF boundary, so a rescan arriving from a page the operator
// never listed is still refused. A request with no Origin at all is not a
// browser -- the Android client's native HTTP bridge sends none.
func TestAPIRescanShelfRefusesAnUnknownOriginWithoutATokenContract(t *testing.T) {
	env := New(t, WithSecurity(LocalTokenSecurity()))

	req := httptest.NewRequest(http.MethodPost, ScansURL(), nil)
	req.Header.Set("Origin", "http://evil.example")
	AssertStatus(t, env.DoRaw(req), http.StatusForbidden)

	req = httptest.NewRequest(http.MethodPost, ScansURL(), nil)
	req.Header.Set("Origin", "http://localhost:20000")
	AssertStatus(t, env.DoRaw(req), http.StatusOK)
}

// Both exemptions - no token needed, allowed in read-only mode - are matched on
// the same path helper, so neither may open a neighbouring route or a shelf-less
// path that happens to end in /scans.
func TestAPIScanExemptionsAreLimitedToTheScanRouteContract(t *testing.T) {
	urls := []string{
		ShelfURL("scans", "extra"),
		ShelfURL("book-cache-exports"),
		"/api/shelves//scans",
		"/api/scans",
	}

	t.Run("token gate", func(t *testing.T) {
		env := New(t)

		for _, url := range urls {
			t.Run(url, func(t *testing.T) {
				// DoRaw sends the request as-is, without the token Do() attaches.
				rec := env.DoRaw(httptest.NewRequest(http.MethodPost, url, nil))
				AssertStatus(t, rec, http.StatusUnauthorized)
			})
		}
	})

	t.Run("read-only gate", func(t *testing.T) {
		env := New(t)
		env.SetReadOnly(t, true)

		for _, url := range urls {
			t.Run(url, func(t *testing.T) {
				AssertStatus(t, env.Post(url, nil), http.StatusForbidden)
			})
		}
	})
}

// A loop of rescans is answered 429 once the burst is spent, and 429 rather
// than 409 so the client can tell "you are too fast" from "someone else is
// walking this shelf right now".
func TestAPIRescanShelfRateLimitContract(t *testing.T) {
	env := New(t)

	// The burst is the shelf package's own, so this walks up to the refusal
	// rather than encoding a count the server contract does not own. The bound
	// only keeps a broken limiter from looping forever.
	var rec *httptest.ResponseRecorder
	for range 100 {
		rec = env.Post(ScansURL(), nil)
		if rec.Code != http.StatusOK {
			break
		}
	}

	AssertStatus(t, rec, http.StatusTooManyRequests)

	limited := DecodeJSON[server.ScanRateLimitResponse](t, rec)
	if limited.RetryAfterSeconds < 1 {
		t.Errorf("retry_after_seconds = %d, want at least 1", limited.RetryAfterSeconds)
	}
	if limited.Message == "" {
		t.Error("a rate-limited rescan carried no readable message")
	}

	if header := rec.Header().Get("Retry-After"); header != strconv.Itoa(limited.RetryAfterSeconds) {
		t.Errorf("Retry-After header = %q, want %q to match the body", header, strconv.Itoa(limited.RetryAfterSeconds))
	}

	// The body must not read as a conflict: 409 carries the running walk's ID
	// and no counts, and a client keying on either would misreport this.
	var asConflict server.ScanConflictResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &asConflict); err != nil {
		t.Fatalf("unmarshal the 429 body: %v", err)
	}
	if asConflict.ScanID != "" {
		t.Errorf("the 429 body carried scan_id %q; a rate limit names no running walk", asConflict.ScanID)
	}
}
