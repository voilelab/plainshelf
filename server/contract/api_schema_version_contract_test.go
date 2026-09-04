package contract_test

import (
	"bytes"
	"encoding/json/v2"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/voilelab/plainshelf/internal/jsonopt"
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
	env := New(t)
	created := ImportTextBook(t, env, "Schema Version Book", "", "schema.txt", "body")

	books := GetJSON[[]map[string]any](t, env, BooksURL())
	if len(books) != 1 {
		t.Fatalf("list returned %d books, want 1", len(books))
	}
	if got := bookMetaSchemaVersion(t, books[0]); got != float64(shelf.BookMetaSchemaVersion) {
		t.Fatalf("list schema_version = %v, want %d", got, shelf.BookMetaSchemaVersion)
	}

	book := GetJSON[map[string]any](t, env, BookURL(created.Meta.ID))
	if got := bookMetaSchemaVersion(t, book); got != float64(shelf.BookMetaSchemaVersion) {
		t.Fatalf("get schema_version = %v, want %d", got, shelf.BookMetaSchemaVersion)
	}
}

// TestAPIUnsupportedSchemaVersionReturns409 verifies the end-to-end behavior for
// a book written by a newer build: still readable over the API, but every
// attempt to modify it fails with 409 and leaves the file untouched.
func TestAPIUnsupportedSchemaVersionReturns409(t *testing.T) {
	env := New(t)
	created := ImportTextBook(t, env, "Future Book", "", "future.txt", "body")

	// The unknown key is written alongside the bumped version so a refused write
	// can be shown to have preserved it.
	metaPath, bumped := BumpBookSchemaVersion(t, env, created.Meta.ID,
		map[string]any{"reading_direction": "vertical-rl"})

	// The book stays readable and reports its real (newer) version, so a client
	// can tell the user this book needs a newer PlainShelf.
	book := GetJSON[map[string]any](t, env, BookURL(created.Meta.ID))
	if got := bookMetaSchemaVersion(t, book); got != float64(shelf.BookMetaSchemaVersion+1) {
		t.Fatalf("schema_version = %v, want %d", got, shelf.BookMetaSchemaVersion+1)
	}

	// Writing is refused.
	PatchBook(t, env, created.Meta.ID, `{"title":"Clobbered"}`, http.StatusConflict)

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

// TestAPIUnsupportedSchemaVersionDoesNotMoveFolder verifies the schema guard runs
// before the folder move. HandleAPIUpdateBook moves the book first, so a guard
// that only ran at SetMeta would rename the folder on disk and then report 409,
// leaving the client with a failed response for an applied mutation.
func TestAPIUnsupportedSchemaVersionDoesNotMoveFolder(t *testing.T) {
	env := New(t)
	created := ImportTextBook(t, env, "Folder Guard", "origin/folder", "folder.txt", "body")

	metaPath, _ := BumpBookSchemaVersion(t, env, created.Meta.ID, nil)

	PatchBook(t, env, created.Meta.ID, `{"folder":["moved","elsewhere"]}`, http.StatusConflict)

	// The book must still be in its original folder, and still on disk there.
	book := GetJSON[server.Book](t, env, BookURL(created.Meta.ID))
	if got := strings.Join(book.Folder, "/"); got != "origin/folder" {
		t.Fatalf("folder = %q, want origin/folder — the refused request moved the book", got)
	}
	if _, err := os.Stat(metaPath); err != nil {
		t.Fatalf("book.json is no longer at its original path: %v", err)
	}
}

// trashMetaPath returns the on-disk path of a trashed book's trash.json. The
// layout is spelled out rather than derived from the shelf package's unexported
// constants: a contract test asserting on disk should notice if it moves.
func (env *Env) trashMetaPath(bookID string) string {
	return filepath.Join(env.LibRoot, "trash", "books", bookID+".bookpkg", "trash.json")
}

// bumpTrashSchemaVersion rewrites a trashed book's trash.json with a schema
// version this build does not support, reproducing a book trashed by a newer
// PlainShelf. It returns the bytes now on disk so a caller can prove a refused
// operation left them alone.
func bumpTrashSchemaVersion(t *testing.T, env *Env, bookID string) (metaPath string, onDisk []byte) {
	t.Helper()

	metaPath = env.trashMetaPath(bookID)
	raw, err := os.ReadFile(metaPath)
	if err != nil {
		t.Fatalf("read trash.json: %v", err)
	}

	var meta map[string]any
	if err := json.Unmarshal(raw, &meta); err != nil {
		t.Fatalf("unmarshal trash.json: %v", err)
	}
	meta["schema_version"] = shelf.TrashMetaSchemaVersion + 1
	meta["restore_note"] = "written by a newer build"

	bumped, err := json.Marshal(meta, jsonopt.Disk())
	if err != nil {
		t.Fatalf("marshal trash.json: %v", err)
	}
	if err := os.WriteFile(metaPath, bumped, 0o644); err != nil {
		t.Fatalf("write trash.json: %v", err)
	}
	return metaPath, bumped
}

// TestAPITrashSchemaVersionContract asserts trashing a book stamps the version
// this build writes into trash.json.
func TestAPITrashSchemaVersionContract(t *testing.T) {
	env := New(t)
	created := ImportTextBook(t, env, "Trash Schema Version", "", "trash.txt", "body")

	AssertStatus(t, env.Post(BookURL(created.Meta.ID, "trash"), nil), http.StatusNoContent)

	raw, err := os.ReadFile(env.trashMetaPath(created.Meta.ID))
	if err != nil {
		t.Fatalf("read trash.json: %v", err)
	}
	var meta map[string]any
	if err := json.Unmarshal(raw, &meta); err != nil {
		t.Fatalf("unmarshal trash.json: %v", err)
	}
	if got, _ := meta["schema_version"].(float64); got != float64(shelf.TrashMetaSchemaVersion) {
		t.Fatalf("trash.json schema_version = %v, want %d", meta["schema_version"], shelf.TrashMetaSchemaVersion)
	}
}

// TestAPIUnsupportedTrashSchemaVersionReturns409 verifies the behavior for a
// book trashed by a newer build: still listed in the trash, but restoring or
// permanently deleting it fails with 409 and leaves the files alone.
func TestAPIUnsupportedTrashSchemaVersionReturns409(t *testing.T) {
	env := New(t)
	created := ImportTextBook(t, env, "Future Trash", "origin/folder", "future.txt", "body")

	AssertStatus(t, env.Post(BookURL(created.Meta.ID, "trash"), nil), http.StatusNoContent)
	metaPath, bumped := bumpTrashSchemaVersion(t, env, created.Meta.ID)

	// The book stays visible in the trash, so a client can tell the user it
	// needs a newer PlainShelf instead of finding it simply gone.
	trashed := GetJSON[[]map[string]any](t, env, TrashBooksURL())
	if len(trashed) != 1 {
		t.Fatalf("trashed books = %d, want 1", len(trashed))
	}
	if id, _ := trashed[0]["id"].(string); id != created.Meta.ID {
		t.Fatalf("trashed id = %q, want %q", id, created.Meta.ID)
	}

	AssertStatus(t, env.Post(TrashBooksURL(created.Meta.ID, "restore"), nil), http.StatusConflict)
	AssertStatus(t, env.Delete(TrashBooksURL(created.Meta.ID)), http.StatusConflict)

	// Emptying the trash is background work, so the refusal cannot be a status
	// code: the sweep reports a partial result and the book stays.
	accepted := EmptyTrash(t, env, http.StatusAccepted)
	chain := WaitForTaskChain(t, env, accepted.TaskChainID)
	if chain.Status != "partially_completed" {
		t.Fatalf("chain status = %q, want partially_completed: %+v", chain.Status, chain)
	}

	if trashed := GetJSON[[]map[string]any](t, env, TrashBooksURL()); len(trashed) != 1 {
		t.Fatalf("trashed books after empty = %d, want the refused book to remain", len(trashed))
	}
	after, err := os.ReadFile(metaPath)
	if err != nil {
		t.Fatalf("re-read trash.json: %v", err)
	}
	if !bytes.Equal(bumped, after) {
		t.Fatalf("refused operations must leave trash.json untouched, got:\n%s", after)
	}
	if !strings.Contains(string(after), "restore_note") {
		t.Fatalf("unknown key must survive a refused operation, got:\n%s", after)
	}
}
