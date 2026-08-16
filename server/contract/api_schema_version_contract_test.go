package contract_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/voilelab/plainshelf/server"
	"github.com/voilelab/plainshelf/shelf"
)

// TestAPIBookSchemaVersionContract asserts schema_version is present on the
// wire. It decodes into map[string]any rather than the Book struct on purpose:
// asserting through the Go type would pass tautologically even if the field
// never reached the JSON response.
func TestAPIBookSchemaVersionContract(t *testing.T) {
	env := newAPITestEnv(t)
	created := importTextBook(t, env, "Schema Version Book", "", "schema.txt", "body")

	rec := env.do(httptest.NewRequest(http.MethodGet, "/api/shelves/default_shelf/books", nil))
	assertStatus(t, rec, http.StatusOK)

	books := decodeJSON[[]map[string]any](t, rec)
	if len(books) != 1 {
		t.Fatalf("list returned %d books, want 1", len(books))
	}
	meta, ok := books[0]["meta"].(map[string]any)
	if !ok {
		t.Fatalf("book has no meta object: %#v", books[0])
	}
	if got := meta["schema_version"]; got != float64(shelf.BookMetaSchemaVersion) {
		t.Fatalf("list schema_version = %#v, want %d", got, shelf.BookMetaSchemaVersion)
	}

	rec = env.do(httptest.NewRequest(http.MethodGet, "/api/shelves/default_shelf/books/"+created.Meta.ID, nil))
	assertStatus(t, rec, http.StatusOK)

	book := decodeJSON[map[string]any](t, rec)
	meta, ok = book["meta"].(map[string]any)
	if !ok {
		t.Fatalf("book has no meta object: %#v", book)
	}
	if got := meta["schema_version"]; got != float64(shelf.BookMetaSchemaVersion) {
		t.Fatalf("get schema_version = %#v, want %d", got, shelf.BookMetaSchemaVersion)
	}
}

// TestAPIUnsupportedSchemaVersionReturns409 verifies the end-to-end behavior for
// a book written by a newer build: still readable over the API, but every
// attempt to modify it fails with 409 and leaves the file untouched.
func TestAPIUnsupportedSchemaVersionReturns409(t *testing.T) {
	env := newAPITestEnv(t)
	created := importTextBook(t, env, "Future Book", "", "future.txt", "body")

	metaPath := env.bookMetaPath(t, created.Meta.ID)
	raw, err := os.ReadFile(metaPath)
	if err != nil {
		t.Fatalf("read book.json: %v", err)
	}

	var onDisk map[string]any
	if err := json.Unmarshal(raw, &onDisk); err != nil {
		t.Fatalf("unmarshal book.json: %v", err)
	}
	onDisk["schema_version"] = shelf.BookMetaSchemaVersion + 1
	onDisk["reading_direction"] = "vertical-rl"
	bumped, err := json.MarshalIndent(onDisk, "", "  ")
	if err != nil {
		t.Fatalf("marshal book.json: %v", err)
	}
	if err := os.WriteFile(metaPath, bumped, 0o644); err != nil {
		t.Fatalf("write book.json: %v", err)
	}

	// The book stays readable and reports its real (newer) version, so a client
	// can tell the user this book needs a newer PlainShelf.
	rec := env.do(httptest.NewRequest(http.MethodGet, "/api/shelves/default_shelf/books/"+created.Meta.ID, nil))
	assertStatus(t, rec, http.StatusOK)
	book := decodeJSON[map[string]any](t, rec)
	meta, ok := book["meta"].(map[string]any)
	if !ok {
		t.Fatalf("book has no meta object: %#v", book)
	}
	if got := meta["schema_version"]; got != float64(shelf.BookMetaSchemaVersion+1) {
		t.Fatalf("schema_version = %#v, want %d", got, shelf.BookMetaSchemaVersion+1)
	}

	// Writing is refused.
	patch := httptest.NewRequest(http.MethodPatch, "/api/shelves/default_shelf/books/"+created.Meta.ID,
		strings.NewReader(`{"title":"Clobbered"}`))
	patch.Header.Set("Content-Type", "application/json")
	rec = env.do(patch)
	assertStatus(t, rec, http.StatusConflict)

	after, err := os.ReadFile(metaPath)
	if err != nil {
		t.Fatalf("re-read book.json: %v", err)
	}
	if !bytes.Equal(bumped, after) {
		t.Fatalf("refused write must leave book.json untouched, got:\n%s", after)
	}
	if !strings.Contains(string(after), "reading_direction") {
		t.Fatalf("unknown key must survive a refused write, got:\n%s", after)
	}
}

// TestAPIUnsupportedSchemaVersionDoesNotMoveLayer verifies the schema guard runs
// before the layer move. HandleAPIUpdateBook moves the book first, so a guard
// that only ran at SetMeta would rename the folder on disk and then report 409,
// leaving the client with a failed response for an applied mutation.
func TestAPIUnsupportedSchemaVersionDoesNotMoveLayer(t *testing.T) {
	env := newAPITestEnv(t)
	created := importTextBook(t, env, "Layer Guard", "origin/layer", "layer.txt", "body")

	metaPath := env.bookMetaPath(t, created.Meta.ID)
	raw, err := os.ReadFile(metaPath)
	if err != nil {
		t.Fatalf("read book.json: %v", err)
	}
	var onDisk map[string]any
	if err := json.Unmarshal(raw, &onDisk); err != nil {
		t.Fatalf("unmarshal book.json: %v", err)
	}
	onDisk["schema_version"] = shelf.BookMetaSchemaVersion + 1
	bumped, err := json.MarshalIndent(onDisk, "", "  ")
	if err != nil {
		t.Fatalf("marshal book.json: %v", err)
	}
	if err := os.WriteFile(metaPath, bumped, 0o644); err != nil {
		t.Fatalf("write book.json: %v", err)
	}

	patch := httptest.NewRequest(http.MethodPatch, "/api/shelves/default_shelf/books/"+created.Meta.ID,
		strings.NewReader(`{"layer":["moved","elsewhere"]}`))
	patch.Header.Set("Content-Type", "application/json")
	rec := env.do(patch)
	assertStatus(t, rec, http.StatusConflict)

	// The book must still be in its original layer, and still on disk there.
	rec = env.do(httptest.NewRequest(http.MethodGet, "/api/shelves/default_shelf/books/"+created.Meta.ID, nil))
	assertStatus(t, rec, http.StatusOK)
	book := decodeJSON[server.Book](t, rec)
	if got := strings.Join(book.Layer, "/"); got != "origin/layer" {
		t.Fatalf("layer = %q, want origin/layer — the refused request moved the book", got)
	}
	if _, err := os.Stat(metaPath); err != nil {
		t.Fatalf("book.json is no longer at its original path: %v", err)
	}
}
