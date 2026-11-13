package books_test

import (
	"bytes"
	"net/http"
	"os"
	"testing"

	"github.com/voilelab/plainshelf/server"
	"github.com/voilelab/plainshelf/server/contract/apitest"
	"github.com/voilelab/plainshelf/shelf"
)

func assertSeries(t *testing.T, label string, got *shelf.BookSeries, want *shelf.BookSeries) {
	t.Helper()

	switch {
	case want == nil && got != nil:
		t.Errorf("%s series = %+v, want none", label, *got)
	case want != nil && (got == nil || *got != *want):
		t.Errorf("%s series = %+v, want %+v", label, got, *want)
	}
}

func listedSeries(t *testing.T, env *apitest.Env, bookID string) *shelf.BookSeries {
	t.Helper()

	for _, book := range apitest.GetJSON[[]server.Book](t, env, apitest.BooksURL()) {
		if book.Meta.ID == bookID {
			return book.Meta.Series
		}
	}
	t.Fatalf("book %s missing from the listing", bookID)
	return nil
}

func TestAPIUpdateBookSeriesContract(t *testing.T) {
	env := apitest.New(t)
	bookID := apitest.ImportTextBook(t, env, "Saga 2", "", "saga.txt", "body").Meta.ID

	if got := apitest.GetJSON[server.Book](t, env, apitest.BookURL(bookID)); got.Meta.Series != nil {
		t.Fatalf("imported book series = %+v, want none", *got.Meta.Series)
	}

	want := &shelf.BookSeries{Name: "Saga", Index: 1.5}
	updated := patchBookOK(t, env, bookID, `{"series":{"name":"  Saga ","index":1.5}}`)
	assertSeries(t, "PATCH response", updated.Meta.Series, want)
	assertSeries(t, "GET book", apitest.GetJSON[server.Book](t, env, apitest.BookURL(bookID)).Meta.Series, want)
	assertSeries(t, "GET books", listedSeries(t, env, bookID), want)

	untouched := patchBookOK(t, env, bookID, `{"title":"Saga 1.5"}`)
	assertSeries(t, "PATCH without series", untouched.Meta.Series, want)

	cleared := patchBookOK(t, env, bookID, `{"series":null}`)
	assertSeries(t, "PATCH series null", cleared.Meta.Series, nil)
	written, err := os.ReadFile(env.BookMetaPath(t, bookID))
	if err != nil {
		t.Fatalf("read book.json: %v", err)
	}
	if bytes.Contains(written, []byte(`"series"`)) {
		t.Errorf("book.json still carries series after clearing:\n%s", written)
	}
}

func TestAPIUpdateBookSeriesRejectsInvalid(t *testing.T) {
	env := apitest.New(t)
	bookID := apitest.ImportTextBook(t, env, "Saga", "", "saga.txt", "body").Meta.ID
	patchBookOK(t, env, bookID, `{"series":{"name":"Saga","index":2}}`)

	metaPath := env.BookMetaPath(t, bookID)
	before, err := os.ReadFile(metaPath)
	if err != nil {
		t.Fatalf("read book.json: %v", err)
	}

	for _, body := range []string{
		`{"series":{"name":" ","index":1}}`,
		`{"series":{"name":"Saga","index":-1}}`,
		`{"series":{"name":"Saga"}}`,
		`{"series":{"index":1}}`,
		`{"series":{"name":"Saga","index":"1"}}`,
		`{"series":{"name":"Saga","index":1,"volume":1}}`,
		`{"series":"Saga"}`,
	} {
		apitest.PatchBook(t, env, bookID, body, http.StatusBadRequest)
	}

	after, err := os.ReadFile(metaPath)
	if err != nil {
		t.Fatalf("re-read book.json: %v", err)
	}
	if !bytes.Equal(before, after) {
		t.Errorf("rejected series PATCH rewrote book.json:\n%s", after)
	}
}
