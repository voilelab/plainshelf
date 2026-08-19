package contract_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/voilelab/plainshelf/server"
	"github.com/voilelab/plainshelf/shelf"
)

// patchBook sends a PATCH to a book and asserts the status it must answer with.
// The body is JSON in every case, so the tests below read as the field-level
// contract they are.
func patchBook(t *testing.T, env *apiTestEnv, bookID, body string, wantStatus int) *httptest.ResponseRecorder {
	t.Helper()

	rec := env.patch(bookURL(bookID), strings.NewReader(body))
	assertStatus(t, rec, wantStatus)
	return rec
}

// patchBookOK patches a book, asserts it succeeded, and returns the updated book
// the response carries.
func patchBookOK(t *testing.T, env *apiTestEnv, bookID, body string) server.Book {
	t.Helper()

	return decodeJSON[server.Book](t, patchBook(t, env, bookID, body, http.StatusOK))
}

func TestAPIGetBooksContract(t *testing.T) {
	env := newAPITestEnv(t)

	rec := env.get(booksURL())
	assertStatus(t, rec, http.StatusOK)
	assertJSONContentType(t, rec)
	if got := decodeJSON[[]server.Book](t, rec); len(got) != 0 {
		t.Fatalf("empty library returned %d books", len(got))
	}

	alpha := importTextBook(t, env, "Alpha Tale", "/fiction/adventure", "alpha.txt", "alpha body")
	_ = importTextBook(t, env, "Beta Notes", "/notes", "beta.txt", "beta body")

	patchBookOK(t, env, alpha.Meta.ID,
		`{"authors":["Ada"],"tags":["contract","api"],"language":"en","comment":"needle comment"}`)

	books := getJSON[[]server.Book](t, env, booksURL())
	if len(books) != 2 {
		t.Fatalf("list returned %d books, want 2", len(books))
	}
	var got *server.Book
	for i := range books {
		if books[i].Meta != nil && books[i].Meta.ID == alpha.Meta.ID {
			got = &books[i]
			break
		}
	}
	if got == nil || got.Meta.Title != "Alpha Tale" {
		t.Fatalf("unexpected book meta: %#v", got)
	}
	if got.Meta.Comments != "needle comment" || got.Meta.Language != "en" {
		t.Fatalf("metadata fields not preserved in list response: %#v", got.Meta)
	}
	if len(got.Meta.Authors) != 1 || got.Meta.Authors[0] != "Ada" {
		t.Fatalf("authors = %#v, want Ada", got.Meta.Authors)
	}
	if len(got.Meta.Tags) != 2 || got.Meta.Tags[0] != "contract" || got.Meta.Tags[1] != "api" {
		t.Fatalf("tags = %#v, want contract/api", got.Meta.Tags)
	}
	if strings.Join(got.Layer, "/") != "fiction/adventure" {
		t.Fatalf("layer = %#v, want fiction/adventure", got.Layer)
	}
}

func TestAPIGetBooksCharCountContract(t *testing.T) {
	env := newAPITestEnv(t)
	_ = importTextBook(t, env, "Char Count Me", "", "charcount.txt", "alpha body")

	// Without include=char_count, the field must not appear in the response at all.
	rec := env.get(booksURL())
	assertStatus(t, rec, http.StatusOK)
	if strings.Contains(rec.Body.String(), "char_count") {
		t.Fatalf("response without include=char_count must not contain char_count field: %s", rec.Body.String())
	}

	// With include=char_count, every book carries a positive char_count.
	books := getJSON[[]server.Book](t, env, booksURL()+"?include=char_count")
	if len(books) != 1 {
		t.Fatalf("list returned %d books, want 1", len(books))
	}
	if books[0].CharCount <= 0 {
		t.Fatalf("char_count = %d, want > 0", books[0].CharCount)
	}
}

