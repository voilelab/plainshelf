package folders_test

import (
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/voilelab/plainshelf/server/contract/apitest"

	"github.com/voilelab/plainshelf/server"
	"github.com/voilelab/plainshelf/shelf"
)

// A folder goes out of the tree with its books, so a name that is usually the
// whole disclosure cannot survive in a breadcrumb or in the destination list
// when moving a book. See apitest.NewNSFWShelf for the shelf this runs against.
func TestAPINSFWFoldersLeaveTheTreeWithTheirBooks(t *testing.T) {
	s := apitest.NewNSFWShelf(t)

	folders := func() []string {
		paths := []string{}
		for _, folder := range apitest.GetJSON[[]shelf.FolderPath](t, s.Env, apitest.ShelfURL("folders")) {
			paths = append(paths, folder.String())
		}
		return paths
	}
	assertListed := func(got []string, want bool, folders ...string) {
		t.Helper()
		for _, folder := range folders {
			if slices.Contains(got, folder) != want {
				t.Errorf("folders = %v, want %q listed = %v", got, folder, want)
			}
		}
	}

	got := folders()
	// Fiction still holds Classic, and Empty never held a book at all, so
	// neither is this setting's to remove.
	assertListed(got, true, "Fiction", "Fiction/Classics", apitest.NSFWEmptyFolder)
	// Marked, and left holding only a marked book, respectively.
	assertListed(got, false, apitest.NSFWMarkedFolder, apitest.NSFWFlaggedFolder)

	apitest.SetShowNSFW(t, s.Env, true)
	assertListed(folders(), true, apitest.NSFWMarkedFolder, apitest.NSFWFlaggedFolder)
}

/*
The trash is a listing like any other, so show_nsfw filters it too. It reaches
this fixture only through a detour: a marked book cannot be trashed while the
setting is off, so each test below trashes with the setting on and then turns it
off, which is the order PSW-94's mark button makes an everyday one.

The folder half of the mark still applies in the trash, because the trash record
remembers the folder the book was deleted from — trash/ is outside books/, so
shelf.json's rules cannot reach the book by its current path.
*/

// trashAllOf trashes the given books with the setting on, then turns it back
// off, leaving a trash whose contents the request may not all see.
func trashAllOf(t *testing.T, s apitest.NSFWShelf, bookIDs ...string) {
	t.Helper()

	apitest.SetShowNSFW(t, s.Env, true)
	for _, bookID := range bookIDs {
		apitest.AssertStatus(t, s.Env.Post(apitest.BookURL(bookID, "trash"), nil), http.StatusNoContent)
	}
	apitest.SetShowNSFW(t, s.Env, false)
}

// The disclosure a trash listing carries is the book's title, its authors and
// the folder it was deleted from, so the assertion is against the response body
// rather than the decoded IDs alone.
func TestAPINSFWBooksAreAbsentFromTheTrash(t *testing.T) {
	s := apitest.NewNSFWShelf(t)
	trashAllOf(t, s, s.Visible, s.FolderHidden, s.BookHidden)

	rec := s.Env.Get(apitest.TrashBooksURL())
	apitest.AssertStatus(t, rec, http.StatusOK)
	body := rec.Body.String()
	for _, disclosure := range []string{
		"FolderHidden", "BookHidden", apitest.NSFWMarkedFolder, apitest.NSFWFlaggedFolder,
	} {
		if strings.Contains(body, disclosure) {
			t.Errorf("trash body contains %q, want it withheld: %s", disclosure, body)
		}
	}

	trashedIDs := func() []string {
		ids := []string{}
		for _, book := range apitest.GetJSON[[]server.TrashedBook](t, s.Env, apitest.TrashBooksURL()) {
			ids = append(ids, book.ID)
		}
		return ids
	}
	apitest.AssertBookIDs(t, trashedIDs(), s.Visible)

	// The reverse half: nothing was deleted, and the whole trash is served once
	// the setting says it may be.
	apitest.SetShowNSFW(t, s.Env, true)
	apitest.AssertBookIDs(t, trashedIDs(), s.Visible, s.FolderHidden, s.BookHidden)
}

// Restoring or erasing a book the listing just withheld would confirm it is
// there, so both answer with the envelope an ID that was never issued gets.
func TestAPINSFWTrashedBooksAnswerNotFoundWhereverTheyAreNamed(t *testing.T) {
	s := apitest.NewNSFWShelf(t)
	trashAllOf(t, s, s.FolderHidden, s.BookHidden)

	routes := []struct {
		method string
		elem   []string
	}{
		{method: http.MethodPost, elem: []string{"restore"}},
		{method: http.MethodDelete},
	}

	for _, tc := range routes {
		t.Run(tc.method+" "+strings.Join(tc.elem, "/"), func(t *testing.T) {
			for _, bookID := range append([]string{"no_such_book"}, s.Hidden()...) {
				rec := s.Env.Request(tc.method, apitest.TrashBooksURL(append([]string{bookID}, tc.elem...)...), nil)
				apitest.AssertErrorEnvelope(t, rec, http.StatusNotFound,
					"TRASHED_BOOK_NOT_FOUND", "trashed book not found")
			}
		})
	}

	// The reverse half: the refusals were the filter, so both books are still
	// in the trash and still restorable once they may be seen.
	apitest.SetShowNSFW(t, s.Env, true)
	for _, bookID := range s.Hidden() {
		apitest.AssertStatus(t, s.Env.Post(apitest.TrashBooksURL(bookID, "restore"), nil), http.StatusNoContent)
	}
}

// Emptying the trash is one command over the whole trash rather than a listing,
// so it erases the marked books too. Leaving them behind would be worse than
// the disclosure: they would reappear the next time the setting is turned on,
// long after the user believed the trash was empty.
func TestAPINSFWEmptyTrashStillErasesEverything(t *testing.T) {
	s := apitest.NewNSFWShelf(t)
	trashAllOf(t, s, s.Visible, s.FolderHidden, s.BookHidden)

	accepted := apitest.EmptyTrash(t, s.Env, http.StatusAccepted)
	if chain := apitest.WaitForTaskChain(t, s.Env, accepted.TaskChainID); chain.Status != "completed" {
		t.Fatalf("chain status = %q, want completed: %+v", chain.Status, chain)
	}

	apitest.SetShowNSFW(t, s.Env, true)
	if trashed := apitest.GetJSON[[]server.TrashedBook](t, s.Env, apitest.TrashBooksURL()); len(trashed) != 0 {
		t.Errorf("trashed books after empty = %+v, want none", trashed)
	}
}
