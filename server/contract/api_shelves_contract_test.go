package contract_test

import (
	"net/http"
	"testing"

	"github.com/voilelab/plainshelf/server"
)

// GET /api/shelves reports each shelf's read-only state, so the UI can drop the
// write affordances a read-only shelf has no use for rather than offering them
// and answering 409 when one is pressed.
func TestAPIShelvesReportPerShelfReadOnlyContract(t *testing.T) {
	env := newAPITestEnv(t, withReadOnlySecondShelf(t.TempDir()))

	rec := env.get("/api/shelves")
	assertStatus(t, rec, http.StatusOK)

	shelves := decodeJSON[[]server.ShelfInfo](t, rec)
	if len(shelves) != 2 {
		t.Fatalf("shelves = %d, want 2", len(shelves))
	}

	readOnlyByID := map[string]bool{}
	for _, info := range shelves {
		readOnlyByID[info.ID] = info.ReadOnly
	}

	if readOnlyByID[defaultShelfID] {
		t.Errorf("shelf %q read_only = true, want false", defaultShelfID)
	}
	if !readOnlyByID[secondShelfID] {
		t.Errorf("shelf %q read_only = false, want true", secondShelfID)
	}
}

// A server-wide read_only reaches the shelves themselves (applyAppReadOnly), so
// every listed shelf reports it. The client needs that: the two settings differ
// in scope, not in what they forbid on the shelf they cover.
func TestAPIShelvesReportServerReadOnlyContract(t *testing.T) {
	env := newAPITestEnv(t, withReadOnlyServer())

	rec := env.get("/api/shelves")
	assertStatus(t, rec, http.StatusOK)

	shelves := decodeJSON[[]server.ShelfInfo](t, rec)
	if len(shelves) == 0 {
		t.Fatal("shelves is empty, want at least one")
	}
	for _, info := range shelves {
		if !info.ReadOnly {
			t.Errorf("shelf %q read_only = false, want true", info.ID)
		}
	}
}

// A writable server lists its shelves as writable, which is what makes the flag
// worth reading at all.
func TestAPIShelvesReportWritableShelfContract(t *testing.T) {
	env := newAPITestEnv(t)

	rec := env.get("/api/shelves")
	assertStatus(t, rec, http.StatusOK)

	shelves := decodeJSON[[]server.ShelfInfo](t, rec)
	if len(shelves) != 1 {
		t.Fatalf("shelves = %d, want 1", len(shelves))
	}
	if shelves[0].ReadOnly {
		t.Errorf("shelf %q read_only = true, want false", shelves[0].ID)
	}
}
