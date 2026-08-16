package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/voilelab/plainshelf/shelf"
)

func TestAPIGetBooksContract(t *testing.T) {
	env := newAPITestEnv(t)

	rec := env.do(httptest.NewRequest(http.MethodGet, "/api/shelves/default_shelf/books", nil))
	assertStatus(t, rec, http.StatusOK)
	assertJSONContentType(t, rec)
	if got := decodeJSON[[]Book](t, rec); len(got) != 0 {
		t.Fatalf("empty library returned %d books", len(got))
	}

	alpha := importTextBook(t, env, "Alpha Tale", "/fiction/adventure", "alpha.txt", "alpha body")
	_ = importTextBook(t, env, "Beta Notes", "/notes", "beta.txt", "beta body")

	patchBody := `{"authors":["Ada"],"tags":["contract","api"],"language":"en","comment":"needle comment"}`
	rec = env.do(httptest.NewRequest(http.MethodPatch, "/api/shelves/default_shelf/books/"+alpha.Meta.ID, strings.NewReader(patchBody)))
	assertStatus(t, rec, http.StatusOK)

	rec = env.do(httptest.NewRequest(http.MethodGet, "/api/shelves/default_shelf/books", nil))
	assertStatus(t, rec, http.StatusOK)
	books := decodeJSON[[]Book](t, rec)
	if len(books) != 2 {
		t.Fatalf("list returned %d books, want 2", len(books))
	}
	var got *Book
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
	rec := env.do(httptest.NewRequest(http.MethodGet, "/api/shelves/default_shelf/books", nil))
	assertStatus(t, rec, http.StatusOK)
	if strings.Contains(rec.Body.String(), "char_count") {
		t.Fatalf("response without include=char_count must not contain char_count field: %s", rec.Body.String())
	}

	// With include=char_count, every book carries a positive char_count.
	rec = env.do(httptest.NewRequest(http.MethodGet, "/api/shelves/default_shelf/books?include=char_count", nil))
	assertStatus(t, rec, http.StatusOK)
	books := decodeJSON[[]Book](t, rec)
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

	body := `{"title":"Patched","authors":["Author A","Author B"],"tags":["tag1"],"language":"zh-Hant","comment":"updated comment","star":5,"layer":["new","layer"]}`
	rec := env.do(httptest.NewRequest(http.MethodPatch, "/api/shelves/default_shelf/books/"+created.Meta.ID, strings.NewReader(body)))
	assertStatus(t, rec, http.StatusOK)
	assertJSONContentType(t, rec)
	updated := decodeJSON[Book](t, rec)
	if updated.Meta.Title != "Patched" || updated.Meta.Comments != "updated comment" || updated.Meta.Language != "zh-Hant" || updated.Meta.Star != 5 {
		t.Fatalf("metadata was not updated: %#v", updated.Meta)
	}
	if len(updated.Meta.Authors) != 2 || updated.Meta.Authors[1] != "Author B" {
		t.Fatalf("authors = %#v", updated.Meta.Authors)
	}
	if strings.Join(updated.Layer, "/") != "new/layer" {
		t.Fatalf("layer = %#v, want new/layer", updated.Layer)
	}

	rec = env.do(httptest.NewRequest(http.MethodPatch, "/api/shelves/default_shelf/books/"+created.Meta.ID, strings.NewReader(`{"unexpected":true}`)))
	assertStatus(t, rec, http.StatusBadRequest)

	rec = env.do(httptest.NewRequest(http.MethodPatch, "/api/shelves/default_shelf/books/"+created.Meta.ID, strings.NewReader(`{"star":6}`)))
	assertStatus(t, rec, http.StatusBadRequest)

	// A malformed language tag is a client error, not a server failure.
	rec = env.do(httptest.NewRequest(http.MethodPatch, "/api/shelves/default_shelf/books/"+created.Meta.ID, strings.NewReader(`{"language":"!!!not-a-tag"}`)))
	assertStatus(t, rec, http.StatusBadRequest)
}

