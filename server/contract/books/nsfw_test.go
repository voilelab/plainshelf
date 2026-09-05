package books_test

import (
	"net/http"
	"reflect"
	"testing"

	"github.com/voilelab/plainshelf/server"
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

// The metadata editor's checkbox: PATCH writes the book's own half of the mark,
// and the book then falls under the same filter as one marked on disk. Without
// an endpoint the only way to mark a book was to edit book.json by hand.
//
// See apitest.NewNSFWShelf for the shelf this runs against.
func TestAPIBookNSFWIsWritableThroughPatch(t *testing.T) {
	s := apitest.NewNSFWShelf(t)
	apitest.SetShowNSFW(t, s.Env, true)

	updated := apitest.SetBookNSFW(t, s.Env, s.Visible, true)
	if !updated.Meta.NSFW {
		t.Errorf("PATCH response meta.nsfw = false, want the mark it just wrote")
	}
	if got := apitest.GetJSON[server.Book](t, s.Env, apitest.BookURL(s.Visible)); !got.Meta.NSFW {
		t.Errorf("re-read meta.nsfw = false, want the mark to have reached book.json")
	}

	// The mark is what the filter acts on, so the newly marked book now leaves
	// with the ones marked before it.
	apitest.SetShowNSFW(t, s.Env, false)
	apitest.AssertBookIDs(t, apitest.ListedBookIDs(t, s.Env), s.Classic)
	apitest.AssertStatus(t, s.Env.Get(apitest.BookURL(s.Visible)), http.StatusNotFound)
}

// Clearing the checkbox takes back only the book's own half. The folder rule is
// the shelf's, and a client that could clear it here would be offering a change
// that silently does not happen — which is why the editor disables the control
// and reports the rule instead.
func TestAPIBookNSFWPatchCannotClearAFolderRule(t *testing.T) {
	s := apitest.NewNSFWShelf(t)
	apitest.SetShowNSFW(t, s.Env, true)

	updated := apitest.SetBookNSFW(t, s.Env, s.FolderHidden, false)
	if updated.Meta.NSFW {
		t.Errorf("meta.nsfw = true, want the book's own half cleared")
	}
	if updated.NSFWFolder == nil {
		t.Fatalf("nsfw_folder = nil, want the rule that still marks the book")
	}

	apitest.SetShowNSFW(t, s.Env, false)
	for _, bookID := range s.Hidden() {
		apitest.AssertStatus(t, s.Env.Get(apitest.BookURL(bookID)), http.StatusNotFound)
	}
}

// A book response carries the folder rule marking it, so a client can tell a
// mark it may edit from one it may not, and name where the second came from.
func TestAPIBookReportsTheFolderRuleThatMarksIt(t *testing.T) {
	s := apitest.NewNSFWShelf(t)
	apitest.SetShowNSFW(t, s.Env, true)

	byID := map[string]server.Book{}
	for _, book := range apitest.GetJSON[[]server.Book](t, s.Env, apitest.BooksURL()) {
		byID[book.Meta.ID] = book
	}

	// The listing and the single-book route have to agree: a chip drawn from one
	// and a checkbox drawn from the other would otherwise contradict each other.
	for _, bookID := range s.All() {
		listed, ok := byID[bookID]
		if !ok {
			t.Fatalf("book %s missing from the listing", bookID)
		}
		single := apitest.GetJSON[server.Book](t, s.Env, apitest.BookURL(bookID))
		if !reflect.DeepEqual(listed.NSFWFolder, single.NSFWFolder) {
			t.Errorf("book %s: listing nsfw_folder %+v, single %+v", bookID, listed.NSFWFolder, single.NSFWFolder)
		}
	}

	rule := byID[s.FolderHidden].NSFWFolder
	if rule == nil {
		t.Fatalf("nsfw_folder = nil for the folder-marked book, want the shelf.json rule")
	}
	if rule.Path != apitest.NSFWMarkedFolder || rule.Reason != apitest.NSFWMarkedReason {
		t.Errorf("nsfw_folder = %+v, want the rule as written in shelf.json", rule)
	}

	// The other three are outside the marked subtree, including the one marked
	// by its own book.json: that half is meta.nsfw, and reporting it here as
	// well would tell a client the mark is not editable when it is.
	for _, bookID := range []string{s.Visible, s.Classic, s.BookHidden} {
		if got := byID[bookID].NSFWFolder; got != nil {
			t.Errorf("book %s: nsfw_folder = %+v, want none", bookID, got)
		}
	}
	if !byID[s.BookHidden].Meta.NSFW {
		t.Errorf("book-marked book reports meta.nsfw = false, want its own mark")
	}
}
