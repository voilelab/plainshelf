package books_test

import (
	"encoding/json/v2"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/voilelab/plainshelf/server/contract/apitest"

	"github.com/voilelab/plainshelf/server"
	"github.com/voilelab/plainshelf/shelf"
)

// patchBookOK patches a book, asserts it succeeded, and returns the updated book
// the response carries.
func patchBookOK(t *testing.T, env *apitest.Env, bookID, body string) server.Book {
	t.Helper()

	return apitest.DecodeJSON[server.Book](t, apitest.PatchBook(t, env, bookID, body, http.StatusOK))
}

func TestAPIGetBooksContract(t *testing.T) {
	env := apitest.New(t)

	rec := env.Get(apitest.BooksURL())
	apitest.AssertStatus(t, rec, http.StatusOK)
	apitest.AssertJSONContentType(t, rec)
	if got := apitest.DecodeJSON[[]server.Book](t, rec); len(got) != 0 {
		t.Fatalf("empty library returned %d books", len(got))
	}

	alpha := apitest.ImportTextBook(t, env, "Alpha Tale", "/fiction/adventure", "alpha.txt", "alpha body")
	_ = apitest.ImportTextBook(t, env, "Beta Notes", "/notes", "beta.txt", "beta body")

	patchBookOK(t, env, alpha.Meta.ID,
		`{"authors":["Ada"],"tags":["contract","api"],"language":"en","comment":"needle comment"}`)

	books := apitest.GetJSON[[]server.Book](t, env, apitest.BooksURL())
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
	if strings.Join(got.Folder, "/") != "fiction/adventure" {
		t.Fatalf("folder = %#v, want fiction/adventure", got.Folder)
	}
}

func TestAPIGetBooksCharCountContract(t *testing.T) {
	env := apitest.New(t)
	_ = apitest.ImportTextBook(t, env, "Char Count Me", "", "charcount.txt", "alpha body")

	// Without include=char_count, the field must not appear in the response at all.
	rec := env.Get(apitest.BooksURL())
	apitest.AssertStatus(t, rec, http.StatusOK)
	if strings.Contains(rec.Body.String(), "char_count") {
		t.Fatalf("response without include=char_count must not contain char_count field: %s", rec.Body.String())
	}

	// With include=char_count, every book carries a positive char_count.
	books := apitest.GetJSON[[]server.Book](t, env, apitest.BooksURL()+"?include=char_count")
	if len(books) != 1 {
		t.Fatalf("list returned %d books, want 1", len(books))
	}
	if books[0].CharCount <= 0 {
		t.Fatalf("char_count = %d, want > 0", books[0].CharCount)
	}
}

// char_count is answered from the book cache rather than by opening one source
// per request, so every route that changes what the current source holds has to
// leave the cached count correct. A stale entry can only be seen from the
// listing, which is what this reads back after each write.
func TestAPIGetBooksCharCountFollowsSourceWritesContract(t *testing.T) {
	env := apitest.New(t)
	created := apitest.ImportTextBook(t, env, "Rewritten", "", "rewrite.txt", "alpha body")
	bookID := created.Meta.ID

	const rewritten = "alpha body, extended"
	wantRewritten := utf8.RuneCountInString(rewritten)

	rec := env.PatchContent(apitest.SourceURL(bookID, created.Meta.CurrentSource, "content"),
		apitest.PlainTextContentType, strings.NewReader(rewritten))
	apitest.AssertStatus(t, rec, http.StatusNoContent)
	if got := charCountByBookID(t, env)[bookID]; got != wantRewritten {
		t.Fatalf("char_count after rewriting the current source = %d, want %d", got, wantRewritten)
	}

	// A second source changes the count only once it is the current one.
	empty := apitest.CreateBookSource(t, env, bookID)
	apitest.AssertStatus(t, env.Put(apitest.SourceURL(bookID, empty, "current"), nil), http.StatusNoContent)
	if got := charCountByBookID(t, env)[bookID]; got != 0 {
		t.Fatalf("char_count of an empty current source = %d, want 0", got)
	}

	// Deleting the current source hands the pointer back to the text.
	apitest.AssertStatus(t, env.Delete(apitest.SourceURL(bookID, empty)), http.StatusNoContent)
	if got := charCountByBookID(t, env)[bookID]; got != wantRewritten {
		t.Fatalf("char_count after deleting the current source = %d, want %d", got, wantRewritten)
	}

	// The refresh route recomputes a count that was never stored - the state
	// clearSourceCharCount reproduces - and the listing must report the result
	// straight away rather than at the next walk of the shelf.
	clearSourceCharCount(t, env, bookID)
	apitest.AssertStatus(t, env.Post(apitest.ScansURL(), nil), http.StatusOK)
	if got := charCountByBookID(t, env)[bookID]; got != 0 {
		t.Fatalf("char_count of a cleared source = %d, want 0", got)
	}

	apitest.AssertStatus(t, env.Post(apitest.SourceURL(bookID, apitest.CurrentSourceOf(t, env, bookID), "refresh"), nil), http.StatusOK)
	if got := charCountByBookID(t, env)[bookID]; got != wantRewritten {
		t.Fatalf("char_count after the refresh route = %d, want %d", got, wantRewritten)
	}
}