// The stored format decides whether the reader parses the text as Markdown, and
// import can only guess it from a file extension. This is the correction path:
// switching it is metadata-only, so the book's content is never rewritten.
func TestAPIUpdateBookFormatContract(t *testing.T) {
	env := newAPITestEnv(t)
	created := importTextBook(t, env, "Format Book", "", "format.txt", "# Notes\n\nhello")
	bookURL := "/api/shelves/default_shelf/books/" + created.Meta.ID

	if created.Meta.Format != "txt" {
		t.Fatalf("imported format = %q, want txt", created.Meta.Format)
	}

	rec := env.do(httptest.NewRequest(http.MethodGet, bookURL+"/content", nil))
	assertStatus(t, rec, http.StatusOK)
	contentBefore := rec.Body.String()

	rec = env.do(httptest.NewRequest(http.MethodPatch, bookURL, strings.NewReader(`{"format":"md"}`)))
	assertStatus(t, rec, http.StatusOK)
	updated := decodeJSON[Book](t, rec)
	if updated.Meta.Format != "md" {
		t.Fatalf("format = %q, want md in the PATCH response", updated.Meta.Format)
	}

	rec = env.do(httptest.NewRequest(http.MethodGet, bookURL, nil))
	assertStatus(t, rec, http.StatusOK)
	if fetched := decodeJSON[Book](t, rec); fetched.Meta.Format != "md" {
		t.Fatalf("format = %q, want md after GET", fetched.Meta.Format)
	}

	// Switching the format must not touch the text it describes.
	rec = env.do(httptest.NewRequest(http.MethodGet, bookURL+"/content", nil))
	assertStatus(t, rec, http.StatusOK)
	if got := rec.Body.String(); got != contentBefore {
		t.Fatalf("content = %q, want it unchanged at %q", got, contentBefore)
	}

	// A format this build cannot render is a client error, and the stored value survives it.
	rec = env.do(httptest.NewRequest(http.MethodPatch, bookURL, strings.NewReader(`{"format":"epub"}`)))
	assertStatus(t, rec, http.StatusBadRequest)

	rec = env.do(httptest.NewRequest(http.MethodPatch, bookURL, strings.NewReader(`{"title":"Format Book Renamed"}`)))
	assertStatus(t, rec, http.StatusOK)
	untouched := decodeJSON[Book](t, rec)
	if untouched.Meta.Format != "md" {
		t.Fatalf("format = %q, want md left alone when the PATCH omits it", untouched.Meta.Format)
	}

	// The switch is reversible: nothing about going to md is one-way.
	rec = env.do(httptest.NewRequest(http.MethodPatch, bookURL, strings.NewReader(`{"format":"txt"}`)))
	assertStatus(t, rec, http.StatusOK)
	if reverted := decodeJSON[Book](t, rec); reverted.Meta.Format != "txt" {
		t.Fatalf("format = %q, want txt after switching back", reverted.Meta.Format)
	}
}

