package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/voilelab/plainshelf/shelf"
)

const bookCacheExportPath = "/api/shelves/default_shelf/book-cache-exports"

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

	rec := env.do(httptest.NewRequest(http.MethodPost, bookCacheExportPath, nil))
	assertStatus(t, rec, http.StatusOK)
	assertJSONContentType(t, rec)

	resp := decodeJSON[BookCacheExportResponse](t, rec)
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

	var hasFiction bool
	for _, layer := range cache.Layers {
		if layer == "Fiction" {
			hasFiction = true
		}
	}
	if !hasFiction {
		t.Errorf("layers = %v, want the Fiction layer", cache.Layers)
	}
}

func TestAPIExportBookCacheRejectsUnknownShelfContract(t *testing.T) {
	env := newAPITestEnv(t)

	rec := env.do(httptest.NewRequest(http.MethodPost, "/api/shelves/missing_shelf/book-cache-exports", nil))
	assertStatus(t, rec, http.StatusNotFound)
}

// The endpoint writes into the shelf, so it must sit inside the local_token
// boundary and be refused in read-only mode.
func TestAPIExportBookCacheRequiresTokenContract(t *testing.T) {
	env := newAPITestEnv(t)

	rec := env.doRaw(httptest.NewRequest(http.MethodPost, bookCacheExportPath, nil))
	assertStatus(t, rec, http.StatusUnauthorized)
}

func TestAPIExportBookCacheRejectedInReadOnlyModeContract(t *testing.T) {
	env := newAPITestEnv(t)
	env.app.conf.ReadOnly = true

	rec := env.do(httptest.NewRequest(http.MethodPost, bookCacheExportPath, nil))
	assertStatus(t, rec, http.StatusForbidden)
}

// The writer ID identifies the installation, so it has to outlive a restart:
// two runs against the same store must keep writing the same file rather than
// leaving a new one behind on every start.
func TestBookCacheWriterIDIsStableAcrossRestarts(t *testing.T) {
	storePath := t.TempDir()
	libRoot := t.TempDir()

	newRun := func() string {
		app, err := NewApp(&AppConf{
			Shelves: []*shelf.ShelfConfWithID{
				{ID: "default_shelf", ShelfConf: shelf.ShelfConf{LibRoot: libRoot}},
			},
			StorePath: storePath,
		})
		if err != nil {
			t.Fatalf("NewApp: %v", err)
		}
		defer func() {
			if err := app.Close(); err != nil {
				t.Fatalf("Close app: %v", err)
			}
		}()

		shelfData, ok := app.shelfManager.GetShelf("default_shelf")
		if !ok {
			t.Fatal("default_shelf missing")
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
