package books_test

import (
	"net/http"
	"testing"

	"github.com/voilelab/plainshelf/server/contract/apitest"
)

// A batch takes book IDs from the client, so it is the one way to name a book
// without going through a route that answers 404. It has to refuse a hidden ID
// the way an unknown one is refused — a chain that moved or trashed it, and
// named it back in succeeded_ids, would undo the whole filter.
//
// See apitest.NewNSFWShelf for the shelf this runs against.
func TestAPINSFWBooksCannotBeReachedThroughABatch(t *testing.T) {
	s := apitest.NewNSFWShelf(t)

	type batchFailure struct {
		BookID string `json:"book_id"`
		Code   string `json:"code"`
	}
	trash := func(bookID string) struct {
		SucceededIDs []string       `json:"succeeded_ids"`
		Failures     []batchFailure `json:"failures"`
	} {
		body := []byte(`{"operation":"trash","book_ids":["` + bookID + `"]}`)
		accepted := apitest.SubmitTaskChain(t, s.Env, apitest.BookBatchURL(), body, http.StatusAccepted)
		return apitest.TaskResult[struct {
			SucceededIDs []string       `json:"succeeded_ids"`
			Failures     []batchFailure `json:"failures"`
		}](t, apitest.WaitForTaskChain(t, s.Env, accepted.TaskChainID))
	}

	unknown := trash("no_such_book")
	if len(unknown.Failures) != 1 {
		t.Fatalf("unknown ID gave %+v, want one failure to compare against", unknown)
	}

	for _, bookID := range s.Hidden() {
		got := trash(bookID)
		if len(got.SucceededIDs) != 0 {
			t.Errorf("batch succeeded on hidden book %s: %v", bookID, got.SucceededIDs)
		}
		if len(got.Failures) != 1 || got.Failures[0].Code != unknown.Failures[0].Code {
			t.Errorf("failures = %+v, want one %q, the answer an unknown ID gets",
				got.Failures, unknown.Failures[0].Code)
		}
	}

	// Nothing was trashed, and the same batch really does work once it may — so
	// the refusals above were the filter, not a batch that never works.
	apitest.SetShowNSFW(t, s.Env, true)
	apitest.AssertBookIDs(t, apitest.ListedBookIDs(t, s.Env), s.All()...)
	if got := trash(s.FolderHidden); len(got.SucceededIDs) != 1 {
		t.Errorf("batch with show_nsfw on = %+v, want the book trashed", got)
	}
}
