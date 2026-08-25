package contract_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/voilelab/plainshelf/server"
	"github.com/voilelab/plainshelf/shelf"
)

func scansURL() string {
	return shelfURL("scans")
}

// writeBookIntoShelf drops a book package straight into books/, the way copying
// a folder in from outside PlainShelf does. Nothing tells the server about it,
// so only a walk of the tree can find it.
func writeBookIntoShelf(t *testing.T, libRoot, dirName, bookID, title string) {
	t.Helper()

	bookDir := filepath.Join(libRoot, "books", dirName+".bookpkg")
	if err := os.MkdirAll(bookDir, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	data, err := json.Marshal(map[string]any{
		"schema_version": shelf.BookMetaSchemaVersion,
		"id":             bookID,
		"title":          title,
	})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if err := os.WriteFile(filepath.Join(bookDir, shelf.BookMetaFile), data, 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}

func TestAPIRescanShelfContract(t *testing.T) {
	env := newAPITestEnv(t)
	importTextBook(t, env, "Already Here", "Fiction", "here.txt", "Some content.")

	// Added the way a user does when they ask why the book has not appeared:
	// from outside PlainShelf, with no request to tell the server about it.
	writeBookIntoShelf(t, env.libRoot, "dropped-in", "drop1nkb", "Dropped In")

	rec := env.post(scansURL(), nil)
	assertStatus(t, rec, http.StatusOK)
	assertJSONContentType(t, rec)

	scan := decodeJSON[server.ScanResponse](t, rec)
	if scan.ScanID == "" {
		t.Error("scan_id is empty")
	}
	if scan.ScannedAt <= 0 {
		t.Errorf("scanned_at = %d, want the Unix time the walk began", scan.ScannedAt)
	}
	if scan.BookCount != 2 {
		t.Errorf("book_count = %d, want 2", scan.BookCount)
	}
	// "/" and "Fiction".
	if scan.FolderCount != 2 {
		t.Errorf("folder_count = %d, want 2", scan.FolderCount)
	}

	// The counts are only worth reporting if the listing agrees with them.
	books := getJSON[[]server.Book](t, env, booksURL())
	if len(books) != 2 {
		t.Fatalf("books after the rescan = %d, want the externally added book to be listed", len(books))
	}
}

// A rescan reads the shelf and writes nothing to it, so read-only mode has no
// reason to refuse it — unlike every other POST, which assertMutationGated
// pins as refused.
func TestAPIRescanShelfIsAllowedInReadOnlyModeContract(t *testing.T) {
	env := newAPITestEnv(t)
	env.setReadOnly(t, true)

	rec := env.post(scansURL(), nil)
	assertStatus(t, rec, http.StatusOK)
}

// The read-only exemption is not a token exemption: a rescan still costs the
// server a full walk, so it stays behind the local_token boundary.
func TestAPIRescanShelfStillRequiresTheTokenContract(t *testing.T) {
	env := newAPITestEnv(t)

	rec := env.doRaw(httptest.NewRequest(http.MethodPost, scansURL(), nil))
	assertStatus(t, rec, http.StatusUnauthorized)
}

// The exemption is matched on the path, so it must not open read-only mode for
// a neighbouring route or for a shelf-less path that happens to end in /scans.
func TestAPIReadOnlyExemptionIsLimitedToTheScanRouteContract(t *testing.T) {
	env := newAPITestEnv(t)
	env.setReadOnly(t, true)

	for _, url := range []string{
		shelfURL("scans", "extra"),
		shelfURL("book-cache-exports"),
		"/api/shelves//scans",
		"/api/scans",
	} {
		t.Run(url, func(t *testing.T) {
			rec := env.post(url, nil)
			assertStatus(t, rec, http.StatusForbidden)
		})
	}
}

func TestAPIRescanUnknownShelfContract(t *testing.T) {
	env := newAPITestEnv(t)

	rec := env.post(shelfIDURL("missing_shelf", "scans"), nil)
	assertStatus(t, rec, http.StatusNotFound)
}
