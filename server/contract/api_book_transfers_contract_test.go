package contract_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/voilelab/plainshelf/server"
	"github.com/voilelab/plainshelf/shelf"
)

// secondShelfID names the extra shelf the cross-shelf transfer tests add
// alongside defaultShelfID. Only these tests configure two shelves, so it lives
// here rather than in the shared env helper.
const secondShelfID = "second_shelf"

// withSecondShelf adds a writable second shelf on libRoot, the destination a
// transfer copies or moves a book into.
func withSecondShelf(libRoot string) appConfOption {
	return func(conf *server.AppConf) {
		conf.Shelves = append(conf.Shelves, &shelf.ShelfConfWithID{
			ID: secondShelfID,
			ShelfConf: shelf.ShelfConf{
				LibRoot:           libRoot,
				BookCacheInterval: "1s",
			},
		})
	}
}

// withReadOnlySecondShelf adds the second shelf opened read-only, so a transfer
// into it is refused the same way any write to a read-only shelf is.
func withReadOnlySecondShelf(libRoot string) appConfOption {
	return func(conf *server.AppConf) {
		conf.Shelves = append(conf.Shelves, &shelf.ShelfConfWithID{
			ID: secondShelfID,
			ShelfConf: shelf.ShelfConf{
				LibRoot:           libRoot,
				BookCacheInterval: "1s",
				ReadOnly:          true,
			},
		})
	}
}

func bookTransfersURL(bookID string) string {
	return bookURL(bookID, "transfers")
}

func secondShelfBooksURL() string {
	return shelfIDURL(secondShelfID, "books")
}

// A copy transfer lands a fresh-ID copy on the target shelf and leaves the
// original on the source shelf.
func TestAPIBookTransferCopyContract(t *testing.T) {
	env := newAPITestEnv(t, withSecondShelf(t.TempDir()))
	original := importTextBook(t, env, "Cross Copy", "fiction", "crosscopy.txt", "body")

	accepted := submitTaskChain(t, env, bookTransfersURL(original.Meta.ID), []byte(`{
		"mode": "copy",
		"target_shelf": "`+secondShelfID+`",
		"target_layer": ["imported"]
	}`), http.StatusAccepted)
	chain := waitForTaskChain(t, env, accepted.TaskChainID)
	if chain.Status != "completed" || chain.Percentage != 100 {
		t.Fatalf("chain = %+v, want completed at 100%%", chain)
	}

	result := taskResult[map[string]any](t, chain)
	if result["operation"] != "copy" || result["target_shelf"] != secondShelfID {
		t.Errorf("result = %#v, want a copy to the second shelf", result)
	}
	newID, _ := result["book_id"].(string)
	if newID == "" || newID == original.Meta.ID {
		t.Fatalf("transferred book_id = %q, want a fresh id distinct from %q", newID, original.Meta.ID)
	}

	// The original stays on the source shelf.
	sourceBooks := getJSON[[]server.Book](t, env, booksURL())
	if len(sourceBooks) != 1 || sourceBooks[0].Meta.ID != original.Meta.ID {
		t.Errorf("source books = %#v, want just the original", sourceBooks)
	}

	// The copy is on the target shelf, in the requested layer.
	targetBooks := getJSON[[]server.Book](t, env, secondShelfBooksURL())
	if len(targetBooks) != 1 || targetBooks[0].Meta.ID != newID {
		t.Fatalf("target books = %#v, want the copy %q", targetBooks, newID)
	}
	if !targetBooks[0].Layer.Equal(shelf.FolderPath{"imported"}) {
		t.Errorf("copy layer = %v, want [imported]", targetBooks[0].Layer)
	}
	if targetBooks[0].Meta.Title != "Cross Copy" {
		t.Errorf("copy title = %q, want %q", targetBooks[0].Meta.Title, "Cross Copy")
	}
}

// A move transfer keeps the book's ID, publishes it on the target, and removes it
// from the source.
func TestAPIBookTransferMoveContract(t *testing.T) {
	env := newAPITestEnv(t, withSecondShelf(t.TempDir()))
	original := importTextBook(t, env, "Cross Move", "fiction", "crossmove.txt", "body")

	accepted := submitTaskChain(t, env, bookTransfersURL(original.Meta.ID), []byte(`{
		"mode": "move",
		"target_shelf": "`+secondShelfID+`"
	}`), http.StatusAccepted)
	chain := waitForTaskChain(t, env, accepted.TaskChainID)
	if chain.Status != "completed" || chain.Percentage != 100 {
		t.Fatalf("chain = %+v, want completed at 100%%", chain)
	}

	result := taskResult[map[string]any](t, chain)
	if result["operation"] != "move" || result["book_id"] != original.Meta.ID {
		t.Errorf("result = %#v, want a move preserving id %q", result, original.Meta.ID)
	}

	// The source shelf no longer lists it.
	sourceBooks := getJSON[[]server.Book](t, env, booksURL())
	if len(sourceBooks) != 0 {
		t.Errorf("source books = %#v, want none after a move", sourceBooks)
	}

	// The target shelf lists it under the original ID, in the source book's layer
	// (no target_layer was given).
	targetBooks := getJSON[[]server.Book](t, env, secondShelfBooksURL())
	if len(targetBooks) != 1 || targetBooks[0].Meta.ID != original.Meta.ID {
		t.Fatalf("target books = %#v, want the moved book %q", targetBooks, original.Meta.ID)
	}
	if !targetBooks[0].Layer.Equal(original.Layer) {
		t.Errorf("moved layer = %v, want the source layer %v", targetBooks[0].Layer, original.Layer)
	}
}

