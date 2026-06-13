package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/voilelab/plainshelf/shelf"
)

func TestAppAddShelfUpdatesGetShelvesWithoutRestMutationRoute(t *testing.T) {
	env := newAPITestEnv(t)

	if err := env.app.AddShelf(ShelfConfWithID{
		ID:   "extra",
		Name: "Extra Shelf",
		ShelfConf: shelf.ShelfConf{
			LibRoot: t.TempDir(),
		},
	}); err != nil {
		t.Fatalf("AddShelf: %v", err)
	}

	rec := env.do(httptest.NewRequest(http.MethodGet, "/api/shelves", nil))
	assertStatus(t, rec, http.StatusOK)

	var shelves []ShelfInfo
	if err := json.Unmarshal(rec.Body.Bytes(), &shelves); err != nil {
		t.Fatalf("Unmarshal shelves: %v", err)
	}
	if len(shelves) != 2 {
		t.Fatalf("expected two shelves, got %#v", shelves)
	}
	if shelves[0].ID != "default_shelf" || shelves[1].ID != "extra" {
		t.Fatalf("unexpected shelf ordering: %#v", shelves)
	}

	rec = env.do(httptest.NewRequest(http.MethodPost, "/api/shelves", nil))
	if rec.Code != http.StatusNotFound && rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST /api/shelves status = %d, want 404 or 405", rec.Code)
	}
}
