package folders_test

import (
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/voilelab/plainshelf/server/contract/apitest"

	"github.com/voilelab/plainshelf/server"
	"github.com/voilelab/plainshelf/server/task"
)

func TestAPITrashLifecycleContract(t *testing.T) {
	env := apitest.New(t)
	created := apitest.ImportTextBook(t, env, "Trash API", "origin/folder", "trash.txt", "body")

	rec := env.Post(apitest.BookURL(created.Meta.ID, "trash"), nil)
	apitest.AssertStatus(t, rec, http.StatusNoContent)

	if books := apitest.GetJSON[[]server.Book](t, env, apitest.BooksURL()); len(books) != 0 {
		t.Fatalf("active books after trash = %d, want 0", len(books))
	}

	trashed := apitest.GetJSON[[]map[string]any](t, env, apitest.TrashBooksURL())
	if len(trashed) != 1 {
		t.Fatalf("trashed books = %d, want 1", len(trashed))
	}
	if id, _ := trashed[0]["id"].(string); id != created.Meta.ID {
		t.Fatalf("trashed id = %q, want %q", id, created.Meta.ID)
	}

	rec = env.Post(apitest.TrashBooksURL(created.Meta.ID, "restore"), nil)
	apitest.AssertStatus(t, rec, http.StatusNoContent)

	if books := apitest.GetJSON[[]server.Book](t, env, apitest.BooksURL()); len(books) != 1 {
		t.Fatalf("active books after restore = %d, want 1", len(books))
	}

	rec = env.Delete(apitest.BookURL(created.Meta.ID))
	apitest.AssertStatus(t, rec, http.StatusNoContent)
	rec = env.Delete(apitest.TrashBooksURL(created.Meta.ID))
	apitest.AssertStatus(t, rec, http.StatusNoContent)
	rec = env.Post(apitest.TrashBooksURL(created.Meta.ID, "restore"), nil)
	apitest.AssertStatus(t, rec, http.StatusNotFound)
}

func TestAPIEmptyTrashContract(t *testing.T) {
	env := apitest.New(t)

	for _, title := range []string{"First", "Second"} {
		created := apitest.ImportTextBook(t, env, title, "", title+".txt", "body")
		rec := env.Post(apitest.BookURL(created.Meta.ID, "trash"), nil)
		apitest.AssertStatus(t, rec, http.StatusNoContent)
	}

	accepted := apitest.EmptyTrash(t, env, http.StatusAccepted)

	chain := apitest.WaitForTaskChain(t, env, accepted.TaskChainID)
	if chain.Status != "completed" {
		t.Fatalf("chain status = %q, want completed: %+v", chain.Status, chain)
	}
	if chain.Percentage != 100 {
		t.Errorf("percentage = %v, want 100", chain.Percentage)
	}
	if chain.Name != task.EmptyTrashTaskName {
		t.Errorf("name = %q, want %q", chain.Name, task.EmptyTrashTaskName)
	}

	if trashed := apitest.GetJSON[[]map[string]any](t, env, apitest.TrashBooksURL()); len(trashed) != 0 {
		t.Errorf("trashed books after empty = %d, want 0", len(trashed))
	}
}

func TestAPIEmptyTrashOnEmptyTrashContract(t *testing.T) {
	env := apitest.New(t)

	accepted := apitest.EmptyTrash(t, env, http.StatusAccepted)

	chain := apitest.WaitForTaskChain(t, env, accepted.TaskChainID)
	if chain.Status != "completed" {
		t.Errorf("chain status = %q, want completed for an already empty trash", chain.Status)
	}
	if chain.Percentage != 100 {
		t.Errorf("percentage = %v, want 100", chain.Percentage)
	}
}

func TestAPIEmptyTrashRunningChainConflict(t *testing.T) {
	env := apitest.New(t)

	first := apitest.AssertDuplicateChainConflict(t, env, func(wantStatus int) apitest.TaskChainSubmitResponse {
		return apitest.EmptyTrash(t, env, wantStatus)
	})

	if next := apitest.EmptyTrash(t, env, http.StatusAccepted); next.TaskChainID == first.TaskChainID {
		t.Errorf("expected a new chain after the previous one finished")
	}
}

func TestAPIDeleteAndTrashRoutes(t *testing.T) {
	env := apitest.New(t)

	routes := map[string]func(bookID string) *http.Request{
		"DELETE /books/{book_id}": func(bookID string) *http.Request {
			return httptest.NewRequest(http.MethodDelete, apitest.BookURL(bookID), nil)
		},
		"POST /books/{book_id}/trash": func(bookID string) *http.Request {
			return httptest.NewRequest(http.MethodPost, apitest.BookURL(bookID, "trash"), nil)
		},
	}

	for name, build := range routes {
		t.Run(name, func(t *testing.T) {
			book := apitest.ImportTextBook(t, env, "Trash Me", "", "trash-me.txt", "body")
			bookID := book.Meta.ID

			apitest.AssertStatus(t, env.Do(build(bookID)), http.StatusNoContent)

			apitest.AssertStatus(t, env.Get(apitest.BookURL(bookID)), http.StatusNotFound)

			trashed := apitest.GetJSON[[]server.TrashedBook](t, env, apitest.ShelfURL("trash", "books"))
			if !slices.ContainsFunc(trashed, func(b server.TrashedBook) bool { return b.ID == bookID }) {
				t.Fatalf("trash = %+v, want it to contain the trashed book %s", trashed, bookID)
			}
		})
	}
}

func TestAPITrashRoutesUnknownBook(t *testing.T) {
	env := apitest.New(t)

	requests := map[string]*http.Request{
		"DELETE /books/{book_id}": httptest.NewRequest(http.MethodDelete,
			apitest.BookURL("no_such_book"), nil),
		"POST /books/{book_id}/trash": httptest.NewRequest(http.MethodPost,
			apitest.BookURL("no_such_book", "trash"), nil),
	}

	for name, req := range requests {
		t.Run(name, func(t *testing.T) {
			rec := env.Do(req)

			apitest.AssertErrorEnvelope(t, rec, http.StatusNotFound,
				"BOOK_NOT_FOUND", "book not found")
		})
	}
}
