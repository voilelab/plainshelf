package books_test

import (
	"net/http"
	"reflect"
	"testing"

	"github.com/voilelab/plainshelf/server"
	"github.com/voilelab/plainshelf/server/contract/apitest"
)

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

	apitest.SetShowNSFW(t, s.Env, true)
	apitest.AssertBookIDs(t, apitest.ListedBookIDs(t, s.Env), s.All()...)
	if got := trash(s.FolderHidden); len(got.SucceededIDs) != 1 {
		t.Errorf("batch with show_nsfw on = %+v, want the book trashed", got)
	}
}

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

	apitest.SetShowNSFW(t, s.Env, false)
	apitest.AssertBookIDs(t, apitest.ListedBookIDs(t, s.Env), s.Classic)
	apitest.AssertStatus(t, s.Env.Get(apitest.BookURL(s.Visible)), http.StatusNotFound)
}

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

func TestAPIBookReportsTheFolderRuleThatMarksIt(t *testing.T) {
	s := apitest.NewNSFWShelf(t)
	apitest.SetShowNSFW(t, s.Env, true)

	byID := map[string]server.Book{}
	for _, book := range apitest.GetJSON[[]server.Book](t, s.Env, apitest.BooksURL()) {
		byID[book.Meta.ID] = book
	}

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

	for _, bookID := range []string{s.Visible, s.Classic, s.BookHidden} {
		if got := byID[bookID].NSFWFolder; got != nil {
			t.Errorf("book %s: nsfw_folder = %+v, want none", bookID, got)
		}
	}
	if !byID[s.BookHidden].Meta.NSFW {
		t.Errorf("book-marked book reports meta.nsfw = false, want its own mark")
	}
}
