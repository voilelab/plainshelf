package readerapi_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/voilelab/plainshelf/reader/readerapi"
	"github.com/voilelab/plainshelf/shelf/bookpkg"
)

const (
	testBookID   = "dune1234"
	testSourceID = "20260315-a1"
	testContent  = "# Chapter 1\n\nA beginning is the time for taking care.\n"
)

// writeBookPackage builds a package the way a shelf would, through the same
// structs the server writes, so a field renamed in shelf/ breaks this test
// rather than the app.
func writeBookPackage(t *testing.T, title string, bookID string) string {
	t.Helper()

	dir := filepath.Join(t.TempDir(), title+".bookpkg")
	sourceDir := filepath.Join(dir, bookpkg.SourcesFolder, testSourceID)
	if err := os.MkdirAll(filepath.Join(sourceDir, "assets"), 0o755); err != nil {
		t.Fatalf("creating package: %v", err)
	}

	writeJSONFile(t, filepath.Join(dir, bookpkg.BookMetaFile), bookpkg.BookMeta{
		SchemaVersion: bookpkg.BookMetaSchemaVersion,
		ID:            bookID,
		Title:         title,
		Authors:       []string{"Frank Herbert"},
		Format:        "md",
		Cover:         "cover.png",
		CurrentSource: testSourceID,
	})
	writeJSONFile(t, filepath.Join(sourceDir, bookpkg.SourceMetaFile), bookpkg.SourceMeta{
		SchemaVersion: bookpkg.SourceMetaSchemaVersion,
		ID:            testSourceID,
		Format:        "md",
	})

	writeFile(t, filepath.Join(sourceDir, bookpkg.SourceFile), []byte(testContent))
	writeFile(t, filepath.Join(sourceDir, "assets", "figure.png"), []byte("figure bytes"))
	writeFile(t, filepath.Join(dir, "cover.png"), []byte("cover bytes"))

	return dir
}

func writeJSONFile(t *testing.T, path string, value any) {
	t.Helper()

	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("encoding %s: %v", path, err)
	}
	writeFile(t, path, encoded)
}

func writeFile(t *testing.T, path string, data []byte) {
	t.Helper()

	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
}

func newOpenLibrary(t *testing.T) *readerapi.Library {
	t.Helper()

	library := readerapi.NewLibrary(nil)
	t.Cleanup(func() { library.Close() }) //nolint:errcheck // test cleanup

	if _, err := library.Open(writeBookPackage(t, "Dune", testBookID)); err != nil {
		t.Fatalf("Open: %v", err)
	}
	return library
}

func get(t *testing.T, handler http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
	return recorder
}

func bookPath(bookID string) string {
	return "/api/shelves/" + readerapi.ShelfID + "/books/" + bookID
}

