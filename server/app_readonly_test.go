package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/voilelab/plainshelf/shelf"
)

func newReadOnlyTestApp(t *testing.T) *App {
	t.Helper()

	app, err := NewApp(&AppConf{
		Shelves: []*shelf.ShelfConfWithID{
			{
				ID:      "default_shelf",
				LibRoot: t.TempDir(),
			},
		},
		StorePath: t.TempDir(),
		ReadOnly:  true,
	})
	if err != nil {
		t.Fatalf("NewApp: %v", err)
	}
	t.Cleanup(func() {
		if err := app.Close(); err != nil {
			t.Fatalf("Close app: %v", err)
		}
	})

	if err := app.Start(); err != nil {
		t.Fatalf("Start app: %v", err)
	}
	waitForInitialShelfScans(t, app)

	return app
}

func TestReadOnlyServerOpensEveryShelfReadOnly(t *testing.T) {
	app := newReadOnlyTestApp(t)

	shelfData, ok := app.ShelfManager().GetShelf("default_shelf")
	if !ok {
		t.Fatal("default_shelf missing")
	}
	if !shelfData.ReadOnly() {
		t.Error("configured shelf ReadOnly() = false, want the app-wide read_only to reach it")
	}

	addedRoot := t.TempDir()
	if err := app.AddShelf(shelf.ShelfConfWithID{
		ID:      "added_later",
		Name:    "Added Later",
		LibRoot: addedRoot,
	}); err != nil {
		t.Fatalf("AddShelf: %v", err)
	}

	added, ok := app.ShelfManager().GetShelf("added_later")
	if !ok {
		t.Fatal("added_later missing from the shelf manager")
	}
	if !added.ReadOnly() {
		t.Error("added shelf ReadOnly() = false, want the app-wide read_only to reach it too")
	}

	if _, err := added.ExportBookCache(); err == nil {
		t.Error("ExportBookCache on a read-only server succeeded, want a refusal")
	}
}

func TestReadOnlyServerDoesNotCreateTheShelf(t *testing.T) {
	libRoot := filepath.Join(t.TempDir(), "missing-shelf")

	app, err := NewApp(&AppConf{
		Shelves: []*shelf.ShelfConfWithID{
			{
				ID:      "default_shelf",
				LibRoot: libRoot,
			},
		},
		StorePath: t.TempDir(),
		ReadOnly:  true,
	})
	if err == nil {
		app.Close()
		t.Fatal("NewApp succeeded on a missing lib_root, want a failure rather than a created shelf")
	}
	if _, statErr := os.Stat(libRoot); !os.IsNotExist(statErr) {
		t.Errorf("stat %s = %v, want the path still missing", libRoot, statErr)
	}
}

func TestReadOnlySafeRequestMatchesOnlyItsRoutes(t *testing.T) {
	cases := []struct {
		method, path string
		want         bool
	}{
		{http.MethodPost, "/api/shelves/s/scans", true},
		{http.MethodPut, "/api/shelves/s/books/b/download", true},
		{http.MethodDelete, "/api/shelves/s/books/b/download", true},
		{http.MethodPost, "/api/shelves/s/books/b/download", false},
		{http.MethodPut, "/api/shelves/s/books/b/downloads", false},
		{http.MethodPut, "/api/shelves/s/books//download", false},
		{http.MethodPut, "/api/shelves/s/books/b/sources/x/download", false},
		{http.MethodPut, "/api/shelves/s/folders/b/download", false},
		{http.MethodPatch, "/api/shelves/s/books/b", false},
		{http.MethodPost, "/api/shelves//scans", false},
		{http.MethodPost, "/api/shelves/s/scans/", false},
		{http.MethodPost, "/api/shelves/s/scans/nested", false},
		{http.MethodPost, "/api/shelves/s//scans", false},
		{http.MethodPut, "/api/shelves/s/books/b/download/", false},
		{http.MethodPut, "/api/shelves/s/books/b/download/nested", false},
		{http.MethodPut, "/api/shelves//books/b/download", false},
		{http.MethodPut, "/api/shelves/s/books/b/../download", false},
		{http.MethodGet, "/api/shelves/s/scans", false},
		{http.MethodHead, "/api/shelves/s/books/b/download", false},
	}
	for _, c := range cases {
		r := httptest.NewRequest(c.method, c.path, nil)
		if got := isReadOnlySafeRequest(r); got != c.want {
			t.Errorf("isReadOnlySafeRequest(%s %s) = %v, want %v", c.method, c.path, got, c.want)
		}
		wantScan := c.want && c.method == http.MethodPost
		if got := isShelfScanRequest(r); got != wantScan {
			t.Errorf("isShelfScanRequest(%s %s) = %v, want %v", c.method, c.path, got, wantScan)
		}
	}

}