func TestAPIUpdateBookContract(t *testing.T) {
	env := newAPITestEnv(t)
	created := importTextBook(t, env, "Patch Me", "old/layer", "patch.txt", "body")

	rec := patchBook(t, env, created.Meta.ID,
		`{"title":"Patched","authors":["Author A","Author B"],"tags":["tag1"],"language":"zh-Hant","comment":"updated comment","star":5,"layer":["new","layer"]}`,
		http.StatusOK)
	assertJSONContentType(t, rec)
	updated := decodeJSON[server.Book](t, rec)
	if updated.Meta.Title != "Patched" || updated.Meta.Comments != "updated comment" || updated.Meta.Language != "zh-Hant" || updated.Meta.Star != 5 {
		t.Fatalf("metadata was not updated: %#v", updated.Meta)
	}
	if len(updated.Meta.Authors) != 2 || updated.Meta.Authors[1] != "Author B" {
		t.Fatalf("authors = %#v", updated.Meta.Authors)
	}
	if strings.Join(updated.Layer, "/") != "new/layer" {
		t.Fatalf("layer = %#v, want new/layer", updated.Layer)
	}

	// An unknown field, an out-of-range star and a malformed language tag are all
	// client errors rather than server failures.
	for _, body := range []string{
		`{"unexpected":true}`,
		`{"star":6}`,
		`{"language":"!!!not-a-tag"}`,
	} {
		patchBook(t, env, created.Meta.ID, body, http.StatusBadRequest)
	}
}

// The stored format decides whether the reader parses the text as Markdown, and
// import can only guess it from a file extension. This is the correction path:
// switching it is metadata-only, so the book's content is never rewritten.
func TestAPIUpdateBookFormatContract(t *testing.T) {
	env := newAPITestEnv(t)
	created := importTextBook(t, env, "Format Book", "", "format.txt", "# Notes\n\nhello")
	bookID := created.Meta.ID

	if created.Meta.Format != "txt" {
		t.Fatalf("imported format = %q, want txt", created.Meta.Format)
	}

	rec := env.get(bookURL(bookID, "content"))
	assertStatus(t, rec, http.StatusOK)
	contentBefore := rec.Body.String()

	if updated := patchBookOK(t, env, bookID, `{"format":"md"}`); updated.Meta.Format != "md" {
		t.Fatalf("format = %q, want md in the PATCH response", updated.Meta.Format)
	}

	if fetched := getJSON[server.Book](t, env, bookURL(bookID)); fetched.Meta.Format != "md" {
		t.Fatalf("format = %q, want md after GET", fetched.Meta.Format)
	}

	// Switching the format must not touch the text it describes.
	rec = env.get(bookURL(bookID, "content"))
	assertStatus(t, rec, http.StatusOK)
	if got := rec.Body.String(); got != contentBefore {
		t.Fatalf("content = %q, want it unchanged at %q", got, contentBefore)
	}

	// A format this build cannot render is a client error, and the stored value survives it.
	patchBook(t, env, bookID, `{"format":"epub"}`, http.StatusBadRequest)

	untouched := patchBookOK(t, env, bookID, `{"title":"Format Book Renamed"}`)
	if untouched.Meta.Format != "md" {
		t.Fatalf("format = %q, want md left alone when the PATCH omits it", untouched.Meta.Format)
	}

	// The switch is reversible: nothing about going to md is one-way.
	if reverted := patchBookOK(t, env, bookID, `{"format":"txt"}`); reverted.Meta.Format != "txt" {
		t.Fatalf("format = %q, want txt after switching back", reverted.Meta.Format)
	}
}

