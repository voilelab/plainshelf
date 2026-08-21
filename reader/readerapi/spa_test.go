package readerapi_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/voilelab/plainshelf/reader/readerapi"
)

func testAssets() fstest.MapFS {
	return fstest.MapFS{
		"index.html": &fstest.MapFile{
			Data: []byte("<html><head><title>PlainShelf</title></head><body><div id=\"app\"></div></body></html>"),
		},
		"assets/app.js": &fstest.MapFile{Data: []byte("console.log('app')")},
	}
}

func serveSPA(t *testing.T, boot readerapi.BootConfig, path string) *httptest.ResponseRecorder {
	t.Helper()

	handler := readerapi.NewSPAHandler(testAssets(), func() readerapi.BootConfig { return boot })
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
	return recorder
}

// The open book reaches the frontend through index.html rather than a request,
// so the shell can route straight to the reader on its first paint.
func TestSPAInjectsTheOpenBook(t *testing.T) {
	boot := readerapi.BootConfig{ShelfID: readerapi.ShelfID, BookID: testBookID}

	response := serveSPA(t, boot, "/")
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}

	body := response.Body.String()
	if !strings.Contains(body, "window.__PLAINSHELF_READER__") {
		t.Error("expected the boot config to be injected")
	}
	if !strings.Contains(body, `"book_id":"`+testBookID+`"`) {
		t.Errorf("expected the open book in the boot config, got:\n%s", body)
	}
	if !strings.Contains(body, `"shelf_id":"`+readerapi.ShelfID+`"`) {
		t.Error("expected the shelf id in the boot config")
	}

	// Ahead of the app's own scripts, which read the flag as they start.
	if strings.Index(body, "window.__PLAINSHELF_READER__") > strings.Index(body, "</head>") {
		t.Error("expected the boot config before </head>")
	}
}

// The window reloads itself at /reader/{id} when a book is opened, and that is
// a Vue route rather than a file on disk.
func TestSPAServesInAppRoutes(t *testing.T) {
	boot := readerapi.BootConfig{ShelfID: readerapi.ShelfID, BookID: testBookID}

	response := serveSPA(t, boot, "/reader/"+testBookID)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if !strings.Contains(response.Body.String(), `id="app"`) {
		t.Error("expected an in-app route to be answered with index.html")
	}
}

// A real asset is served as itself: injecting into a JavaScript bundle would
// corrupt it.
func TestSPAServesAssetsUntouched(t *testing.T) {
	response := serveSPA(t, readerapi.BootConfig{}, "/assets/app.js")
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if body := response.Body.String(); body != "console.log('app')" {
		t.Errorf("asset body = %q, want it unchanged", body)
	}
}

// Before the user has picked a folder there is no book, and the shell has to be
// able to tell that apart from a book it failed to load.
func TestSPAMarksTheReaderRuntimeWithNoBook(t *testing.T) {
	response := serveSPA(t, readerapi.BootConfig{ShelfID: readerapi.ShelfID}, "/")

	body := response.Body.String()
	if !strings.Contains(body, "window.__PLAINSHELF_READER__") {
		t.Error("expected the reader marker even with no book open")
	}
	if !strings.Contains(body, `"book_id":""`) {
		t.Errorf("expected an empty book id, got:\n%s", body)
	}
}
