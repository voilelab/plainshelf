package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// The shelf lookup preamble used to be copied into every handler, and the
// copies disagreed on how to word the same failure. These tests pin the wording
// and the JSON envelope so a future edit to the shared helpers cannot silently
// reintroduce the drift.

// resolveShelf's 400 branch is not reachable through the mux: a request whose
// shelf_id segment is empty does not match the route, and one with a malformed
// escape is rejected before routing. It is exercised directly so the wording
// stays pinned regardless.
func TestResolveShelfRejectsMissingShelfID(t *testing.T) {
	env := newAPITestEnv(t)

	rec := httptest.NewRecorder()
	shelfData, ok := env.app.resolveShelf(rec, httptest.NewRequest(http.MethodGet, "/api/shelves", nil))

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

	// One route per handler file that resolves a shelf, so a helper regression
	// shows up as more than a single failing route.
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

	// /api/mode and /api/version used to omit the charset parameter that every
	// other JSON route sent.
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

	// json.Encoder.Encode appended a newline, so the shared writer must too:
	// clients that read the body line-wise would otherwise block.
	rec := env.do(httptest.NewRequest(http.MethodGet, "/api/version", nil))

	assertStatus(t, rec, http.StatusOK)
	if body := rec.Body.String(); !strings.HasSuffix(body, "\n") {
		t.Fatalf("body = %q, want a trailing newline", body)
	}
}
