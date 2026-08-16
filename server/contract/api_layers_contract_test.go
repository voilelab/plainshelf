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

func TestAPIInvalidLayerNameIsARequestErrorContract(t *testing.T) {
	env := newAPITestEnv(t)
	book := importTextBook(t, env, "Layer Book", "keep", "layer.txt", "body")

	tests := []struct {
		name         string
		method, path string
		body         string
	}{
		{
			name:   "create layer",
			method: http.MethodPost,
			path:   shelfURL("layers", "bad%2F..%2Fesc"),
		},
		{
			name:   "move layer",
			method: http.MethodPost,
			path:   shelfURL("layer-moves"),
			body:   `{"layer":[".."],"target_layer":["beta"]}`,
		},
		{
			name:   "move book to layer",
			method: http.MethodPatch,
			path:   bookURL(book.Meta.ID),
			body:   `{"layer":[".."]}`,
		},
		{
			// Creating and importing a book place it in a layer too, so they
			// reject a bad one on the same terms.
			name:   "create book in layer",
			method: http.MethodPost,
			path:   booksURL(),
			body:   `{"title":"X","layer":[".."]}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := env.request(tt.method, tt.path, strings.NewReader(tt.body))

			assertStatus(t, rec, http.StatusBadRequest)
			if got := strings.TrimSpace(rec.Body.String()); got != "invalid layer name" {
				t.Fatalf("body = %q, want %q", got, "invalid layer name")
			}
		})
	}
}

// The layer routes answer outcomes the error table cannot name as a conflict.
// Routing them through the table must not turn those into 500s.
func TestAPILayerConflictsStillAnswerConflictContract(t *testing.T) {
	env := newAPITestEnv(t)

	for _, layer := range []string{"alpha", "beta"} {
		assertStatus(t, env.post(shelfURL("layers", layer), nil), http.StatusNoContent)
	}

	t.Run("rename onto an existing layer", func(t *testing.T) {
		rec := env.patch(shelfURL("layers", "alpha"), strings.NewReader(`{"name":"beta"}`))
		assertStatus(t, rec, http.StatusConflict)
	})

	t.Run("move a layer under itself", func(t *testing.T) {
		rec := env.post(shelfURL("layer-moves"),
			strings.NewReader(`{"layer":["alpha"],"target_layer":["alpha"]}`))
		assertStatus(t, rec, http.StatusConflict)
	})
}

// Importing places the book in a layer as well, so a bad one must be refused
// the same way rather than reported as an import failure.
func TestAPIImportRejectsInvalidLayerNameContract(t *testing.T) {
	env := newAPITestEnv(t)

	rec := postBookImport(t, env,
		bookUpload("import.txt", plainTextContentType, "body", [2]string{"layer", ".."}))

	assertStatus(t, rec, http.StatusBadRequest)
	if got := strings.TrimSpace(rec.Body.String()); got != "invalid layer name" {
		t.Fatalf("body = %q, want %q", got, "invalid layer name")
	}
}