func TestAPIUpdateBookIdentifiersContract(t *testing.T) {
	env := newAPITestEnv(t)
	created := importTextBook(t, env, "Identifiers Book", "identifiers/layer", "identifiers.txt", "body")
	bookURL := "/api/shelves/default_shelf/books/" + created.Meta.ID

	// Setting identifiers is reflected in the PATCH response and a subsequent GET.
	rec := env.do(httptest.NewRequest(http.MethodPatch, bookURL, strings.NewReader(`{"identifiers":{"isbn":"978-0-13-468599-1","douban":"123"}}`)))
	assertStatus(t, rec, http.StatusOK)
	updated := decodeJSON[Book](t, rec)
	if updated.Meta.Identifiers["isbn"] != "978-0-13-468599-1" || updated.Meta.Identifiers["douban"] != "123" {
		t.Fatalf("identifiers not set in PATCH response: %#v", updated.Meta.Identifiers)
	}

	rec = env.do(httptest.NewRequest(http.MethodGet, bookURL, nil))
	assertStatus(t, rec, http.StatusOK)
	fetched := decodeJSON[Book](t, rec)
	if fetched.Meta.Identifiers["isbn"] != "978-0-13-468599-1" || fetched.Meta.Identifiers["douban"] != "123" {
		t.Fatalf("identifiers not set after GET: %#v", fetched.Meta.Identifiers)
	}

	// A subsequent PATCH with a new identifiers map fully replaces the old one (not a merge).
	rec = env.do(httptest.NewRequest(http.MethodPatch, bookURL, strings.NewReader(`{"identifiers":{"isbn":"999"}}`)))
	assertStatus(t, rec, http.StatusOK)
	replaced := decodeJSON[Book](t, rec)
	if replaced.Meta.Identifiers["isbn"] != "999" {
		t.Fatalf("identifiers isbn not replaced: %#v", replaced.Meta.Identifiers)
	}
	if _, ok := replaced.Meta.Identifiers["douban"]; ok {
		t.Fatalf("expected douban identifier to be gone after full replace, got: %#v", replaced.Meta.Identifiers)
	}

	// A PATCH that omits the identifiers field entirely leaves the existing value untouched.
	rec = env.do(httptest.NewRequest(http.MethodPatch, bookURL, strings.NewReader(`{"title":"Identifiers Book Renamed"}`)))
	assertStatus(t, rec, http.StatusOK)
	untouched := decodeJSON[Book](t, rec)
	if untouched.Meta.Title != "Identifiers Book Renamed" {
		t.Fatalf("title not updated: %#v", untouched.Meta)
	}
	if untouched.Meta.Identifiers["isbn"] != "999" {
		t.Fatalf("identifiers should be unchanged when omitted from PATCH body: %#v", untouched.Meta.Identifiers)
	}

	// An explicit empty identifiers object clears the map.
	rec = env.do(httptest.NewRequest(http.MethodPatch, bookURL, strings.NewReader(`{"identifiers":{}}`)))
	assertStatus(t, rec, http.StatusOK)
	cleared := decodeJSON[Book](t, rec)
	if len(cleared.Meta.Identifiers) != 0 {
		t.Fatalf("expected identifiers to be cleared, got: %#v", cleared.Meta.Identifiers)
	}

	// An identifiers map with an empty key is rejected.
	rec = env.do(httptest.NewRequest(http.MethodPatch, bookURL, strings.NewReader(`{"identifiers":{"":"x"}}`)))
	assertStatus(t, rec, http.StatusBadRequest)
}

func TestAPISplitConfigContract(t *testing.T) {
	env := newAPITestEnv(t)
	created := importTextBook(t, env, "Split Me", "", "split.txt", "one\ntwo\nthree")
	url := "/api/shelves/default_shelf/books/" + created.Meta.ID + "/split_config"

	rec := env.do(httptest.NewRequest(http.MethodGet, url, nil))
	assertStatus(t, rec, http.StatusOK)
	assertJSONContentType(t, rec)
	initial := decodeJSON[shelf.SplitConfig](t, rec)
	if initial.Type != shelf.SplitTypeNone {
		t.Fatalf("initial split type = %q, want none", initial.Type)
	}

	payload := `{"type":"line_count","line_count":42}`
	rec = env.do(httptest.NewRequest(http.MethodPatch, url, strings.NewReader(payload)))
	assertStatus(t, rec, http.StatusNoContent)

	rec = env.do(httptest.NewRequest(http.MethodGet, url, nil))
	assertStatus(t, rec, http.StatusOK)
	roundTrip := decodeJSON[shelf.SplitConfig](t, rec)
	if roundTrip.Type != shelf.SplitTypeLineCount || roundTrip.LineCount != 42 {
		t.Fatalf("round-trip split config = %#v", roundTrip)
	}
}
