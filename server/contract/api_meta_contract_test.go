package contract_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

type modeResponse struct {
	ReadOnly bool `json:"read_only"`
}

type versionResponse struct {
	Version string `json:"version"`
}

func TestAPIVersionContract(t *testing.T) {
	env := newAPITestEnv(t)

	rec := env.do(httptest.NewRequest(http.MethodGet, "/api/version", nil))
	assertStatus(t, rec, http.StatusOK)
	resp := decodeJSON[versionResponse](t, rec)
	if resp.Version == "" {
		t.Fatalf("version is empty, want a non-empty value")
	}
}

func TestAPIReadOnlyModeContract(t *testing.T) {
	env := newAPITestEnv(t)
	env.app.Conf().ReadOnly = true

	rec := env.do(httptest.NewRequest(http.MethodGet, "/api/mode", nil))
	assertStatus(t, rec, http.StatusOK)
	mode := decodeJSON[modeResponse](t, rec)
	if !mode.ReadOnly {
		t.Fatalf("read_only = false, want true")
	}

	rec = env.do(httptest.NewRequest(http.MethodPost, "/api/shelves/default_shelf/layers/blocked", nil))
	assertStatus(t, rec, http.StatusForbidden)

	rec = env.do(httptest.NewRequest(http.MethodGet, "/api/shelves/default_shelf/layers", nil))
	assertStatus(t, rec, http.StatusOK)
}
