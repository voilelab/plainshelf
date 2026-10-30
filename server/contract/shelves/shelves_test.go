package shelves_test

import (
	"net/http"
	"testing"

	"github.com/voilelab/plainshelf/server/contract/apitest"

	"github.com/voilelab/plainshelf/server"
)

func TestAPIShelvesReportShelfReadOnly(t *testing.T) {
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

func TestAPIShelvesReportServerReadOnly(t *testing.T) {
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

func TestAPIReadOnlyShelfKeepsServerMode(t *testing.T) {
	env := apitest.New(t, apitest.WithReadOnlyShelf())

	mode := apitest.GetJSON[map[string]any](t, env, "/api/mode")
	if got, want := mode["read_only"], any(false); got != want {
		t.Errorf("/api/mode read_only = %v, want %v", got, want)
	}
}
