package contract_test

import (
	"net/http"
	"testing"
)

type modeResponse struct {
	ReadOnly bool `json:"read_only"`
}

type versionResponse struct {
	Version string `json:"version"`
}

// The version string is what an "about" screen shows and what a client compares
// to decide it is talking to a build it understands, so an empty one is a
// regression the shared response-shape table cannot see: {} and {"version":""}
// both carry the right status, content type and trailing newline.
func TestAPIVersionContract(t *testing.T) {
	env := newAPITestEnv(t)

	if resp := getJSON[versionResponse](t, env, "/api/version"); resp.Version == "" {
		t.Fatalf("version is empty, want a non-empty value")
	}
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
