package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/voilelab/plainshelf/server/store"
)

// Reading and writing a bookmark are the same route pair, so they agree on
// whether the shelf has to exist.
func TestMarksRoutesAgreeOnUnknownShelf(t *testing.T) {
	env := newAPITestEnv(t)
	book := importTextBook(t, env, "Marked", "", "marked.txt", "body")

	requests := map[string]*http.Request{
		"read": httptest.NewRequest(http.MethodGet,
			"/api/shelves/no_such_shelf/marks/"+book.Meta.ID, nil),
		"write": httptest.NewRequest(http.MethodPost,
			"/api/shelves/no_such_shelf/marks/"+book.Meta.ID, strings.NewReader(`{"char_offset":10}`)),
	}

	for name, req := range requests {
		t.Run(name, func(t *testing.T) {
			rec := env.do(req)

			assertStatus(t, rec, http.StatusNotFound)
			if got := strings.TrimSpace(rec.Body.String()); got != "shelf not found" {
				t.Fatalf("body = %q, want %q", got, "shelf not found")
			}
		})
	}
}

// The zero bookmark is how a reader opens a book for the first time, so an
// unread book must keep answering 200 rather than 404.
func TestMarksReadReturnsZeroBookmarkForAnUnreadBook(t *testing.T) {
	env := newAPITestEnv(t)
	book := importTextBook(t, env, "Unread", "", "unread.txt", "body")

	rec := env.do(httptest.NewRequest(http.MethodGet,
		"/api/shelves/default_shelf/marks/"+book.Meta.ID, nil))

	assertStatus(t, rec, http.StatusOK)
	if mark := decodeJSON[store.Bookmark](t, rec); mark.CharOffset != 0 {
		t.Fatalf("char_offset = %d, want 0 for an unread book", mark.CharOffset)
	}
}

func TestMarksRoundTrip(t *testing.T) {
	env := newAPITestEnv(t)
	book := importTextBook(t, env, "Round Trip", "", "roundtrip.txt", "body")
	url := "/api/shelves/default_shelf/marks/" + book.Meta.ID

	assertStatus(t, env.do(httptest.NewRequest(http.MethodPost, url,
		strings.NewReader(`{"char_offset":42}`))), http.StatusNoContent)

	rec := env.do(httptest.NewRequest(http.MethodGet, url, nil))
	assertStatus(t, rec, http.StatusOK)
	if mark := decodeJSON[store.Bookmark](t, rec); mark.CharOffset != 42 {
		t.Fatalf("char_offset = %d, want 42", mark.CharOffset)
	}
}