// A description is Markdown, and for an EPUB import it is the HTML the OPF
// carried over. The detail page sanitizes it at the moment it renders it, which
// is the only place that may: the shelf keeps the source text, so book.json
// stays a file its owner can edit by hand and read back unchanged, and nothing
// on disk drifts from the EPUB it came from.
func TestAPIBookCommentIsStoredVerbatimContract(t *testing.T) {
	env := apitest.New(t)
	created := apitest.ImportTextBook(t, env, "Described", "", "described.txt", "body")

	const description = "<p>第一段</p>\n\n**粗體** 與 <script>alert(1)</script>\n\n- 項目"
	body, err := json.Marshal(map[string]string{"comment": description})
	if err != nil {
		t.Fatalf("marshal patch body: %v", err)
	}

	if updated := patchBookOK(t, env, created.Meta.ID, string(body)); updated.Meta.Comments != description {
		t.Fatalf("PATCH answered comment %q, want %q", updated.Meta.Comments, description)
	}
	if fetched := apitest.GetJSON[server.Book](t, env, apitest.BookURL(created.Meta.ID)); fetched.Meta.Comments != description {
		t.Fatalf("GET answered comment %q, want %q", fetched.Meta.Comments, description)
	}

	raw, err := os.ReadFile(env.BookMetaPath(t, created.Meta.ID))
	if err != nil {
		t.Fatalf("read book.json: %v", err)
	}
	var meta shelf.BookMeta
	if err := json.Unmarshal(raw, &meta); err != nil {
		t.Fatalf("unmarshal book.json: %v", err)
	}
	if meta.Comments != description {
		t.Fatalf("book.json holds comment %q, want %q", meta.Comments, description)
	}
}

func TestAPIUpdateBookContract(t *testing.T) {
	env := apitest.New(t)
	created := apitest.ImportTextBook(t, env, "Patch Me", "old/folder", "patch.txt", "body")

	rec := apitest.PatchBook(t, env, created.Meta.ID,
		`{"title":"Patched","authors":["Author A","Author B"],"tags":["tag1"],"language":"zh-Hant","comment":"updated comment","star":5,"folder":["new","folder"]}`,
		http.StatusOK)
	apitest.AssertJSONContentType(t, rec)
	updated := apitest.DecodeJSON[server.Book](t, rec)
	if updated.Meta.Title != "Patched" || updated.Meta.Comments != "updated comment" || updated.Meta.Language != "zh-Hant" || updated.Meta.Star != 5 {
		t.Fatalf("metadata was not updated: %#v", updated.Meta)
	}
	if len(updated.Meta.Authors) != 2 || updated.Meta.Authors[1] != "Author B" {
		t.Fatalf("authors = %#v", updated.Meta.Authors)
	}
	if strings.Join(updated.Folder, "/") != "new/folder" {
		t.Fatalf("folder = %#v, want new/folder", updated.Folder)
	}

	// An unknown field, an out-of-range star and a malformed language tag are all
	// client errors rather than server failures.
	for _, body := range []string{
		`{"unexpected":true}`,
		`{"star":6}`,
		`{"language":"!!!not-a-tag"}`,
	} {
		apitest.PatchBook(t, env, created.Meta.ID, body, http.StatusBadRequest)
	}
}

