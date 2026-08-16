package contract_test

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/voilelab/plainshelf/server"
)

func TestAPICreateBookSourceContract(t *testing.T) {
	env := newAPITestEnv(t)
	created := importTextBook(t, env, "Source Book", "", "src.txt", "content")
	sourcesURL := "/api/shelves/default_shelf/books/" + created.Meta.ID + "/sources"

	// Creating a source on a nonexistent book should return 404.
	rec := env.do(httptest.NewRequest(http.MethodPost, "/api/shelves/default_shelf/books/no-such-book/sources", nil))
	assertStatus(t, rec, http.StatusNotFound)

	// Creating a source returns 200 with the new source metadata.
	rec = env.do(httptest.NewRequest(http.MethodPost, sourcesURL, nil))
	assertStatus(t, rec, http.StatusOK)
	assertJSONContentType(t, rec)
	newSource := decodeJSON[map[string]any](t, rec)
	newSourceID, _ := newSource["id"].(string)
	if newSourceID == "" {
		t.Fatalf("expected non-empty source id in response, got %#v", newSource)
	}

	// The new source should appear in the list.
	rec = env.do(httptest.NewRequest(http.MethodGet, sourcesURL, nil))
	assertStatus(t, rec, http.StatusOK)
	sources := decodeJSON[[]map[string]any](t, rec)
	found := false
	for _, s := range sources {
		if id, _ := s["id"].(string); id == newSourceID {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("newly created source %q not found in list: %#v", newSourceID, sources)
	}

	// A derived source is uploaded as multipart so the whole book is not placed
	// in metadata JSON. Creation and optional activation happen in one request.
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("format", "md"); err != nil {
		t.Fatalf("WriteField format: %v", err)
	}
	if err := writer.WriteField("comment", "derived in contract test"); err != nil {
		t.Fatalf("WriteField comment: %v", err)
	}
	if err := writer.WriteField("set_current", "true"); err != nil {
		t.Fatalf("WriteField set_current: %v", err)
	}
	part, err := writer.CreateFormFile("content", "source.txt")
	if err != nil {
		t.Fatalf("CreateFormFile content: %v", err)
	}
	if _, err := io.WriteString(part, "# Book\n\n## One\nBody 🍥"); err != nil {
		t.Fatalf("write source content: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close multipart writer: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, sourcesURL, &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec = env.do(req)
	assertStatus(t, rec, http.StatusOK)
	derived := decodeJSON[map[string]any](t, rec)
	derivedID, _ := derived["id"].(string)
	if derived["format"] != "md" || derived["schema_version"] != float64(1) {
		t.Fatalf("derived source metadata = %#v, want schema 1 Markdown", derived)
	}
	if derived["comment"] != "derived in contract test" {
		t.Fatalf("derived source comment = %#v", derived["comment"])
	}

	rec = env.do(httptest.NewRequest(http.MethodGet, "/api/shelves/default_shelf/books/"+created.Meta.ID, nil))
	assertStatus(t, rec, http.StatusOK)
	activated := decodeJSON[server.Book](t, rec)
	if activated.Meta == nil || activated.Meta.CurrentSource != derivedID || activated.Meta.Format != "md" {
		t.Fatalf("activated book = %#v, want current derived Markdown source", activated.Meta)
	}
}

func TestAPIDeleteBookSourceContract(t *testing.T) {
	env := newAPITestEnv(t)
	created := importTextBook(t, env, "Delete Source Book", "", "del.txt", "content")
	sourcesURL := "/api/shelves/default_shelf/books/" + created.Meta.ID + "/sources"

	// Create a new source to delete.
	rec := env.do(httptest.NewRequest(http.MethodPost, sourcesURL, nil))
	assertStatus(t, rec, http.StatusOK)
	newSource := decodeJSON[map[string]any](t, rec)
	newSourceID, _ := newSource["id"].(string)
	if newSourceID == "" {
		t.Fatalf("expected non-empty source id in response, got %#v", newSource)
	}

	// Deleting the source should succeed.
	rec = env.do(httptest.NewRequest(http.MethodDelete, sourcesURL+"/"+newSourceID, nil))
	assertStatus(t, rec, http.StatusNoContent)

	// The deleted source should no longer appear in the list.
	rec = env.do(httptest.NewRequest(http.MethodGet, sourcesURL, nil))
	assertStatus(t, rec, http.StatusOK)
	sources := decodeJSON[[]map[string]any](t, rec)
	for _, s := range sources {
		if id, _ := s["id"].(string); id == newSourceID {
			t.Fatalf("deleted source %q still present in list", newSourceID)
		}
	}

	// Deleting a nonexistent source should return 404.
	rec = env.do(httptest.NewRequest(http.MethodDelete, sourcesURL+"/nonexistent-source", nil))
	assertStatus(t, rec, http.StatusNotFound)

	// Deleting a source from a nonexistent book should return 404.
	rec = env.do(httptest.NewRequest(http.MethodDelete, "/api/shelves/default_shelf/books/no-such-book/sources/"+newSourceID, nil))
	assertStatus(t, rec, http.StatusNotFound)
}

func TestAPISetCurrentBookSourceContract(t *testing.T) {
	env := newAPITestEnv(t)
	created := importTextBook(t, env, "Set Current Source Book", "", "src.txt", "content")
	sourcesURL := "/api/shelves/default_shelf/books/" + created.Meta.ID + "/sources"

	// Create a second source.
	rec := env.do(httptest.NewRequest(http.MethodPost, sourcesURL, nil))
	assertStatus(t, rec, http.StatusOK)
	newSource := decodeJSON[map[string]any](t, rec)
	newSourceID, _ := newSource["id"].(string)
	if newSourceID == "" {
		t.Fatalf("expected non-empty source id in response, got %#v", newSource)
	}

	// Setting the current source should succeed.
	rec = env.do(httptest.NewRequest(http.MethodPut, sourcesURL+"/"+newSourceID+"/current", nil))
	assertStatus(t, rec, http.StatusNoContent)

	// The book should reflect the new current source.
	rec = env.do(httptest.NewRequest(http.MethodGet, "/api/shelves/default_shelf/books/"+created.Meta.ID, nil))
	assertStatus(t, rec, http.StatusOK)
	bookData := decodeJSON[map[string]any](t, rec)
	meta, _ := bookData["meta"].(map[string]any)
	if currentSource, _ := meta["current_source"].(string); currentSource != newSourceID {
		t.Fatalf("expected current_source %q, got %q", newSourceID, currentSource)
	}

	// Setting current source for a nonexistent source should return 404.
	rec = env.do(httptest.NewRequest(http.MethodPut, sourcesURL+"/nonexistent-source/current", nil))
	assertStatus(t, rec, http.StatusNotFound)

	// Setting current source for a nonexistent book should return 404.
	rec = env.do(httptest.NewRequest(http.MethodPut, "/api/shelves/default_shelf/books/no-such-book/sources/"+newSourceID+"/current", nil))
	assertStatus(t, rec, http.StatusNotFound)
}

func TestAPIRefreshBookSourceMetaContract(t *testing.T) {
	env := newAPITestEnv(t)
	created := importTextBook(t, env, "Refresh Source", "", "refresh.txt", "line one\nline two\nline three")
	sourceID := created.Meta.CurrentSource
	refreshURL := "/api/shelves/default_shelf/books/" + created.Meta.ID + "/sources/" + sourceID + "/refresh"

	rec := env.do(httptest.NewRequest(http.MethodPost, refreshURL, nil))
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

	// Refreshing a nonexistent source returns 404.
	rec = env.do(httptest.NewRequest(http.MethodPost, "/api/shelves/default_shelf/books/"+created.Meta.ID+"/sources/nonexistent/refresh", nil))
	assertStatus(t, rec, http.StatusNotFound)

	// Refreshing a nonexistent book returns 404.
	rec = env.do(httptest.NewRequest(http.MethodPost, "/api/shelves/default_shelf/books/no-such-book/sources/"+sourceID+"/refresh", nil))
	assertStatus(t, rec, http.StatusNotFound)
}
