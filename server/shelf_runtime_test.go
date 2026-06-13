package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/voilelab/plainshelf/shelf"
)

func TestRegisterShelfUpdatesGetShelvesWithoutRestMutationEndpoint(t *testing.T) {
	env := newAPITestEnv(t)

	if err := env.app.RegisterShelf(shelf.ShelfConfWithID{
		ID:        "desktop-books",
		Name:      "Desktop Books",
		ShelfConf: shelf.ShelfConf{LibRoot: t.TempDir()},
	}); err != nil {
		t.Fatalf("RegisterShelf: %v", err)
	}

	rec := env.do(httptest.NewRequest(http.MethodGet, "/api/shelves", nil))
	assertStatus(t, rec, http.StatusOK)

	var shelves []ShelfInfo
	if err := json.Unmarshal(rec.Body.Bytes(), &shelves); err != nil {
		t.Fatalf("Unmarshal shelves: %v", err)
	}
	if len(shelves) != 2 {
		t.Fatalf("len(shelves) = %d, want 2; shelves=%#v", len(shelves), shelves)
	}
	if shelves[0].ID != "default_shelf" || shelves[1].ID != "desktop-books" {
		t.Fatalf("shelves = %#v", shelves)
	}

	for _, method := range []string{http.MethodPost, http.MethodPatch, http.MethodDelete} {
		rec = env.do(httptest.NewRequest(method, "/api/shelves", nil))
		if rec.Code == http.StatusOK || rec.Code == http.StatusCreated || rec.Code == http.StatusNoContent {
			t.Fatalf("%s /api/shelves unexpectedly succeeded with status %d", method, rec.Code)
		}
	}
}