// The response shape is the frontend's contract: it reads metadata from "meta"
// and the folder from beside it, and a package has none.
func TestHandlerServesTheOpenBook(t *testing.T) {
	handler := readerapi.NewHandler(newOpenLibrary(t), "test")

	response := get(t, handler, bookPath(testBookID))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (%s)", response.Code, http.StatusOK, response.Body)
	}

	var payload struct {
		Meta   bookpkg.BookMeta `json:"meta"`
		Folder []string         `json:"folder"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decoding book: %v", err)
	}
	if payload.Meta.ID != testBookID {
		t.Errorf("book id = %q, want %q", payload.Meta.ID, testBookID)
	}
	if payload.Meta.Title != "Dune" {
		t.Errorf("title = %q, want %q", payload.Meta.Title, "Dune")
	}
	if payload.Folder == nil {
		t.Error("expected an empty folder list rather than null")
	}
	if len(payload.Folder) != 0 {
		t.Errorf("folder = %v, want none", payload.Folder)
	}
}

func TestHandlerServesContentAndSources(t *testing.T) {
	handler := readerapi.NewHandler(newOpenLibrary(t), "test")

	for _, path := range []string{
		bookPath(testBookID) + "/content",
		bookPath(testBookID) + "/sources/" + testSourceID + "/content",
	} {
		response := get(t, handler, path)
		if response.Code != http.StatusOK {
			t.Errorf("GET %s status = %d, want %d", path, response.Code, http.StatusOK)
			continue
		}
		if response.Body.String() != testContent {
			t.Errorf("GET %s body = %q, want %q", path, response.Body.String(), testContent)
		}
		if contentType := response.Header().Get("Content-Type"); !strings.HasPrefix(contentType, "text/plain") {
			t.Errorf("GET %s content type = %q, want text/plain", path, contentType)
		}
	}

	response := get(t, handler, bookPath(testBookID)+"/sources")
	if response.Code != http.StatusOK {
		t.Fatalf("listing sources: status = %d", response.Code)
	}

	var sources []bookpkg.SourceMeta
	if err := json.Unmarshal(response.Body.Bytes(), &sources); err != nil {
		t.Fatalf("decoding sources: %v", err)
	}
	if len(sources) != 1 || sources[0].ID != testSourceID {
		t.Errorf("sources = %+v, want one entry with id %q", sources, testSourceID)
	}
}

// Cover and illustrations reach the UI as bytes with a type the browser can
// render, not as octet-stream downloads.
func TestHandlerServesImagesWithTheirType(t *testing.T) {
	handler := readerapi.NewHandler(newOpenLibrary(t), "test")

	images := map[string]string{
		bookPath(testBookID) + "/cover":                                          "cover bytes",
		bookPath(testBookID) + "/sources/" + testSourceID + "/assets/figure.png": "figure bytes",
	}
	for path, want := range images {
		response := get(t, handler, path)
		if response.Code != http.StatusOK {
			t.Errorf("GET %s status = %d, want %d", path, response.Code, http.StatusOK)
			continue
		}
		if response.Body.String() != want {
			t.Errorf("GET %s body = %q, want %q", path, response.Body.String(), want)
		}
		if contentType := response.Header().Get("Content-Type"); !strings.HasPrefix(contentType, "image/png") {
			t.Errorf("GET %s content type = %q, want image/png", path, contentType)
		}
	}
}

// A reader that has not been given a folder yet is not broken, and a request
// for some other book is not this reader's business.
func TestHandlerReportsWhatItDoesNotHave(t *testing.T) {
	t.Run("no book open", func(t *testing.T) {
		handler := readerapi.NewHandler(readerapi.NewLibrary(nil), "test")

		response := get(t, handler, bookPath(testBookID))
		if response.Code != http.StatusServiceUnavailable {
			t.Errorf("status = %d, want %d", response.Code, http.StatusServiceUnavailable)
		}
	})

	t.Run("another book", func(t *testing.T) {
		handler := readerapi.NewHandler(newOpenLibrary(t), "test")

		response := get(t, handler, bookPath("someoneelse"))
		if response.Code != http.StatusNotFound {
			t.Errorf("status = %d, want %d", response.Code, http.StatusNotFound)
		}
	})

	t.Run("a route the reader does not serve", func(t *testing.T) {
		handler := readerapi.NewHandler(newOpenLibrary(t), "test")

		for _, path := range []string{
			"/api/shelves/" + readerapi.ShelfID + "/books",
			"/api/shelves/" + readerapi.ShelfID + "/folders",
			"/api/shelves/" + readerapi.ShelfID + "/trash/books",
			"/api/logs",
		} {
			if response := get(t, handler, path); response.Code != http.StatusNotFound {
				t.Errorf("GET %s status = %d, want %d", path, response.Code, http.StatusNotFound)
			}
		}
	})
}

// The reader has no write surface at all: the methods the editing UI would use
// are refused by the router, before any handler can be reached.
func TestHandlerRefusesWrites(t *testing.T) {
	handler := readerapi.NewHandler(newOpenLibrary(t), "test")

	writes := []struct {
		method string
		path   string
	}{
		{http.MethodPatch, bookPath(testBookID)},
		{http.MethodDelete, bookPath(testBookID)},
		{http.MethodPut, bookPath(testBookID) + "/cover"},
		{http.MethodPost, bookPath(testBookID) + "/sources"},
		{http.MethodPatch, bookPath(testBookID) + "/sources/" + testSourceID + "/content"},
	}
	for _, write := range writes {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(write.method, write.path, strings.NewReader("{}")))
		if recorder.Code < 400 {
			t.Errorf("%s %s status = %d, want a refusal", write.method, write.path, recorder.Code)
		}
	}
}

// Opening another folder swaps the book the window serves, and the previous
// book stops being addressable — the reader holds exactly one.
func TestLibraryOpensAnotherBook(t *testing.T) {
	library := newOpenLibrary(t)
	handler := readerapi.NewHandler(library, "test")

	const nextBookID = "emma5678"
	if _, err := library.Open(writeBookPackage(t, "Emma", nextBookID)); err != nil {
		t.Fatalf("Open: %v", err)
	}

	if library.BookID() != nextBookID {
		t.Errorf("open book = %q, want %q", library.BookID(), nextBookID)
	}
	if response := get(t, handler, bookPath(nextBookID)); response.Code != http.StatusOK {
		t.Errorf("new book status = %d, want %d", response.Code, http.StatusOK)
	}
	if response := get(t, handler, bookPath(testBookID)); response.Code != http.StatusNotFound {
		t.Errorf("previous book status = %d, want %d", response.Code, http.StatusNotFound)
	}
}

// A folder the user picked by mistake must not take the book they were reading
// away from them.
func TestLibraryKeepsTheOpenBookWhenAnOpenFails(t *testing.T) {
	library := newOpenLibrary(t)

	if _, err := library.Open(t.TempDir()); err == nil {
		t.Fatal("expected opening a folder without book.json to fail")
	}
	if library.BookID() != testBookID {
		t.Errorf("open book = %q, want the previous book %q", library.BookID(), testBookID)
	}
}

func TestHandlerReportsReadOnlyMode(t *testing.T) {
	handler := readerapi.NewHandler(newOpenLibrary(t), "test-version")

	response := get(t, handler, "/api/mode")
	var mode struct {
		ReadOnly bool `json:"read_only"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &mode); err != nil {
		t.Fatalf("decoding mode: %v", err)
	}
	if !mode.ReadOnly {
		t.Error("expected the reader to report read-only")
	}

	response = get(t, handler, "/api/version")
	var reported struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &reported); err != nil {
		t.Fatalf("decoding version: %v", err)
	}
	if reported.Version != "test-version" {
		t.Errorf("version = %q, want %q", reported.Version, "test-version")
	}
}
