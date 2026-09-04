package contract_test

import (
	"net/http"
	"testing"

	"github.com/voilelab/plainshelf/server/contract/apitest"

	"github.com/voilelab/plainshelf/server"
)

// GET /api/shelves reports each shelf's read-only state, so the UI can drop the
// write affordances a read-only shelf has no use for rather than offering them
// and answering 409 when one is pressed.
func TestAPIShelvesReportPerShelfReadOnlyContract(t *testing.T) {
	env := apitest.New(t, apitest.WithReadOnlySecondShelf(t.TempDir()))

	rec := env.Get("/api/shelves")
	apitest.AssertStatus(t, rec, http.StatusOK)

	shelves := apitest.DecodeJSON[[]server.ShelfInfo](t, rec)
	if len(shelves) != 2 {
		t.Fatalf("shelves = %d, want 2", len(shelves))
	}

	readOnlyByID := map[string]bool{}
	for _, info := range shelves {
		readOnlyByID[info.ID] = info.ReadOnly
	}

	if readOnlyByID[apitest.DefaultShelfID] {
		t.Errorf("shelf %q read_only = true, want false", apitest.DefaultShelfID)
	}
	if !readOnlyByID[apitest.SecondShelfID] {
		t.Errorf("shelf %q read_only = false, want true", apitest.SecondShelfID)
	}
}

// A server-wide read_only reaches the shelves themselves (applyAppReadOnly), so
// every listed shelf reports it. The client needs that: the two settings differ
// in scope, not in what they forbid on the shelf they cover.
func TestAPIShelvesReportServerReadOnlyContract(t *testing.T) {
	env := apitest.New(t, apitest.WithReadOnlyServer())

	rec := env.Get("/api/shelves")
	apitest.AssertStatus(t, rec, http.StatusOK)

	shelves := apitest.DecodeJSON[[]server.ShelfInfo](t, rec)
	if len(shelves) == 0 {
		t.Fatal("shelves is empty, want at least one")
	}
	for _, info := range shelves {
		if !info.ReadOnly {
			t.Errorf("shelf %q read_only = false, want true", info.ID)
		}
	}
}

// The two settings stay separate in the other direction too: a shelf opened
// read-only does not make the server read-only, so /api/mode still answers for
// the app as a whole.
func TestAPIReadOnlyShelfLeavesServerModeContract(t *testing.T) {
	env := apitest.New(t, apitest.WithReadOnlyShelf())

	mode := apitest.GetJSON[map[string]any](t, env, "/api/mode")
	if got, want := mode["read_only"], any(false); got != want {
		t.Errorf("/api/mode read_only = %v, want %v", got, want)
	}
}
