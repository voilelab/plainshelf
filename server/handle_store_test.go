package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
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
