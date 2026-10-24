package books_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/voilelab/plainshelf/server/contract/apitest"

	"github.com/voilelab/plainshelf/server"
	"github.com/voilelab/plainshelf/shelf"
)

func TestAPIBookTransferCopyContract(t *testing.T) {
	env := apitest.New(t, apitest.WithSecondShelf(t.TempDir()))
	original := apitest.ImportTextBook(t, env, "Cross Copy", "fiction", "crosscopy.txt", "body")

	accepted := apitest.SubmitTaskChain(t, env, apitest.BookTransfersURL(original.Meta.ID), []byte(`{
		"mode": "copy",
		"target_shelf": "`+apitest.SecondShelfID+`",
		"target_folder": ["imported"]
	}`), http.StatusAccepted)
	chain := apitest.WaitForTaskChain(t, env, accepted.TaskChainID)
	if chain.Status != "completed" || chain.Percentage != 100 {
		t.Fatalf("chain = %+v, want completed at 100%%", chain)
	}

	result := apitest.TaskResult[map[string]any](t, chain)
	if result["operation"] != "copy" || result["target_shelf"] != apitest.SecondShelfID {
		t.Errorf("result = %#v, want a copy to the second shelf", result)
	}
	newID, _ := result["book_id"].(string)
	if newID == "" || newID == original.Meta.ID {
		t.Fatalf("transferred book_id = %q, want a fresh id distinct from %q", newID, original.Meta.ID)
	}

	sourceBooks := apitest.GetJSON[[]server.Book](t, env, apitest.BooksURL())
	if len(sourceBooks) != 1 || sourceBooks[0].Meta.ID != original.Meta.ID {
		t.Errorf("source books = %#v, want just the original", sourceBooks)
	}

	targetBooks := apitest.GetJSON[[]server.Book](t, env, apitest.SecondShelfBooksURL())
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

func TestAPIBookTransferMoveContract(t *testing.T) {
	env := apitest.New(t, apitest.WithSecondShelf(t.TempDir()))
	original := apitest.ImportTextBook(t, env, "Cross Move", "fiction", "crossmove.txt", "body")

	accepted := apitest.SubmitTaskChain(t, env, apitest.BookTransfersURL(original.Meta.ID), []byte(`{
		"mode": "move",
		"target_shelf": "`+apitest.SecondShelfID+`"
	}`), http.StatusAccepted)
	chain := apitest.WaitForTaskChain(t, env, accepted.TaskChainID)
	if chain.Status != "completed" || chain.Percentage != 100 {
		t.Fatalf("chain = %+v, want completed at 100%%", chain)
	}

	result := apitest.TaskResult[map[string]any](t, chain)
	if result["operation"] != "move" || result["book_id"] != original.Meta.ID {
		t.Errorf("result = %#v, want a move preserving id %q", result, original.Meta.ID)
	}

	sourceBooks := apitest.GetJSON[[]server.Book](t, env, apitest.BooksURL())
	if len(sourceBooks) != 0 {
		t.Errorf("source books = %#v, want none after a move", sourceBooks)
	}

	targetBooks := apitest.GetJSON[[]server.Book](t, env, apitest.SecondShelfBooksURL())
	if len(targetBooks) != 1 || targetBooks[0].Meta.ID != original.Meta.ID {
		t.Fatalf("target books = %#v, want the moved book %q", targetBooks, original.Meta.ID)
	}
	if !targetBooks[0].Folder.Equal(original.Folder) {
		t.Errorf("moved folder = %v, want the source folder %v", targetBooks[0].Folder, original.Folder)
	}
}

func TestAPIBookTransferMoveIDConflictContract(t *testing.T) {
	env := apitest.New(t, apitest.WithSecondShelf(t.TempDir()))
	original := apitest.ImportTextBook(t, env, "Twice", "fiction", "twice.txt", "body")
	id := original.Meta.ID

	first := apitest.SubmitTaskChain(t, env, apitest.BookTransfersURL(id), []byte(`{
		"mode": "move",
		"target_shelf": "`+apitest.SecondShelfID+`"
	}`), http.StatusAccepted)
	if chain := apitest.WaitForTaskChain(t, env, first.TaskChainID); chain.Status != "completed" {
		t.Fatalf("first move chain = %+v, want completed", chain)
	}

	apitest.AssertStatus(t, env.Post(apitest.ShelfURL("trash", "books", id, "restore"), nil), http.StatusNoContent)

	rec := env.Post(apitest.BookTransfersURL(id), strings.NewReader(`{
		"mode": "move",
		"target_shelf": "`+apitest.SecondShelfID+`"
	}`))
	apitest.AssertStatus(t, rec, http.StatusConflict)
}

func TestAPIBookTransferSameShelfContract(t *testing.T) {
	env := apitest.New(t, apitest.WithSecondShelf(t.TempDir()))
	book := apitest.ImportTextBook(t, env, "Here", "", "here.txt", "body")

	rec := env.Post(apitest.BookTransfersURL(book.Meta.ID), strings.NewReader(`{
		"mode": "copy",
		"target_shelf": "`+apitest.DefaultShelfID+`"
	}`))
	apitest.AssertStatus(t, rec, http.StatusBadRequest)
}

func TestAPIBookTransferBadRequestContract(t *testing.T) {
	env := apitest.New(t, apitest.WithSecondShelf(t.TempDir()))
	book := apitest.ImportTextBook(t, env, "Bad", "", "bad.txt", "body")

	missing := env.Post(apitest.BookTransfersURL(book.Meta.ID), strings.NewReader(`{
		"mode": "copy",
		"target_shelf": "no_such_shelf"
	}`))
	apitest.AssertStatus(t, missing, http.StatusNotFound)

	badMode := env.Post(apitest.BookTransfersURL(book.Meta.ID), strings.NewReader(`{
		"mode": "duplicate",
		"target_shelf": "`+apitest.SecondShelfID+`"
	}`))
	apitest.AssertStatus(t, badMode, http.StatusBadRequest)
}

func TestAPIBookTransferReadOnlyTargetContract(t *testing.T) {
	env := apitest.New(t, apitest.WithReadOnlySecondShelf(t.TempDir()))
	book := apitest.ImportTextBook(t, env, "No Room", "", "noroom.txt", "body")

	rec := env.Post(apitest.BookTransfersURL(book.Meta.ID), strings.NewReader(`{
		"mode": "copy",
		"target_shelf": "`+apitest.SecondShelfID+`"
	}`))
	apitest.AssertStatus(t, rec, http.StatusConflict)
	if got := rec.Body.String(); strings.Contains(got, "taskchain_id") {
		t.Errorf("body = %s, want a refusal rather than a queued chain", got)
	}
}
