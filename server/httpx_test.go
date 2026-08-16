package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// resolveShelf's 400 branch is not reachable through the mux, so it is
// exercised directly. What the routed shelf errors look like to a client is
// pinned by the contract tests instead.
func TestResolveShelfRejectsMissingShelfID(t *testing.T) {
	app := newTestApp(t)

	rec := httptest.NewRecorder()
	shelfData, ok := app.handlers.core.resolveShelf(rec, httptest.NewRequest(http.MethodGet, "/api/shelves", nil))

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
