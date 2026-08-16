package contract_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/voilelab/plainshelf/server"
)

func TestAPILayerMoveAndRenameContract(t *testing.T) {
	env := newAPITestEnv(t)
	created := importTextBook(t, env, "Layer Ops", "alpha/beta", "layer.txt", "body")

	rec := env.post(shelfURL("layers", "gamma"), nil)
	assertStatus(t, rec, http.StatusNoContent)

	rec = env.post(shelfURL("layer-moves"), strings.NewReader(`{"layer":["alpha","beta"],"target_layer":["gamma"]}`))
	assertStatus(t, rec, http.StatusNoContent)

	rec = env.patch(shelfURL("layers", "gamma", "beta"), strings.NewReader(`{"name":"renamed"}`))
	assertStatus(t, rec, http.StatusNoContent)

	got := getJSON[server.Book](t, env, bookURL(created.Meta.ID))
	if strings.Join(got.Layer, "/") != "gamma/renamed" {
		t.Fatalf("layer = %#v, want gamma/renamed", got.Layer)
	}

	// Moving onto a layer that does not exist is a conflict, and a name that is
	// not a single path segment is a client error.
	rec = env.post(shelfURL("layer-moves"), strings.NewReader(`{"layer":["gamma","renamed"],"target_layer":["missing"]}`))
	assertStatus(t, rec, http.StatusConflict)

	rec = env.patch(shelfURL("layers", "gamma", "renamed"), strings.NewReader(`{"name":"bad/name"}`))
	assertStatus(t, rec, http.StatusBadRequest)
}
