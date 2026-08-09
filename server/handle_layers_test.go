package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func layerRequest(t *testing.T, env *apiTestEnv, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	return env.do(httptest.NewRequest(method, path, strings.NewReader(body)))
}

// A layer name the shelf refuses is a request error. These routes used to
// report it as a conflict or as a server failure, because shelf returned it
// without a sentinel to match on.
func TestInvalidLayerNameIsARequestError(t *testing.T) {
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
			path:   "/api/shelves/default_shelf/layers/bad%2F..%2Fesc",
		},
		{
			name:   "move layer",
			method: http.MethodPost,
			path:   "/api/shelves/default_shelf/layer-moves",
			body:   `{"layer":[".."],"target_layer":["beta"]}`,
		},
		{
			name:   "move book to layer",
			method: http.MethodPatch,
			path:   "/api/shelves/default_shelf/books/" + book.Meta.ID,
			body:   `{"layer":[".."]}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := layerRequest(t, env, tt.method, tt.path, tt.body)

			assertStatus(t, rec, http.StatusBadRequest)
			if got := strings.TrimSpace(rec.Body.String()); got != "invalid layer name" {
				t.Fatalf("body = %q, want %q", got, "invalid layer name")
			}
		})
	}
}

// The layer routes answer a family of outcomes the error table cannot name --
// a missing layer, an occupied destination -- as a conflict. Routing them
// through the table must not turn those into 500s.
func TestLayerConflictsStillAnswerConflict(t *testing.T) {
	env := newAPITestEnv(t)

	for _, layer := range []string{"alpha", "beta"} {
		rec := layerRequest(t, env, http.MethodPost, "/api/shelves/default_shelf/layers/"+layer, "")
		assertStatus(t, rec, http.StatusNoContent)
	}

	t.Run("rename onto an existing layer", func(t *testing.T) {
		rec := layerRequest(t, env, http.MethodPatch, "/api/shelves/default_shelf/layers/alpha", `{"name":"beta"}`)
		assertStatus(t, rec, http.StatusConflict)
	})

	t.Run("move a layer under itself", func(t *testing.T) {
		rec := layerRequest(t, env, http.MethodPost, "/api/shelves/default_shelf/layer-moves",
			`{"layer":["alpha"],"target_layer":["alpha"]}`)
		assertStatus(t, rec, http.StatusConflict)
	})
}
