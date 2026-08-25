package contract_test

import (
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/voilelab/plainshelf/server"
)

// DELETE /books/{id} and POST /books/{id}/trash share one handler; these pin
// that they stay interchangeable.

// The full trash lifecycle is covered by TestAPITrashLifecycleContract; what is
// unique here is that the two routes are interchangeable. One env serves both:
// each route trashes its own book, so they never observe each other's effect.
func TestAPIDeleteAndTrashRoutesBothTrashTheBookContract(t *testing.T) {
	env := newAPITestEnv(t)

	routes := map[string]func(bookID string) *http.Request{
		"DELETE /books/{book_id}": func(bookID string) *http.Request {
			return httptest.NewRequest(http.MethodDelete, bookURL(bookID), nil)
		},
		"POST /books/{book_id}/trash": func(bookID string) *http.Request {
			return httptest.NewRequest(http.MethodPost, bookURL(bookID, "trash"), nil)
		},
	}

	for name, build := range routes {
		t.Run(name, func(t *testing.T) {
			book := importTextBook(t, env, "Trash Me", "", "trash-me.txt", "body")
			bookID := book.Meta.ID

			assertStatus(t, env.do(build(bookID)), http.StatusNoContent)

			assertStatus(t, env.get(bookURL(bookID)), http.StatusNotFound)

			// Recoverable, which is what makes both routes a trash operation.
			trashed := getJSON[[]server.TrashedBook](t, env, shelfURL("trash", "books"))
			if !slices.ContainsFunc(trashed, func(b server.TrashedBook) bool { return b.ID == bookID }) {
				t.Fatalf("trash = %+v, want it to contain the trashed book %s", trashed, bookID)
			}
		})
	}
}

func TestAPIDeleteAndTrashRoutesAgreeOnUnknownBookContract(t *testing.T) {
	env := newAPITestEnv(t)

	requests := map[string]*http.Request{
		"DELETE /books/{book_id}": httptest.NewRequest(http.MethodDelete,
			bookURL("no_such_book"), nil),
		"POST /books/{book_id}/trash": httptest.NewRequest(http.MethodPost,
			bookURL("no_such_book", "trash"), nil),
	}

	for name, req := range requests {
		t.Run(name, func(t *testing.T) {
			rec := env.do(req)

			assertStatus(t, rec, http.StatusNotFound)
			if got := strings.TrimSpace(rec.Body.String()); got != "book not found" {
				t.Fatalf("body = %q, want %q", got, "book not found")
			}
		})
	}
}
