package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// resolveShelf's 400 branch is not reachable through the mux, so it is
// exercised directly.
func TestResolveShelfRejectsMissingShelfID(t *testing.T) {
	env := newAPITestEnv(t)

	rec := httptest.NewRecorder()
	shelfData, ok := env.app.handlers.core.resolveShelf(rec, httptest.NewRequest(http.MethodGet, "/api/shelves", nil))

	if ok || shelfData != nil {
		t.Fatalf("resolveShelf = (%v, %v), want (nil, false)", shelfData, ok)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	if got := strings.TrimSpace(rec.Body.String()); got != "invalid shelf_id" {
		t.Fatalf("body = %q, want %q", got, "invalid shelf_id")
	}
}

func TestResolveShelfRejectsUnknownShelfConsistently(t *testing.T) {
	env := newAPITestEnv(t)

	paths := []string{
		"/api/shelves/missing_shelf/books",
		"/api/shelves/missing_shelf/status",
		"/api/shelves/missing_shelf/layers",
		"/api/shelves/missing_shelf/trash/books",
		"/api/shelves/missing_shelf/books/duplicate",
	}

	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			rec := env.do(httptest.NewRequest(http.MethodGet, path, nil))

			assertStatus(t, rec, http.StatusNotFound)
			if got := strings.TrimSpace(rec.Body.String()); got != "shelf not found" {
				t.Fatalf("body = %q, want %q", got, "shelf not found")
			}
		})
	}
}

func TestJSONResponsesShareOneContentType(t *testing.T) {
	env := newAPITestEnv(t)

	paths := []string{
		"/api/mode",
		"/api/version",
		"/api/shelves",
		"/api/shelves/default_shelf/books",
		"/api/setting/default_split_config",
		"/api/setting/epub_import_strategy",
	}

	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			rec := env.do(httptest.NewRequest(http.MethodGet, path, nil))

			assertStatus(t, rec, http.StatusOK)
			assertJSONContentType(t, rec)
		})
	}
}

func TestWriteJSONTerminatesBodyWithNewline(t *testing.T) {
	env := newAPITestEnv(t)

	// A client reading the body line-wise would block without the newline.
	rec := env.do(httptest.NewRequest(http.MethodGet, "/api/version", nil))

	assertStatus(t, rec, http.StatusOK)
	if body := rec.Body.String(); !strings.HasSuffix(body, "\n") {
		t.Fatalf("body = %q, want a trailing newline", body)
	}
}
