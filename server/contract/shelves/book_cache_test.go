package shelves_test

import (
	"encoding/json/v2"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/voilelab/plainshelf/server/contract/apitest"

	"github.com/voilelab/plainshelf/internal/testutil"
	"github.com/voilelab/plainshelf/server"
	"github.com/voilelab/plainshelf/shelf"
)

// readExportedBookCache returns the single book cache file the app wrote into
// the shelf's app folder, waiting for it to appear. Its name carries the
// installation's writer ID, which is generated on first start and is therefore
// not predictable from a test.
func readExportedBookCache(t *testing.T, libRoot string) shelf.BookCacheFile {
	t.Helper()

	raw := testutil.WaitForExportedBookCache(t, libRoot)
	var cache shelf.BookCacheFile
	if err := json.Unmarshal(raw, &cache); err != nil {
		t.Fatalf("decode exported book cache: %v", err)
	}
	return cache
}

func TestAPIExportBookCacheContract(t *testing.T) {
	env := apitest.New(t)
	book := apitest.ImportTextBook(t, env, "Exported Book", "Fiction", "exported.txt", "Some content.")

	rec := env.Post(apitest.BookCacheExportURL(), nil)
	apitest.AssertStatus(t, rec, http.StatusOK)
	apitest.AssertJSONContentType(t, rec)

	resp := apitest.DecodeJSON[server.BookCacheExportResponse](t, rec)
	if resp.Timestamp <= 0 {
		t.Fatalf("timestamp = %d, want the Unix time of the walk", resp.Timestamp)
	}

	cache := readExportedBookCache(t, env.LibRoot)
	if cache.SchemaVersion != shelf.BookCacheSchemaVersion {
		t.Errorf("schema_version = %d, want %d", cache.SchemaVersion, shelf.BookCacheSchemaVersion)
	}
	if cache.WriterID == "" {
		t.Error("writer_id is empty; the server should have generated and persisted one")
	}
	if cache.Timestamp != resp.Timestamp {
		t.Errorf("file timestamp = %d, response timestamp = %d; they must agree", cache.Timestamp, resp.Timestamp)
	}
	entry, ok := cache.Books[book.Meta.ID]
	if !ok {
		t.Fatalf("imported book %q missing from the exported cache: %v", book.Meta.ID, cache.Books)
	}
	// The path is what lets a client match a package it found on the remote
	// storage against this entry, so it has to be the real package location.
	if !strings.HasPrefix(entry.Path, "books/Fiction/") {
		t.Errorf("path = %q, want the package under books/Fiction/", entry.Path)
	}
	if entry.Meta == nil || entry.Meta.Title != "Exported Book" {
		t.Errorf("exported meta = %+v, want the book.json of the imported book", entry.Meta)
	}

	hasFiction := slices.Contains(cache.Folders, "Fiction")
	if !hasFiction {
		t.Errorf("folders = %v, want the Fiction folder", cache.Folders)
	}
}

// A shelf opened after startup — the desktop "add shelf" flow — must get the
// installation's writer ID too. Without it the new shelf exports nothing and
// its manual export fails until the app is restarted.
//
// The writer ID's other half — that it survives a restart at all — needs no
// request and is pinned in server's own tests.
func TestBookCacheWriterIDAppliesToShelvesAddedAtRuntime(t *testing.T) {
	env := apitest.New(t)

	// The startup export runs on a timer, so wait for the file rather than for
	// the interval.
	startupWriterID := readExportedBookCache(t, env.LibRoot).WriterID

	addedRoot := t.TempDir()
	if err := env.App.AddShelf(shelf.ShelfConfWithID{
		ID:        "added_later",
		Name:      "Added Later",
		ShelfConf: shelf.ShelfConf{LibRoot: addedRoot},
	}); err != nil {
		t.Fatalf("AddShelf: %v", err)
	}

	shelfData, ok := env.App.ShelfManager().GetShelf("added_later")
	if !ok {
		t.Fatal("added_later missing from the shelf manager")
	}
	if err := shelfData.WaitReady(t.Context()); err != nil {
		t.Fatalf("WaitReady: %v", err)
	}

	rec := env.Post(apitest.ShelfIDURL("added_later", "book-cache-exports"), nil)
	apitest.AssertStatus(t, rec, http.StatusOK)

	added := readExportedBookCache(t, addedRoot)
	if added.WriterID != startupWriterID {
		t.Errorf("writer_id = %q, want the installation's %q", added.WriterID, startupWriterID)
	}
}
