package contract_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/voilelab/plainshelf/server"
	"github.com/voilelab/plainshelf/shelf"
)

// SecondShelfID names the extra shelf the cross-shelf transfer tests add
// alongside DefaultShelfID. Only these tests configure two shelves, so it lives
// here rather than in the shared env helper.
const SecondShelfID = "second_shelf"

// WithSecondShelf adds a writable second shelf on libRoot, the destination a
// transfer copies or moves a book into.
func WithSecondShelf(libRoot string) Option {
	return func(conf *server.AppConf) {
		conf.Shelves = append(conf.Shelves, &shelf.ShelfConfWithID{
			ID: SecondShelfID,
			ShelfConf: shelf.ShelfConf{
				LibRoot:           libRoot,
				BookCacheInterval: "1s",
			},
		})
	}
}

// WithReadOnlySecondShelf adds the second shelf opened read-only, so a transfer
// into it is refused the same way any write to a read-only shelf is.
func WithReadOnlySecondShelf(libRoot string) Option {
	return func(conf *server.AppConf) {
		conf.Shelves = append(conf.Shelves, &shelf.ShelfConfWithID{
			ID: SecondShelfID,
			ShelfConf: shelf.ShelfConf{
				LibRoot:           libRoot,
				BookCacheInterval: "1s",
				ReadOnly:          true,
			},
		})
	}
}

func BookTransfersURL(bookID string) string {
	return BookURL(bookID, "transfers")
}

func SecondShelfBooksURL() string {
	return ShelfIDURL(SecondShelfID, "books")
}

// A copy transfer lands a fresh-ID copy on the target shelf and leaves the
// original on the source shelf.
func TestAPIBookTransferCopyContract(t *testing.T) {
	env := New(t, WithSecondShelf(t.TempDir()))
	original := ImportTextBook(t, env, "Cross Copy", "fiction", "crosscopy.txt", "body")

	accepted := SubmitTaskChain(t, env, BookTransfersURL(original.Meta.ID), []byte(`{
		"mode": "copy",
		"target_shelf": "`+SecondShelfID+`",
		"target_folder": ["imported"]
	}`), http.StatusAccepted)
	chain := WaitForTaskChain(t, env, accepted.TaskChainID)
	if chain.Status != "completed" || chain.Percentage != 100 {
		t.Fatalf("chain = %+v, want completed at 100%%", chain)
	}

	result := taskResult[map[string]any](t, chain)
	if result["operation"] != "copy" || result["target_shelf"] != SecondShelfID {
		t.Errorf("result = %#v, want a copy to the second shelf", result)
	}
	newID, _ := result["book_id"].(string)
	if newID == "" || newID == original.Meta.ID {
		t.Fatalf("transferred book_id = %q, want a fresh id distinct from %q", newID, original.Meta.ID)
	}

	// The original stays on the source shelf.
	sourceBooks := GetJSON[[]server.Book](t, env, BooksURL())
	if len(sourceBooks) != 1 || sourceBooks[0].Meta.ID != original.Meta.ID {
		t.Errorf("source books = %#v, want just the original", sourceBooks)
	}

	// The copy is on the target shelf, in the requested folder.
	targetBooks := GetJSON[[]server.Book](t, env, SecondShelfBooksURL())
	if len(targetBooks) != 1 || targetBooks[0].Meta.ID != newID {
		t.Fatalf("target books = %#v, want the copy %q", targetBooks, newID)
	}
	if !targetBooks[0].Folder.Equal(shelf.FolderPath{"imported"}) {
		t.Errorf("copy folder = %v, want [imported]", targetBooks[0].Folder)
	}
	if targetBooks[0].Meta.Title != "Cross Copy" {
		t.Errorf("copy title = %q, want %q", targetBooks[0].Meta.Title, "Cross Copy")
	}
}

