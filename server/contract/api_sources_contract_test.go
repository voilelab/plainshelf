package contract_test

import (
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/voilelab/plainshelf/server"
)

// createBookSource creates an empty source on a book and returns its ID, which is
// how these tests get a second source to delete or activate.
func createBookSource(t *testing.T, env *apiTestEnv, bookID string) string {
	t.Helper()

	rec := env.post(bookURL(bookID, "sources"), nil)
	assertStatus(t, rec, http.StatusOK)
	assertJSONContentType(t, rec)

	created := decodeJSON[map[string]any](t, rec)
	sourceID, _ := created["id"].(string)
	if sourceID == "" {
		t.Fatalf("expected non-empty source id in response, got %#v", created)
	}
	return sourceID
}

// sourceIDs lists the IDs the sources endpoint reports for a book.
func sourceIDs(t *testing.T, env *apiTestEnv, bookID string) []string {
	t.Helper()

	var ids []string
	for _, source := range getJSON[[]map[string]any](t, env, bookURL(bookID, "sources")) {
		id, _ := source["id"].(string)
		ids = append(ids, id)
	}
	return ids
}

func TestAPICreateBookSourceContract(t *testing.T) {
	env := newAPITestEnv(t)
	created := importTextBook(t, env, "Source Book", "", "src.txt", "content")

	// Creating a source on a nonexistent book should return 404.
	rec := env.post(bookURL("no-such-book", "sources"), nil)
	assertStatus(t, rec, http.StatusNotFound)

	// Creating a source returns 200 with the new source metadata, and the new
	// source appears in the list.
	newSourceID := createBookSource(t, env, created.Meta.ID)
	if ids := sourceIDs(t, env, created.Meta.ID); !slices.Contains(ids, newSourceID) {
		t.Fatalf("newly created source %q not found in list: %#v", newSourceID, ids)
	}

	// A derived source uses JSON so Wails does not have to stream generated
	// multipart data through WebKit's custom-scheme handler. Creation and
	// optional activation still happen atomically in one request.
	derivedBody := `{"content":"# Book\n\n## One\nBody 🍥","format":"md","comment":"derived in contract test","set_current":true}`
	rec = env.requestTyped(http.MethodPost, bookURL(created.Meta.ID, "sources"),
		"application/json; charset=utf-8", strings.NewReader(derivedBody))
	assertStatus(t, rec, http.StatusOK)
	derived := decodeJSON[map[string]any](t, rec)
	derivedID, _ := derived["id"].(string)
	if derived["format"] != "md" || derived["schema_version"] != float64(1) {
		t.Fatalf("derived source metadata = %#v, want schema 1 Markdown", derived)
	}
	if derived["comment"] != "derived in contract test" {
		t.Fatalf("derived source comment = %#v", derived["comment"])
	}

	contentRec := env.get(sourceURL(created.Meta.ID, derivedID, "content"))
	assertStatus(t, contentRec, http.StatusOK)
	if got := contentRec.Body.String(); got != "# Book\n\n## One\nBody 🍥" {
		t.Fatalf("derived source content = %q", got)
	}

	// Keep accepting multipart requests from clients released before the JSON
	// transport was introduced.
	legacyUpload := formUpload{
		fields: [][2]string{
			{"format", "txt"},
			{"comment", "legacy multipart client"},
		},
		fileField:   "content",
		filename:    "source.txt",
		contentType: "application/octet-stream",
		content:     "legacy body",
	}
	rec = env.do(legacyUpload.request(t, http.MethodPost, bookURL(created.Meta.ID, "sources")))
	assertStatus(t, rec, http.StatusOK)

	activated := getJSON[server.Book](t, env, bookURL(created.Meta.ID))
	if activated.Meta == nil || activated.Meta.CurrentSource != derivedID || activated.Meta.Format != "md" {
		t.Fatalf("activated book = %#v, want current derived Markdown source", activated.Meta)
	}
}

func TestAPIDeleteBookSourceContract(t *testing.T) {
	env := newAPITestEnv(t)
	created := importTextBook(t, env, "Delete Source Book", "", "del.txt", "content")
	newSourceID := createBookSource(t, env, created.Meta.ID)

	// Deleting the source should succeed, and it should leave the list.
	rec := env.delete(sourceURL(created.Meta.ID, newSourceID))
	assertStatus(t, rec, http.StatusNoContent)
	if ids := sourceIDs(t, env, created.Meta.ID); slices.Contains(ids, newSourceID) {
		t.Fatalf("deleted source %q still present in list", newSourceID)
	}

	// Deleting a nonexistent source, or a source of a nonexistent book, is a 404.
	rec = env.delete(sourceURL(created.Meta.ID, "nonexistent-source"))
	assertStatus(t, rec, http.StatusNotFound)
	rec = env.delete(sourceURL("no-such-book", newSourceID))
	assertStatus(t, rec, http.StatusNotFound)
}

