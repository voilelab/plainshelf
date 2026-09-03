package contract_test

import (
	"encoding/json/v2"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/voilelab/plainshelf/server"
	"github.com/voilelab/plainshelf/shelf"
)

func bookCacheExportURL() string {
	return shelfURL("book-cache-exports")
}

// readExportedBookCache returns the single book cache file the app wrote into
// the shelf's app folder. Its name carries the installation's writer ID, which
// is generated on first start and is therefore not predictable from a test.
func readExportedBookCache(t *testing.T, libRoot string) shelf.BookCacheFile {
	t.Helper()

	appDir := filepath.Join(libRoot, "app")
	entries, err := os.ReadDir(appDir)
	if err != nil {
		t.Fatalf("read app dir: %v", err)
	}

	var found string
	for _, entry := range entries {
		name := entry.Name()
		if !entry.IsDir() && strings.HasPrefix(name, "book-cache-") && strings.HasSuffix(name, ".json") {
			if found != "" {
				t.Fatalf("expected exactly one exported book cache, also found %s", name)
			}
			found = filepath.Join(appDir, name)
		}
	}
	if found == "" {
		t.Fatalf("no exported book cache under %s", appDir)
	}

	raw, err := os.ReadFile(found)
	if err != nil {
		t.Fatalf("read exported book cache: %v", err)
	}
	var cache shelf.BookCacheFile
	if err := json.Unmarshal(raw, &cache); err != nil {
		t.Fatalf("decode exported book cache: %v", err)
	}
	return cache
}

func TestAPIExportBookCacheContract(t *testing.T) {
	env := newAPITestEnv(t)
	book := importTextBook(t, env, "Exported Book", "Fiction", "exported.txt", "Some content.")

	rec := env.post(bookCacheExportURL(), nil)
	assertStatus(t, rec, http.StatusOK)
	assertJSONContentType(t, rec)

	resp := decodeJSON[server.BookCacheExportResponse](t, rec)
	if resp.Timestamp <= 0 {
		t.Fatalf("timestamp = %d, want the Unix time of the walk", resp.Timestamp)
	}

	cache := readExportedBookCache(t, env.libRoot)
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

// The writer ID identifies the installation, so it has to outlive a restart:
// two runs against the same store must keep writing the same file rather than
// leaving a new one behind on every start.
func TestBookCacheWriterIDIsStableAcrossRestarts(t *testing.T) {
	storePath := t.TempDir()
	libRoot := t.TempDir()

	newRun := func() string {
		app, err := server.NewApp(apiAppConf(t, withLibRoot(libRoot), withStorePath(storePath)))
		if err != nil {
			t.Fatalf("NewApp: %v", err)
		}
		defer func() {
			if err := app.Close(); err != nil {
				t.Fatalf("Close app: %v", err)
			}
		}()

		shelfData, ok := app.ShelfManager().GetShelf(defaultShelfID)
		if !ok {
			t.Fatalf("%s missing", defaultShelfID)
		}
		if err := shelfData.WaitReady(t.Context()); err != nil {
			t.Fatalf("WaitReady: %v", err)
		}
		if _, err := shelfData.ExportBookCache(); err != nil {
			t.Fatalf("ExportBookCache: %v", err)
		}
		return readExportedBookCache(t, libRoot).WriterID
	}

	first := newRun()
	second := newRun()
	if first == "" || first != second {
		t.Fatalf("writer ID changed across restarts: %q then %q", first, second)
	}
	// readExportedBookCache fails when a second file appears, so reaching here
	// also proves the restart did not orphan the first run's cache.
}

// A shelf opened after startup — the desktop "add shelf" flow — must get the
// installation's writer ID too. Without it the new shelf exports nothing and
// its manual export fails until the app is restarted.
func TestBookCacheWriterIDAppliesToShelvesAddedAtRuntime(t *testing.T) {
	env := newAPITestEnv(t)

	// Wait a moment for the initial book cache export to finish.
	time.Sleep(2 * time.Second)

	startupWriterID := readExportedBookCache(t, env.libRoot).WriterID

	addedRoot := t.TempDir()
	if err := env.app.AddShelf(shelf.ShelfConfWithID{
		ID:        "added_later",
		Name:      "Added Later",
		ShelfConf: shelf.ShelfConf{LibRoot: addedRoot},
	}); err != nil {
		t.Fatalf("AddShelf: %v", err)
	}

	shelfData, ok := env.app.ShelfManager().GetShelf("added_later")
	if !ok {
		t.Fatal("added_later missing from the shelf manager")
	}
	if err := shelfData.WaitReady(t.Context()); err != nil {
		t.Fatalf("WaitReady: %v", err)
	}

	rec := env.post(shelfIDURL("added_later", "book-cache-exports"), nil)
	assertStatus(t, rec, http.StatusOK)

	added := readExportedBookCache(t, addedRoot)
	if added.WriterID != startupWriterID {
		t.Errorf("writer_id = %q, want the installation's %q", added.WriterID, startupWriterID)
	}
}
