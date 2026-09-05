package folders_test

import (
	"net/http"
	"net/http/httptest"
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

// Emptying the trash is not filtered, so a client that quoted the listing's
// length as what the sweep will erase would understate it — it is this header
// that tells the client to warn instead of naming a number. The header says
// only that something is missing, never what.
func TestAPINSFWTrashListingMarksItselfPartial(t *testing.T) {
	s := apitest.NewNSFWShelf(t)
	partial := func() string {
		rec := s.Env.Get(apitest.TrashBooksURL())
		apitest.AssertStatus(t, rec, http.StatusOK)
		return rec.Header().Get(server.TrashListingPartialHeader)
	}

	// Nothing withheld yet: an empty trash, then one holding only a book the
	// request may see. The header must stay off for both, or the warning it
	// drives would replace the count on every ordinary shelf.
	if got := partial(); got != "" {
		t.Errorf("header on an empty trash = %q, want it absent", got)
	}
	trashAllOf(t, s, s.Visible)
	if got := partial(); got != "" {
		t.Errorf("header with nothing withheld = %q, want it absent", got)
	}

	trashAllOf(t, s, s.BookHidden)
	if got := partial(); got != "true" {
		t.Errorf("header with a marked book withheld = %q, want \"true\"", got)
	}

	// It describes this response, not the shelf: with the setting on, the same
	// trash is answered in full and the header goes away again.
	apitest.SetShowNSFW(t, s.Env, true)
	if got := partial(); got != "" {
		t.Errorf("header with show_nsfw on = %q, want it absent", got)
	}
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

/*
Moving a marked folder — or any folder above one — takes its subtree out from
under the shelf.json rule that marks it, so the books below it are served from
the next request on. That is a whole folder's worth of books changing from
hidden to public in one action nobody described, which is why the folder routes
ask first: they answer 409 with nsfwRevealConflictKind and do nothing, and the
caller retries with ?confirm=1.

The mark itself is untouched either way. shelf.json is the user's own file and
PlainShelf only reads it, so the folder really is unmarked afterwards — the
confirmation is the whole of what this adds, not a rewrite of the rule.
*/

// nsfwRevealConflictBody mirrors the 409 body those routes answer with, pinning
// the wire shape the frontend's confirmation dialog reads.
type nsfwRevealConflictBody struct {
	Error       string `json:"error"`
	Message     string `json:"message"`
	HiddenBooks int    `json:"hidden_books"`
}

const nsfwRevealConflictKind = "nsfw_reveal_requires_confirmation"

// assertRevealConflict reads the refusal and checks it names the books it is
// about, so a dialog built on it cannot quote the wrong number.
func assertRevealConflict(t *testing.T, rec *httptest.ResponseRecorder, wantHidden int) {
	t.Helper()

	apitest.AssertStatus(t, rec, http.StatusConflict)
	body := apitest.DecodeJSON[nsfwRevealConflictBody](t, rec)
	if body.Error != nsfwRevealConflictKind {
		t.Errorf("error = %q, want %q", body.Error, nsfwRevealConflictKind)
	}
	if body.HiddenBooks != wantHidden {
		t.Errorf("hidden_books = %d, want %d", body.HiddenBooks, wantHidden)
	}
	if body.Message == "" {
		t.Error("message is empty, want the explanation the dialog shows")
	}
}

// confirmed is the same URL with the flag the caller retries under.
func confirmed(url string) string { return url + "?confirm=1" }

const nsfwMoveBody = `{"folder":["Fiction"],"target_folder":["Archive"]}`

// newNSFWShelfWithArchive adds a root folder outside the marked subtree, which
// is somewhere a move can go.
func newNSFWShelfWithArchive(t *testing.T) apitest.NSFWShelf {
	t.Helper()

	s := apitest.NewNSFWShelf(t)
	apitest.AssertStatus(t, s.Env.Post(apitest.ShelfURL("folders", "Archive"), nil), http.StatusNoContent)
	return s
}

// Moving Fiction under Archive would carry Fiction/Adult to Archive/Fiction/Adult,
// which no rule names. Exactly one of the two hidden books comes out with it:
// BookHidden is marked in its own book.json and stays marked wherever it goes.
func TestAPINSFWFolderMoveAsksBeforeUnhiding(t *testing.T) {
	s := newNSFWShelfWithArchive(t)
	moves := apitest.ShelfURL("folder-moves")

	assertRevealConflict(t, s.Env.Post(moves, strings.NewReader(nsfwMoveBody)), 1)

	// The refusal did nothing: Fiction is where it was, and the books it holds
	// are still filtered the way they were.
	folders := apitest.GetJSON[[]shelf.FolderPath](t, s.Env, apitest.ShelfURL("folders"))
	if !slices.ContainsFunc(folders, func(f shelf.FolderPath) bool { return f.String() == "Fiction" }) {
		t.Fatalf("folders = %v, want Fiction still at the root", folders)
	}
	apitest.AssertBookIDs(t, apitest.ListedBookIDs(t, s.Env), s.Visible, s.Classic)

	apitest.AssertStatus(t, s.Env.Post(confirmed(moves), strings.NewReader(nsfwMoveBody)), http.StatusNoContent)

	// Confirmed, the move happened and did exactly what the refusal said it
	// would: the folder-marked book is served, the book-marked one is not.
	apitest.AssertBookIDs(t, apitest.ListedBookIDs(t, s.Env), s.Visible, s.Classic, s.FolderHidden)
}

// With the setting on there is nothing to reveal, so neither route asks and both
// behave as they did before this existed.
func TestAPINSFWFolderMoveDoesNotAskWhileShowNSFWIsOn(t *testing.T) {
	s := newNSFWShelfWithArchive(t)
	apitest.SetShowNSFW(t, s.Env, true)

	apitest.AssertStatus(t,
		s.Env.Post(apitest.ShelfURL("folder-moves"), strings.NewReader(nsfwMoveBody)),
		http.StatusNoContent)
}

// A move that carries no marked folder is not this rule's business, whichever
// way the setting is set. Fiction/Flagged is the case worth pinning: it holds a
// book the request cannot see, but the mark is the book's own and travels with
// it, so nothing is revealed.
func TestAPINSFWFolderMoveDoesNotAskWithoutAMarkedFolder(t *testing.T) {
	s := newNSFWShelfWithArchive(t)
	moves := apitest.ShelfURL("folder-moves")

	for _, folder := range []string{"Fiction/Classics", apitest.NSFWFlaggedFolder} {
		body := `{"folder":["Fiction","` + strings.Split(folder, "/")[1] + `"],"target_folder":["Archive"]}`
		apitest.AssertStatus(t, s.Env.Post(moves, strings.NewReader(body)), http.StatusNoContent)
	}

	apitest.AssertBookIDs(t, apitest.ListedBookIDs(t, s.Env), s.Visible, s.Classic)
}

// A rename is the same disclosure by another route: the folder keeps its parent
// and stops matching the rule. Both the marked folder itself and the ancestor
// the user can actually see in the tree are held to it.
func TestAPINSFWFolderRenameAsksBeforeUnhiding(t *testing.T) {
	renames := map[string]string{
		"the marked folder itself": apitest.NSFWMarkedFolder,
		"an ancestor of it":        "Fiction",
	}

	for name, folder := range renames {
		t.Run(name, func(t *testing.T) {
			s := apitest.NewNSFWShelf(t)
			url := apitest.ShelfURL("folders", folder)
			body := func() *strings.Reader { return strings.NewReader(`{"name":"General"}`) }

			assertRevealConflict(t, s.Env.Patch(url, body()), 1)
			apitest.AssertBookIDs(t, apitest.ListedBookIDs(t, s.Env), s.Visible, s.Classic)

			apitest.AssertStatus(t, s.Env.Patch(confirmed(url), body()), http.StatusNoContent)
			apitest.AssertBookIDs(t, apitest.ListedBookIDs(t, s.Env), s.Visible, s.Classic, s.FolderHidden)
		})
	}
}

// A folder renamed to a name no rule covers is a reveal even when it is empty:
// the folder's own name is the disclosure, which is why it is dropped from the
// tree whether or not it holds a book. hidden_books is 0 there, and a client
// must not read that as "nothing would change".
func TestAPINSFWFolderRenameAsksForAnEmptyMarkedFolder(t *testing.T) {
	s := apitest.NewNSFWShelf(t)

	apitest.SetShowNSFW(t, s.Env, true)
	apitest.AssertStatus(t, s.Env.Post(apitest.BookURL(s.FolderHidden, "trash"), nil), http.StatusNoContent)
	apitest.SetShowNSFW(t, s.Env, false)

	assertRevealConflict(t, s.Env.Patch(
		apitest.ShelfURL("folders", apitest.NSFWMarkedFolder),
		strings.NewReader(`{"name":"General"}`)), 0)
}

/*
A cross-shelf transfer is judged on the source alone. Only a book's own nsfw
travels with it — that is written in its book.json — while shelf.json stays
behind, so whether the target shelf happens to mark the same path is not this
shelf's answer to give. A copy is asked the same question as a move: it leaves
this shelf untouched but publishes the same titles on the other one.
*/
func TestAPINSFWFolderTransferAsksBeforeUnhiding(t *testing.T) {
	for _, mode := range []string{"copy", "move"} {
		t.Run(mode, func(t *testing.T) {
			s := apitest.NewNSFWShelf(t, apitest.WithSecondShelf(t.TempDir()))
			transfers := apitest.FolderTransfersURL()
			body := `{"mode":"` + mode + `","source_folder":["Fiction"],` +
				`"target_shelf":"` + apitest.SecondShelfID + `","target_folder":["Imported"]}`

			assertRevealConflict(t, s.Env.Post(transfers, strings.NewReader(body)), 1)
			if books := apitest.GetJSON[[]server.Book](t, s.Env, apitest.SecondShelfBooksURL()); len(books) != 0 {
				t.Fatalf("target books after the refusal = %#v, want none", books)
			}

			accepted := apitest.SubmitTaskChain(t, s.Env, confirmed(transfers), []byte(body), http.StatusAccepted)
			if chain := apitest.WaitForTaskChain(t, s.Env, accepted.TaskChainID); chain.Status != "completed" {
				t.Fatalf("chain status = %q, want completed: %+v", chain.Status, chain)
			}
		})
	}
}

// The reverse half of the transfer rule: a source folder holding no marked
// folder transfers without a confirmation, exactly as it did before.
func TestAPINSFWFolderTransferDoesNotAskWithoutAMarkedFolder(t *testing.T) {
	s := apitest.NewNSFWShelf(t, apitest.WithSecondShelf(t.TempDir()))

	apitest.SubmitTaskChain(t, s.Env, apitest.FolderTransfersURL(), []byte(`{
		"mode": "copy",
		"source_folder": ["Fiction", "Classics"],
		"target_shelf": "`+apitest.SecondShelfID+`",
		"target_folder": ["Imported"]
	}`), http.StatusAccepted)
}