func TestAPISetCurrentBookSourceContract(t *testing.T) {
	env := newAPITestEnv(t)
	created := importTextBook(t, env, "Set Current Source Book", "", "src.txt", "content")
	newSourceID := createBookSource(t, env, created.Meta.ID)

	// Setting the current source should succeed.
	rec := env.put(sourceURL(created.Meta.ID, newSourceID, "current"), nil)
	assertStatus(t, rec, http.StatusNoContent)

	// The book should reflect the new current source.
	bookData := getJSON[map[string]any](t, env, bookURL(created.Meta.ID))
	meta, _ := bookData["meta"].(map[string]any)
	if currentSource, _ := meta["current_source"].(string); currentSource != newSourceID {
		t.Fatalf("expected current_source %q, got %q", newSourceID, currentSource)
	}

	// A nonexistent source, or a nonexistent book, is a 404.
	rec = env.put(sourceURL(created.Meta.ID, "nonexistent-source", "current"), nil)
	assertStatus(t, rec, http.StatusNotFound)
	rec = env.put(sourceURL("no-such-book", newSourceID, "current"), nil)
	assertStatus(t, rec, http.StatusNotFound)
}

func TestAPIRefreshBookSourceMetaContract(t *testing.T) {
	env := newAPITestEnv(t)
	created := importTextBook(t, env, "Refresh Source", "", "refresh.txt", "line one\nline two\nline three")
	sourceID := created.Meta.CurrentSource

	rec := env.post(sourceURL(created.Meta.ID, sourceID, "refresh"), nil)
	assertStatus(t, rec, http.StatusOK)
	assertJSONContentType(t, rec)
	meta := decodeJSON[map[string]any](t, rec)
	if id, _ := meta["id"].(string); id != sourceID {
		t.Fatalf("refreshed source id = %q, want %q", id, sourceID)
	}
	if lc, _ := meta["line_count"].(float64); lc <= 0 {
		t.Fatalf("line_count = %v, want > 0", lc)
	}
	if cc, _ := meta["char_count"].(float64); cc <= 0 {
		t.Fatalf("char_count = %v, want > 0", cc)
	}

	// Refreshing a nonexistent source, or a source of a nonexistent book, is a 404.
	rec = env.post(sourceURL(created.Meta.ID, "nonexistent", "refresh"), nil)
	assertStatus(t, rec, http.StatusNotFound)
	rec = env.post(sourceURL("no-such-book", sourceID, "refresh"), nil)
	assertStatus(t, rec, http.StatusNotFound)
}

// currentSourceOf reports the pointer the book endpoint publishes, which is what
// a client reads back after a source is deleted.
func currentSourceOf(t *testing.T, env *apiTestEnv, bookID string) string {
	t.Helper()

	book := getJSON[server.Book](t, env, bookURL(bookID))
	if book.Meta == nil {
		t.Fatalf("book %s carried no meta", bookID)
	}
	return book.Meta.CurrentSource
}

// TestAPIDeleteCurrentBookSourceContract pins the promise that deleting the
// active source leaves a readable book: the pointer moves to a source that
// exists instead of being left dangling, which used to fail every read.
func TestAPIDeleteCurrentBookSourceContract(t *testing.T) {
	env := newAPITestEnv(t)
	created := importTextBook(t, env, "Delete Current Source", "", "cur.txt", "imported body")
	importedID := created.Meta.CurrentSource

	newSourceID := createBookSource(t, env, created.Meta.ID)
	rec := env.put(sourceURL(created.Meta.ID, newSourceID, "current"), nil)
	assertStatus(t, rec, http.StatusNoContent)

	rec = env.delete(sourceURL(created.Meta.ID, newSourceID))
	assertStatus(t, rec, http.StatusNoContent)

	if got := currentSourceOf(t, env, created.Meta.ID); got != importedID {
		t.Fatalf("current_source = %q, want the surviving source %q", got, importedID)
	}
	if ids := sourceIDs(t, env, created.Meta.ID); !slices.Equal(ids, []string{importedID}) {
		t.Fatalf("sources = %#v, want only %q", ids, importedID)
	}
	assertStatus(t, env.get(bookURL(created.Meta.ID, "content")), http.StatusOK)
}

// TestAPIDeleteLastBookSourceContract pins the other half: a book always keeps
// at least one source, so deleting the only one leaves an empty replacement
// rather than a book with nothing to read.
func TestAPIDeleteLastBookSourceContract(t *testing.T) {
	env := newAPITestEnv(t)
	created := importTextBook(t, env, "Delete Last Source", "", "last.txt", "imported body")
	importedID := created.Meta.CurrentSource

	rec := env.delete(sourceURL(created.Meta.ID, importedID))
	assertStatus(t, rec, http.StatusNoContent)

	ids := sourceIDs(t, env, created.Meta.ID)
	if len(ids) != 1 {
		t.Fatalf("sources = %#v, want exactly the empty replacement", ids)
	}
	if ids[0] == importedID {
		t.Fatalf("replacement reused the deleted source ID %q", importedID)
	}
	if got := currentSourceOf(t, env, created.Meta.ID); got != ids[0] {
		t.Fatalf("current_source = %q, want the replacement %q", got, ids[0])
	}

	content := env.get(bookURL(created.Meta.ID, "content"))
	assertStatus(t, content, http.StatusOK)
	if body := content.Body.String(); body != "" {
		t.Fatalf("content = %q, want the replacement to be empty", body)
	}
}
