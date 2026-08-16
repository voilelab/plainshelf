package contract_test

import (
	"net/http"
	"slices"
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

	// A derived source is uploaded as multipart so the whole book is not placed
	// in metadata JSON. Creation and optional activation happen in one request.
	derivedUpload := formUpload{
		fields: [][2]string{
			{"format", "md"},
			{"comment", "derived in contract test"},
			{"set_current", "true"},
		},
		fileField:   "content",
		filename:    "source.txt",
		contentType: "application/octet-stream",
		content:     "# Book\n\n## One\nBody 🍥",
	}
	rec = env.do(derivedUpload.request(t, http.MethodPost, bookURL(created.Meta.ID, "sources")))
	assertStatus(t, rec, http.StatusOK)
	derived := decodeJSON[map[string]any](t, rec)
	derivedID, _ := derived["id"].(string)
	if derived["format"] != "md" || derived["schema_version"] != float64(1) {
		t.Fatalf("derived source metadata = %#v, want schema 1 Markdown", derived)
	}
	if derived["comment"] != "derived in contract test" {
		t.Fatalf("derived source comment = %#v", derived["comment"])
	}

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
