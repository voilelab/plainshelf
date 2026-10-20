package crosscut_test

import (
	"bytes"
	"encoding/json/v2"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/voilelab/plainshelf/server/contract/apitest"

	"github.com/voilelab/plainshelf/internal/jsonopt"
	"github.com/voilelab/plainshelf/server"
	"github.com/voilelab/plainshelf/shelf"
)

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
	env := apitest.New(t)
	created := apitest.ImportTextBook(t, env, "Schema Version Book", "", "schema.txt", "body")

	books := apitest.GetJSON[[]map[string]any](t, env, apitest.BooksURL())
	if len(books) != 1 {
		t.Fatalf("list returned %d books, want 1", len(books))
	}
	if got := bookMetaSchemaVersion(t, books[0]); got != float64(shelf.BookMetaSchemaVersion) {
		t.Fatalf("list schema_version = %v, want %d", got, shelf.BookMetaSchemaVersion)
	}

	book := apitest.GetJSON[map[string]any](t, env, apitest.BookURL(created.Meta.ID))
	if got := bookMetaSchemaVersion(t, book); got != float64(shelf.BookMetaSchemaVersion) {
		t.Fatalf("get schema_version = %v, want %d", got, shelf.BookMetaSchemaVersion)
	}
}

// TestAPIUnsupportedSchemaVersionReturns409 verifies the end-to-end behavior for
// a book written by a newer build: still readable over the API, but every
// attempt to modify it fails with 409 and leaves the file untouched.
func TestAPIUnsupportedSchemaVersionReturns409(t *testing.T) {
	env := apitest.New(t)
	created := apitest.ImportTextBook(t, env, "Future Book", "", "future.txt", "body")

	metaPath, bumped := apitest.BumpBookSchemaVersion(t, env, created.Meta.ID,
		map[string]any{"reading_direction": "vertical-rl"})

	book := apitest.GetJSON[map[string]any](t, env, apitest.BookURL(created.Meta.ID))
	if got := bookMetaSchemaVersion(t, book); got != float64(shelf.BookMetaSchemaVersion+1) {
		t.Fatalf("schema_version = %v, want %d", got, shelf.BookMetaSchemaVersion+1)
	}

	apitest.PatchBook(t, env, created.Meta.ID, `{"title":"Clobbered"}`, http.StatusConflict)

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
	env := apitest.New(t)
	created := apitest.ImportTextBook(t, env, "Folder Guard", "origin/folder", "folder.txt", "body")

	metaPath, _ := apitest.BumpBookSchemaVersion(t, env, created.Meta.ID, nil)

	apitest.PatchBook(t, env, created.Meta.ID, `{"folder":["moved","elsewhere"]}`, http.StatusConflict)

	book := apitest.GetJSON[server.Book](t, env, apitest.BookURL(created.Meta.ID))
	if got := strings.Join(book.Folder, "/"); got != "origin/folder" {
		t.Fatalf("folder = %q, want origin/folder — the refused request moved the book", got)
	}
	if _, err := os.Stat(metaPath); err != nil {
		t.Fatalf("book.json is no longer at its original path: %v", err)
	}
}

func trashMetaPath(env *apitest.Env, bookID string) string {
	return filepath.Join(env.LibRoot, "trash", "books", bookID+".bookpkg", "trash.json")
}

func bumpTrashSchemaVersion(t *testing.T, env *apitest.Env, bookID string) (metaPath string, onDisk []byte) {
	t.Helper()

	metaPath = trashMetaPath(env, bookID)
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
	env := apitest.New(t)
	created := apitest.ImportTextBook(t, env, "Trash Schema Version", "", "trash.txt", "body")

	apitest.AssertStatus(t, env.Post(apitest.BookURL(created.Meta.ID, "trash"), nil), http.StatusNoContent)

	raw, err := os.ReadFile(trashMetaPath(env, created.Meta.ID))
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
	env := apitest.New(t)
	created := apitest.ImportTextBook(t, env, "Future Trash", "origin/folder", "future.txt", "body")

	apitest.AssertStatus(t, env.Post(apitest.BookURL(created.Meta.ID, "trash"), nil), http.StatusNoContent)
	metaPath, bumped := bumpTrashSchemaVersion(t, env, created.Meta.ID)

	trashed := apitest.GetJSON[[]map[string]any](t, env, apitest.TrashBooksURL())
	if len(trashed) != 1 {
		t.Fatalf("trashed books = %d, want 1", len(trashed))
	}
	if id, _ := trashed[0]["id"].(string); id != created.Meta.ID {
		t.Fatalf("trashed id = %q, want %q", id, created.Meta.ID)
	}

	apitest.AssertStatus(t, env.Post(apitest.TrashBooksURL(created.Meta.ID, "restore"), nil), http.StatusConflict)
	apitest.AssertStatus(t, env.Delete(apitest.TrashBooksURL(created.Meta.ID)), http.StatusConflict)

	accepted := apitest.EmptyTrash(t, env, http.StatusAccepted)
	chain := apitest.WaitForTaskChain(t, env, accepted.TaskChainID)
	if chain.Status != "partially_completed" {
		t.Fatalf("chain status = %q, want partially_completed: %+v", chain.Status, chain)
	}

	if trashed := apitest.GetJSON[[]map[string]any](t, env, apitest.TrashBooksURL()); len(trashed) != 1 {
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