// The stored format decides whether the reader parses the text as Markdown, and
// import can only guess it from a file extension. This is the correction path:
// switching it is metadata-only, so the book's content is never rewritten.
func TestAPIUpdateBookFormatContract(t *testing.T) {
	env := apitest.New(t)
	created := apitest.ImportTextBook(t, env, "Format Book", "", "format.txt", "# Notes\n\nhello")
	bookID := created.Meta.ID

	if created.Meta.Format != "txt" {
		t.Fatalf("imported format = %q, want txt", created.Meta.Format)
	}

	rec := env.Get(apitest.BookURL(bookID, "content"))
	apitest.AssertStatus(t, rec, http.StatusOK)
	contentBefore := rec.Body.String()

	if updated := patchBookOK(t, env, bookID, `{"format":"md"}`); updated.Meta.Format != "md" {
		t.Fatalf("format = %q, want md in the PATCH response", updated.Meta.Format)
	}

	if fetched := apitest.GetJSON[server.Book](t, env, apitest.BookURL(bookID)); fetched.Meta.Format != "md" {
		t.Fatalf("format = %q, want md after GET", fetched.Meta.Format)
	}

	// Switching the format must not touch the text it describes.
	rec = env.Get(apitest.BookURL(bookID, "content"))
	apitest.AssertStatus(t, rec, http.StatusOK)
	if got := rec.Body.String(); got != contentBefore {
		t.Fatalf("content = %q, want it unchanged at %q", got, contentBefore)
	}

	// A format this build cannot render is a client error, and the stored value survives it.
	apitest.PatchBook(t, env, bookID, `{"format":"epub"}`, http.StatusBadRequest)

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
	env := apitest.New(t)
	created := apitest.ImportTextBook(t, env, "Identifiers Book", "identifiers/folder", "identifiers.txt", "body")
	bookID := created.Meta.ID

	// Setting identifiers is reflected in the PATCH response and a subsequent GET.
	updated := patchBookOK(t, env, bookID, `{"identifiers":{"isbn":"978-0-13-468599-1","douban":"123"}}`)
	if updated.Meta.Identifiers["isbn"] != "978-0-13-468599-1" || updated.Meta.Identifiers["douban"] != "123" {
		t.Fatalf("identifiers not set in PATCH response: %#v", updated.Meta.Identifiers)
	}

	fetched := apitest.GetJSON[server.Book](t, env, apitest.BookURL(bookID))
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
	apitest.PatchBook(t, env, bookID, `{"identifiers":{"":"x"}}`, http.StatusBadRequest)
}

// removeSourceFolder deletes a source's folder behind the API's back, which is
// how a shelf edited by hand or by a sync tool ends up with a current_source
// pointing at nothing. Removing the folder rather than rewriting book.json keeps
// the book metadata's own staleness checks out of the picture.
func removeSourceFolder(t *testing.T, env *apitest.Env, bookID, sourceID string) {
	t.Helper()

	bookDir := filepath.Dir(env.BookMetaPath(t, bookID))
	if err := os.RemoveAll(filepath.Join(bookDir, shelf.SourcesFolder, sourceID)); err != nil {
		t.Fatalf("remove source folder: %v", err)
	}
}

// TestAPIDanglingCurrentSourceFallsBackOnRead pins the read-path tolerance: a
// book whose current_source names a source that no longer exists still serves
// its text, from the newest source it does have. The reads must not repair
// book.json, because the filesystem stays the source of truth.
func TestAPIDanglingCurrentSourceFallsBackOnRead(t *testing.T) {
	env := apitest.New(t)
	created := apitest.ImportTextBook(t, env, "Dangling Current Source", "", "dangle.txt", "surviving body")
	survivingID := created.Meta.CurrentSource

	danglingID := apitest.CreateBookSource(t, env, created.Meta.ID)
	rec := env.Put(apitest.SourceURL(created.Meta.ID, danglingID, "current"), nil)
	apitest.AssertStatus(t, rec, http.StatusNoContent)

	removeSourceFolder(t, env, created.Meta.ID, danglingID)

	content := env.Get(apitest.BookURL(created.Meta.ID, "content"))
	apitest.AssertStatus(t, content, http.StatusOK)
	if body := content.Body.String(); !strings.Contains(body, "surviving body") {
		t.Fatalf("content = %q, want the surviving source's text", body)
	}

	if got := apitest.CurrentSourceOf(t, env, created.Meta.ID); got != danglingID {
		t.Fatalf("current_source = %q, want reads to leave it at %q", got, danglingID)
	}
	if ids := apitest.SourceIDs(t, env, created.Meta.ID); !slices.Equal(ids, []string{survivingID}) {
		t.Fatalf("sources = %#v, want only %q", ids, survivingID)
	}
}

// TestAPIBookWithoutAnySourceReturns404 pins the other end of that tolerance: a
// book with nothing to read is a missing source, not a server fault, so the
// content routes answer 404 rather than 500.
func TestAPIBookWithoutAnySourceReturns404(t *testing.T) {
	env := apitest.New(t)
	created := apitest.ImportTextBook(t, env, "Sourceless Book", "", "gone.txt", "body")

	removeSourceFolder(t, env, created.Meta.ID, created.Meta.CurrentSource)

	apitest.AssertStatus(t, env.Get(apitest.BookURL(created.Meta.ID, "content")), http.StatusNotFound)

	// The book itself is still perfectly readable, which is what keeps its
	// detail page working while the shelf is in this state.
	apitest.AssertStatus(t, env.Get(apitest.BookURL(created.Meta.ID)), http.StatusOK)
}