// A move transfer keeps the book's ID, publishes it on the target, and removes it
// from the source.
func TestAPIBookTransferMoveContract(t *testing.T) {
	env := New(t, WithSecondShelf(t.TempDir()))
	original := ImportTextBook(t, env, "Cross Move", "fiction", "crossmove.txt", "body")

	accepted := SubmitTaskChain(t, env, BookTransfersURL(original.Meta.ID), []byte(`{
		"mode": "move",
		"target_shelf": "`+SecondShelfID+`"
	}`), http.StatusAccepted)
	chain := WaitForTaskChain(t, env, accepted.TaskChainID)
	if chain.Status != "completed" || chain.Percentage != 100 {
		t.Fatalf("chain = %+v, want completed at 100%%", chain)
	}

	result := taskResult[map[string]any](t, chain)
	if result["operation"] != "move" || result["book_id"] != original.Meta.ID {
		t.Errorf("result = %#v, want a move preserving id %q", result, original.Meta.ID)
	}

	// The source shelf no longer lists it.
	sourceBooks := GetJSON[[]server.Book](t, env, BooksURL())
	if len(sourceBooks) != 0 {
		t.Errorf("source books = %#v, want none after a move", sourceBooks)
	}

	// The target shelf lists it under the original ID, in the source book's folder
	// (no target_folder was given).
	targetBooks := GetJSON[[]server.Book](t, env, SecondShelfBooksURL())
	if len(targetBooks) != 1 || targetBooks[0].Meta.ID != original.Meta.ID {
		t.Fatalf("target books = %#v, want the moved book %q", targetBooks, original.Meta.ID)
	}
	if !targetBooks[0].Folder.Equal(original.Folder) {
		t.Errorf("moved folder = %v, want the source folder %v", targetBooks[0].Folder, original.Folder)
	}
}

// A move onto a target shelf that already holds the book's ID is refused with 409
// before any work is scheduled. The collision is built by moving the book across
// and then restoring the source's trashed copy, so both shelves hold the ID.
func TestAPIBookTransferMoveIDConflictContract(t *testing.T) {
	env := New(t, WithSecondShelf(t.TempDir()))
	original := ImportTextBook(t, env, "Twice", "fiction", "twice.txt", "body")
	id := original.Meta.ID

	first := SubmitTaskChain(t, env, BookTransfersURL(id), []byte(`{
		"mode": "move",
		"target_shelf": "`+SecondShelfID+`"
	}`), http.StatusAccepted)
	if chain := WaitForTaskChain(t, env, first.TaskChainID); chain.Status != "completed" {
		t.Fatalf("first move chain = %+v, want completed", chain)
	}

	// Restore the source's trashed copy so the ID lives on both shelves.
	AssertStatus(t, env.Post(ShelfURL("trash", "books", id, "restore"), nil), http.StatusNoContent)

	rec := env.Post(BookTransfersURL(id), strings.NewReader(`{
		"mode": "move",
		"target_shelf": "`+SecondShelfID+`"
	}`))
	AssertStatus(t, rec, http.StatusConflict)
}

// Naming the source shelf as the target is a 400 that points at the same-shelf
// endpoints, before anything is scheduled.
func TestAPIBookTransferSameShelfContract(t *testing.T) {
	env := New(t, WithSecondShelf(t.TempDir()))
	book := ImportTextBook(t, env, "Here", "", "here.txt", "body")

	rec := env.Post(BookTransfersURL(book.Meta.ID), strings.NewReader(`{
		"mode": "copy",
		"target_shelf": "`+DefaultShelfID+`"
	}`))
	AssertStatus(t, rec, http.StatusBadRequest)
}

// A missing target shelf is a 404, and an unknown mode is a 400.
func TestAPIBookTransferBadRequestContract(t *testing.T) {
	env := New(t, WithSecondShelf(t.TempDir()))
	book := ImportTextBook(t, env, "Bad", "", "bad.txt", "body")

	missing := env.Post(BookTransfersURL(book.Meta.ID), strings.NewReader(`{
		"mode": "copy",
		"target_shelf": "no_such_shelf"
	}`))
	AssertStatus(t, missing, http.StatusNotFound)

	badMode := env.Post(BookTransfersURL(book.Meta.ID), strings.NewReader(`{
		"mode": "duplicate",
		"target_shelf": "`+SecondShelfID+`"
	}`))
	AssertStatus(t, badMode, http.StatusBadRequest)
}

// A transfer into a read-only target shelf is refused with 409, the same status
// every write to a read-only shelf gets, and nothing is scheduled.
func TestAPIBookTransferReadOnlyTargetContract(t *testing.T) {
	env := New(t, WithReadOnlySecondShelf(t.TempDir()))
	book := ImportTextBook(t, env, "No Room", "", "noroom.txt", "body")

	rec := env.Post(BookTransfersURL(book.Meta.ID), strings.NewReader(`{
		"mode": "copy",
		"target_shelf": "`+SecondShelfID+`"
	}`))
	AssertStatus(t, rec, http.StatusConflict)
	if got := rec.Body.String(); strings.Contains(got, "taskchain_id") {
		t.Errorf("body = %s, want a refusal rather than a queued chain", got)
	}
}