// A move onto a target shelf that already holds the book's ID is refused with 409
// before any work is scheduled. The collision is built by moving the book across
// and then restoring the source's trashed copy, so both shelves hold the ID.
func TestAPIBookTransferMoveIDConflictContract(t *testing.T) {
	env := newAPITestEnv(t, withSecondShelf(t.TempDir()))
	original := importTextBook(t, env, "Twice", "fiction", "twice.txt", "body")
	id := original.Meta.ID

	first := submitTaskChain(t, env, bookTransfersURL(id), []byte(`{
		"mode": "move",
		"target_shelf": "`+secondShelfID+`"
	}`), http.StatusAccepted)
	if chain := waitForTaskChain(t, env, first.TaskChainID); chain.Status != "completed" {
		t.Fatalf("first move chain = %+v, want completed", chain)
	}

	// Restore the source's trashed copy so the ID lives on both shelves.
	assertStatus(t, env.post(shelfURL("trash", "books", id, "restore"), nil), http.StatusNoContent)

	rec := env.post(bookTransfersURL(id), strings.NewReader(`{
		"mode": "move",
		"target_shelf": "`+secondShelfID+`"
	}`))
	assertStatus(t, rec, http.StatusConflict)
}

// Naming the source shelf as the target is a 400 that points at the same-shelf
// endpoints, before anything is scheduled.
func TestAPIBookTransferSameShelfContract(t *testing.T) {
	env := newAPITestEnv(t, withSecondShelf(t.TempDir()))
	book := importTextBook(t, env, "Here", "", "here.txt", "body")

	rec := env.post(bookTransfersURL(book.Meta.ID), strings.NewReader(`{
		"mode": "copy",
		"target_shelf": "`+defaultShelfID+`"
	}`))
	assertStatus(t, rec, http.StatusBadRequest)
}

// A missing target shelf is a 404, and an unknown mode is a 400.
func TestAPIBookTransferBadRequestContract(t *testing.T) {
	env := newAPITestEnv(t, withSecondShelf(t.TempDir()))
	book := importTextBook(t, env, "Bad", "", "bad.txt", "body")

	missing := env.post(bookTransfersURL(book.Meta.ID), strings.NewReader(`{
		"mode": "copy",
		"target_shelf": "no_such_shelf"
	}`))
	assertStatus(t, missing, http.StatusNotFound)

	badMode := env.post(bookTransfersURL(book.Meta.ID), strings.NewReader(`{
		"mode": "duplicate",
		"target_shelf": "`+secondShelfID+`"
	}`))
	assertStatus(t, badMode, http.StatusBadRequest)
}

// A transfer into a read-only target shelf is refused with 409, the same status
// every write to a read-only shelf gets, and nothing is scheduled.
func TestAPIBookTransferReadOnlyTargetContract(t *testing.T) {
	env := newAPITestEnv(t, withReadOnlySecondShelf(t.TempDir()))
	book := importTextBook(t, env, "No Room", "", "noroom.txt", "body")

	rec := env.post(bookTransfersURL(book.Meta.ID), strings.NewReader(`{
		"mode": "copy",
		"target_shelf": "`+secondShelfID+`"
	}`))
	assertStatus(t, rec, http.StatusConflict)
	if got := rec.Body.String(); strings.Contains(got, "taskchain_id") {
		t.Errorf("body = %s, want a refusal rather than a queued chain", got)
	}
}

// The transfer endpoint stays behind both write gates: the token requirement and
// the read-only refusal.
func TestAPIBookTransferIsGatedContract(t *testing.T) {
	env := newAPITestEnv(t, withSecondShelf(t.TempDir()))
	book := importTextBook(t, env, "Gated", "", "gated.txt", "body")

	assertMutationGated(t, env, http.MethodPost, bookTransfersURL(book.Meta.ID),
		[]byte(`{"mode":"copy","target_shelf":"`+secondShelfID+`"}`))
}
