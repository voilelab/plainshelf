package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// DELETE /books/{id} and POST /books/{id}/trash are served by one handler
// because deleting a book is trashing it. These pin that the two routes stay
// interchangeable, so pointing one of them somewhere else cannot pass silently.

func TestDeleteAndTrashRoutesBothTrashTheBook(t *testing.T) {
	routes := map[string]func(bookID string) *http.Request{
		"DELETE /books/{book_id}": func(bookID string) *http.Request {
			return httptest.NewRequest(http.MethodDelete,
				"/api/shelves/default_shelf/books/"+bookID, nil)
		},
		"POST /books/{book_id}/trash": func(bookID string) *http.Request {
			return httptest.NewRequest(http.MethodPost,
				"/api/shelves/default_shelf/books/"+bookID+"/trash", nil)
		},
	}

	for name, build := range routes {
		t.Run(name, func(t *testing.T) {
			env := newAPITestEnv(t)
			book := importTextBook(t, env, "Trash Me", "", "trash-me.txt", "body")
			bookID := book.Meta.ID

			assertStatus(t, env.do(build(bookID)), http.StatusNoContent)

			// Gone from the shelf...
			assertStatus(t,
				env.do(httptest.NewRequest(http.MethodGet,
					"/api/shelves/default_shelf/books/"+bookID, nil)),
				http.StatusNotFound)

			// ...and recoverable from the trash, which is what makes both
			// routes a trash operation rather than a delete.
			trashRec := env.do(httptest.NewRequest(http.MethodGet,
				"/api/shelves/default_shelf/trash/books", nil))
			assertStatus(t, trashRec, http.StatusOK)

			trashed := decodeJSON[[]TrashedBook](t, trashRec)
			if len(trashed) != 1 || trashed[0].ID != bookID {
				t.Fatalf("trash = %+v, want exactly the trashed book %s", trashed, bookID)
			}
		})
	}
}

func TestDeleteAndTrashRoutesAgreeOnUnknownBook(t *testing.T) {
	env := newAPITestEnv(t)

	requests := map[string]*http.Request{
		"DELETE /books/{book_id}": httptest.NewRequest(http.MethodDelete,
			"/api/shelves/default_shelf/books/no_such_book", nil),
		"POST /books/{book_id}/trash": httptest.NewRequest(http.MethodPost,
			"/api/shelves/default_shelf/books/no_such_book/trash", nil),
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
