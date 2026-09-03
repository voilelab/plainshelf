package contract_test

import (
	"net/http"
	"testing"
)

type modeResponse struct {
	ReadOnly bool `json:"read_only"`
}

func TestAPIReadOnlyModeContract(t *testing.T) {
	env := newAPITestEnv(t)
	env.setReadOnly(t, true)

	if mode := getJSON[modeResponse](t, env, "/api/mode"); !mode.ReadOnly {
		t.Fatalf("read_only = false, want true")
	}

	// Read-only mode refuses writes while leaving reads alone.
	rec := env.post(shelfURL("folders", "blocked"), nil)
	assertStatus(t, rec, http.StatusForbidden)

	rec = env.get(shelfURL("folders"))
	assertStatus(t, rec, http.StatusOK)
}