func TestAPIUpdateBookIdentifiersContract(t *testing.T) {
	env := newAPITestEnv(t)
	created := importTextBook(t, env, "Identifiers Book", "identifiers/layer", "identifiers.txt", "body")
	bookID := created.Meta.ID

	// Setting identifiers is reflected in the PATCH response and a subsequent GET.
	updated := patchBookOK(t, env, bookID, `{"identifiers":{"isbn":"978-0-13-468599-1","douban":"123"}}`)
	if updated.Meta.Identifiers["isbn"] != "978-0-13-468599-1" || updated.Meta.Identifiers["douban"] != "123" {
		t.Fatalf("identifiers not set in PATCH response: %#v", updated.Meta.Identifiers)
	}

	fetched := getJSON[server.Book](t, env, bookURL(bookID))
	if fetched.Meta.Identifiers["isbn"] != "978-0-13-468599-1" || fetched.Meta.Identifiers["douban"] != "123" {
		t.Fatalf("identifiers not set after GET: %#v", fetched.Meta.Identifiers)
	}

	// A subsequent PATCH with a new identifiers map fully replaces the old one (not a merge).
	replaced := patchBookOK(t, env, bookID, `{"identifiers":{"isbn":"999"}}`)
	if replaced.Meta.Identifiers["isbn"] != "999" {
		t.Fatalf("identifiers isbn not replaced: %#v", replaced.Meta.Identifiers)
	}
	if _, ok := replaced.Meta.Identifiers["douban"]; ok {
		t.Fatalf("expected douban identifier to be gone after full replace, got: %#v", replaced.Meta.Identifiers)
	}

	// A PATCH that omits the identifiers field entirely leaves the existing value untouched.
	untouched := patchBookOK(t, env, bookID, `{"title":"Identifiers Book Renamed"}`)
	if untouched.Meta.Title != "Identifiers Book Renamed" {
		t.Fatalf("title not updated: %#v", untouched.Meta)
	}
	if untouched.Meta.Identifiers["isbn"] != "999" {
		t.Fatalf("identifiers should be unchanged when omitted from PATCH body: %#v", untouched.Meta.Identifiers)
	}

	// An explicit empty identifiers object clears the map.
	cleared := patchBookOK(t, env, bookID, `{"identifiers":{}}`)
	if len(cleared.Meta.Identifiers) != 0 {
		t.Fatalf("expected identifiers to be cleared, got: %#v", cleared.Meta.Identifiers)
	}

	// An identifiers map with an empty key is rejected.
	patchBook(t, env, bookID, `{"identifiers":{"":"x"}}`, http.StatusBadRequest)
}

// removeSourceFolder deletes a source's folder behind the API's back, which is
// how a shelf edited by hand or by a sync tool ends up with a current_source
// pointing at nothing. Removing the folder rather than rewriting book.json keeps
// the book metadata's own staleness checks out of the picture.
func removeSourceFolder(t *testing.T, env *apiTestEnv, bookID, sourceID string) {
	t.Helper()

	bookDir := filepath.Dir(env.bookMetaPath(t, bookID))
	if err := os.RemoveAll(filepath.Join(bookDir, shelf.SourcesFolder, sourceID)); err != nil {
		t.Fatalf("remove source folder: %v", err)
	}
}

// TestAPIDanglingCurrentSourceFallsBackOnRead pins the read-path tolerance: a
// book whose current_source names a source that no longer exists still serves
// its text, from the newest source it does have. The reads must not repair
// book.json, because the filesystem stays the source of truth.
func TestAPIDanglingCurrentSourceFallsBackOnRead(t *testing.T) {
	env := newAPITestEnv(t)
	created := importTextBook(t, env, "Dangling Current Source", "", "dangle.txt", "surviving body")
	survivingID := created.Meta.CurrentSource

	danglingID := createBookSource(t, env, created.Meta.ID)
	rec := env.put(sourceURL(created.Meta.ID, danglingID, "current"), nil)
	assertStatus(t, rec, http.StatusNoContent)

	removeSourceFolder(t, env, created.Meta.ID, danglingID)

	content := env.get(bookURL(created.Meta.ID, "content"))
	assertStatus(t, content, http.StatusOK)
	if body := content.Body.String(); !strings.Contains(body, "surviving body") {
		t.Fatalf("content = %q, want the surviving source's text", body)
	}

	if got := currentSourceOf(t, env, created.Meta.ID); got != danglingID {
		t.Fatalf("current_source = %q, want reads to leave it at %q", got, danglingID)
	}
	if ids := sourceIDs(t, env, created.Meta.ID); !slices.Equal(ids, []string{survivingID}) {
		t.Fatalf("sources = %#v, want only %q", ids, survivingID)
	}
}

// TestAPIBookWithoutAnySourceReturns404 pins the other end of that tolerance: a
// book with nothing to read is a missing source, not a server fault, so the
// content routes answer 404 rather than 500.
func TestAPIBookWithoutAnySourceReturns404(t *testing.T) {
	env := newAPITestEnv(t)
	created := importTextBook(t, env, "Sourceless Book", "", "gone.txt", "body")

	removeSourceFolder(t, env, created.Meta.ID, created.Meta.CurrentSource)

	assertStatus(t, env.get(bookURL(created.Meta.ID, "content")), http.StatusNotFound)

	// The book itself is still perfectly readable, which is what keeps its
	// detail page working while the shelf is in this state.
	assertStatus(t, env.get(bookURL(created.Meta.ID)), http.StatusOK)
}
