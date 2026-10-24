package shelves_test

import (
	"encoding/json/v2"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/voilelab/plainshelf/server/contract/apitest"

	"github.com/voilelab/plainshelf/server"
	"github.com/voilelab/plainshelf/shelf"
)

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
	env := apitest.New(t)
	apitest.ImportTextBook(t, env, "Already Here", "Fiction", "here.txt", "Some content.")

	writeBookIntoShelf(t, env.LibRoot, "dropped-in", "drop1nkb", "Dropped In")

	rec := env.Post(apitest.ScansURL(), nil)
	apitest.AssertStatus(t, rec, http.StatusOK)
	apitest.AssertJSONContentType(t, rec)

	scan := apitest.DecodeJSON[server.ScanResponse](t, rec)
	if scan.ScanID == "" {
		t.Error("scan_id is empty")
	}
	if scan.ScannedAt <= 0 {
		t.Errorf("scanned_at = %d, want the Unix time the walk began", scan.ScannedAt)
	}
	if scan.BookCount != 2 {
		t.Errorf("book_count = %d, want 2", scan.BookCount)
	}

	if scan.FolderCount != 2 {
		t.Errorf("folder_count = %d, want 2", scan.FolderCount)
	}

	books := apitest.GetJSON[[]server.Book](t, env, apitest.BooksURL())
	if len(books) != 2 {
		t.Fatalf("books after the rescan = %d, want the externally added book to be listed", len(books))
	}
}

func TestAPIRescanShelfIsAllowedInReadOnlyModeContract(t *testing.T) {
	env := apitest.New(t)
	env.SetReadOnly(t, true)

	rec := env.Post(apitest.ScansURL(), nil)
	apitest.AssertStatus(t, rec, http.StatusOK)
}

func TestAPIRescanWithoutReadProtection(t *testing.T) {
	env := apitest.New(t)

	rec := env.DoRaw(httptest.NewRequest(http.MethodPost, apitest.ScansURL(), nil))
	apitest.AssertStatus(t, rec, http.StatusOK)
}

func TestAPIRescanWithReadProtection(t *testing.T) {
	security := apitest.LocalTokenSecurity()
	security.ProtectRead = true
	env := apitest.New(t, apitest.WithSecurity(security))

	rec := env.DoRaw(httptest.NewRequest(http.MethodPost, apitest.ScansURL(), nil))
	apitest.AssertStatus(t, rec, http.StatusUnauthorized)

	rec = env.Post(apitest.ScansURL(), nil)
	apitest.AssertStatus(t, rec, http.StatusOK)
}

func TestAPIRescanRejectsUnknownOrigin(t *testing.T) {
	env := apitest.New(t, apitest.WithSecurity(apitest.LocalTokenSecurity()))

	req := httptest.NewRequest(http.MethodPost, apitest.ScansURL(), nil)
	req.Header.Set("Origin", "http://evil.example")
	apitest.AssertStatus(t, env.DoRaw(req), http.StatusForbidden)

	req = httptest.NewRequest(http.MethodPost, apitest.ScansURL(), nil)
	req.Header.Set("Origin", "http://localhost:20000")
	apitest.AssertStatus(t, env.DoRaw(req), http.StatusOK)
}

func TestAPIScanRouteExemptions(t *testing.T) {
	urls := []string{
		apitest.ShelfURL("scans", "extra"),
		apitest.ShelfURL("book-cache-exports"),
		"/api/shelves//scans",
		"/api/scans",
	}

	t.Run("token gate", func(t *testing.T) {
		env := apitest.New(t)

		for _, url := range urls {
			t.Run(url, func(t *testing.T) {

				rec := env.DoRaw(httptest.NewRequest(http.MethodPost, url, nil))
				apitest.AssertStatus(t, rec, http.StatusUnauthorized)
			})
		}
	})

	t.Run("read-only gate", func(t *testing.T) {
		env := apitest.New(t)
		env.SetReadOnly(t, true)

		for _, url := range urls {
			t.Run(url, func(t *testing.T) {
				apitest.AssertStatus(t, env.Post(url, nil), http.StatusForbidden)
			})
		}
	})
}

func TestAPIRescanShelfRateLimitContract(t *testing.T) {
	env := apitest.New(t)

	var rec *httptest.ResponseRecorder
	for range 100 {
		rec = env.Post(apitest.ScansURL(), nil)
		if rec.Code != http.StatusOK {
			break
		}
	}

	apitest.AssertStatus(t, rec, http.StatusTooManyRequests)

	limited := apitest.DecodeJSON[server.ScanRateLimitResponse](t, rec)
	if limited.RetryAfterSeconds < 1 {
		t.Errorf("retry_after_seconds = %d, want at least 1", limited.RetryAfterSeconds)
	}
	if limited.Message == "" {
		t.Error("a rate-limited rescan carried no readable message")
	}

	if header := rec.Header().Get("Retry-After"); header != strconv.Itoa(limited.RetryAfterSeconds) {
		t.Errorf("Retry-After header = %q, want %q to match the body", header, strconv.Itoa(limited.RetryAfterSeconds))
	}

	var asConflict server.ScanConflictResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &asConflict); err != nil {
		t.Fatalf("unmarshal the 429 body: %v", err)
	}
	if asConflict.ScanID != "" {
		t.Errorf("the 429 body carried scan_id %q; a rate limit names no running walk", asConflict.ScanID)
	}
}
