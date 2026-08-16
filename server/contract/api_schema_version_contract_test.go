package contract_test

import (
	"bytes"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/voilelab/plainshelf/server"
	"github.com/voilelab/plainshelf/shelf"
)

// bookMetaSchemaVersion reads the schema_version the API reports for a book. It
// decodes into map[string]any rather than the Book struct on purpose: asserting
// through the Go type would pass tautologically even if the field never reached
// the JSON response.
func bookMetaSchemaVersion(t *testing.T, book map[string]any) float64 {
	t.Helper()

	meta, ok := book["meta"].(map[string]any)
	if !ok {
		t.Fatalf("book has no meta object: %#v", book)
	}
	version, ok := meta["schema_version"].(float64)
	if !ok {
		t.Fatalf("meta has no schema_version: %#v", meta)
	}
	return version
}

// TestAPIBookSchemaVersionContract asserts schema_version is present on the wire
// in both the list and the single-book response.
func TestAPIBookSchemaVersionContract(t *testing.T) {
	env := newAPITestEnv(t)
	created := importTextBook(t, env, "Schema Version Book", "", "schema.txt", "body")

	books := getJSON[[]map[string]any](t, env, booksURL())
	if len(books) != 1 {
		t.Fatalf("list returned %d books, want 1", len(books))
	}
	if got := bookMetaSchemaVersion(t, books[0]); got != float64(shelf.BookMetaSchemaVersion) {
		t.Fatalf("list schema_version = %v, want %d", got, shelf.BookMetaSchemaVersion)
	}

	book := getJSON[map[string]any](t, env, bookURL(created.Meta.ID))
	if got := bookMetaSchemaVersion(t, book); got != float64(shelf.BookMetaSchemaVersion) {
		t.Fatalf("get schema_version = %v, want %d", got, shelf.BookMetaSchemaVersion)
	}
}

// TestAPIUnsupportedSchemaVersionReturns409 verifies the end-to-end behavior for
// a book written by a newer build: still readable over the API, but every
// attempt to modify it fails with 409 and leaves the file untouched.
func TestAPIUnsupportedSchemaVersionReturns409(t *testing.T) {
	env := newAPITestEnv(t)
	created := importTextBook(t, env, "Future Book", "", "future.txt", "body")

	// The unknown key is written alongside the bumped version so a refused write
	// can be shown to have preserved it.
	metaPath, bumped := bumpBookSchemaVersion(t, env, created.Meta.ID,
		map[string]any{"reading_direction": "vertical-rl"})

	// The book stays readable and reports its real (newer) version, so a client
	// can tell the user this book needs a newer PlainShelf.
	book := getJSON[map[string]any](t, env, bookURL(created.Meta.ID))
	if got := bookMetaSchemaVersion(t, book); got != float64(shelf.BookMetaSchemaVersion+1) {
		t.Fatalf("schema_version = %v, want %d", got, shelf.BookMetaSchemaVersion+1)
	}

	// Writing is refused.
	patchBook(t, env, created.Meta.ID, `{"title":"Clobbered"}`, http.StatusConflict)

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

	metaPath, _ := bumpBookSchemaVersion(t, env, created.Meta.ID, nil)

	patchBook(t, env, created.Meta.ID, `{"layer":["moved","elsewhere"]}`, http.StatusConflict)

	// The book must still be in its original layer, and still on disk there.
	book := getJSON[server.Book](t, env, bookURL(created.Meta.ID))
	if got := strings.Join(book.Layer, "/"); got != "origin/layer" {
		t.Fatalf("layer = %q, want origin/layer — the refused request moved the book", got)
	}
	if _, err := os.Stat(metaPath); err != nil {
		t.Fatalf("book.json is no longer at its original path: %v", err)
	}
}
