package contract_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/voilelab/plainshelf/server"
)

func TestAPILayerMoveAndRenameContract(t *testing.T) {
	env := newAPITestEnv(t)
	created := importTextBook(t, env, "Layer Ops", "alpha/beta", "layer.txt", "body")

	rec := env.do(httptest.NewRequest(http.MethodPost, "/api/shelves/default_shelf/layers/gamma", nil))
	assertStatus(t, rec, http.StatusNoContent)

	rec = env.do(httptest.NewRequest(http.MethodPost, "/api/shelves/default_shelf/layer-moves", strings.NewReader(`{"layer":["alpha","beta"],"target_layer":["gamma"]}`)))
	assertStatus(t, rec, http.StatusNoContent)

	rec = env.do(httptest.NewRequest(http.MethodPatch, "/api/shelves/default_shelf/layers/gamma/beta", strings.NewReader(`{"name":"renamed"}`)))
	assertStatus(t, rec, http.StatusNoContent)

	rec = env.do(httptest.NewRequest(http.MethodGet, "/api/shelves/default_shelf/books/"+created.Meta.ID, nil))
	assertStatus(t, rec, http.StatusOK)
	got := decodeJSON[server.Book](t, rec)
	if strings.Join(got.Layer, "/") != "gamma/renamed" {
		t.Fatalf("layer = %#v, want gamma/renamed", got.Layer)
	}

	rec = env.do(httptest.NewRequest(http.MethodPost, "/api/shelves/default_shelf/layer-moves", strings.NewReader(`{"layer":["gamma","renamed"],"target_layer":["missing"]}`)))
	assertStatus(t, rec, http.StatusConflict)

	rec = env.do(httptest.NewRequest(http.MethodPatch, "/api/shelves/default_shelf/layers/gamma/renamed", strings.NewReader(`{"name":"bad/name"}`)))
	assertStatus(t, rec, http.StatusBadRequest)
}
