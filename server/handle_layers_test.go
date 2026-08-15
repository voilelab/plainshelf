package server

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"strings"
	"testing"
)

func layerRequest(t *testing.T, env *apiTestEnv, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	return env.do(httptest.NewRequest(method, path, strings.NewReader(body)))
}

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
		{
			// Creating and importing a book place it in a layer too, so they
			// reject a bad one on the same terms.
			name:   "create book in layer",
			method: http.MethodPost,
			path:   "/api/shelves/default_shelf/books",
			body:   `{"title":"X","layer":[".."]}`,
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

// The layer routes answer outcomes the error table cannot name as a conflict.
// Routing them through the table must not turn those into 500s.
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

// Importing places the book in a layer as well, so a bad one must be refused
// the same way rather than reported as an import failure.
func TestImportRejectsInvalidLayerName(t *testing.T) {
	env := newAPITestEnv(t)

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	if err := writer.WriteField("layer", ".."); err != nil {
		t.Fatalf("WriteField layer: %v", err)
	}
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", `form-data; name="file"; filename="import.txt"`)
	h.Set("Content-Type", "text/plain; charset=utf-8")
	part, err := writer.CreatePart(h)
	if err != nil {
		t.Fatalf("CreatePart: %v", err)
	}
	if _, err := part.Write([]byte("body")); err != nil {
		t.Fatalf("write part: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close multipart writer: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/shelves/default_shelf/books/import", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := env.do(req)

	assertStatus(t, rec, http.StatusBadRequest)
	if got := strings.TrimSpace(rec.Body.String()); got != "invalid layer name" {
		t.Fatalf("body = %q, want %q", got, "invalid layer name")
	}
}
