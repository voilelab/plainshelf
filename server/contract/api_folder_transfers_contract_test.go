package contract_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/voilelab/plainshelf/server"
	"github.com/voilelab/plainshelf/shelf"
)

// folderTransfersURL is the cross-shelf folder transfer endpoint on the default
// (source) shelf. secondShelfID and its helpers come from the book-transfer
// contract test in this same package.
func folderTransfersURL() string {
	return shelfURL("folder-transfers")
}

// folderTransferConflictBody mirrors the 409 body a pre-flight refusal answers
// with, so a test can tell the two conflict kinds apart and read the colliding
// IDs. Restating it here pins the wire shape the frontend relies on.
type folderTransferConflictBody struct {
	Error              string   `json:"error"`
	Message            string   `json:"message"`
	ConflictingBookIDs []string `json:"conflicting_book_ids"`
}

// booksUnderFolder returns the IDs of the shelf's books that sit exactly under
// folder, so a test can assert the set that landed there.
func booksUnderFolder(books []server.Book, folder shelf.FolderPath) []string {
	var ids []string
	for _, book := range books {
		if book.Folder.Equal(folder) {
			ids = append(ids, book.Meta.ID)
		}
	}
	return ids
}

// A copy transfer reproduces the source subtree on the target with fresh IDs and
// leaves the source shelf untouched.
func TestAPIFolderTransferCopyContract(t *testing.T) {
	env := newAPITestEnv(t, withSecondShelf(t.TempDir()))
	top := importTextBook(t, env, "Top", "fiction", "top.txt", "a")
	deep := importTextBook(t, env, "Deep", "fiction/sci-fi", "deep.txt", "b")

	accepted := submitTaskChain(t, env, folderTransfersURL(), []byte(`{
		"mode": "copy",
		"source_folder": ["fiction"],
		"target_shelf": "`+secondShelfID+`",
		"target_folder": ["imported"]
	}`), http.StatusAccepted)
	chain := waitForTaskChain(t, env, accepted.TaskChainID)
	if chain.Status != "completed" || chain.Percentage != 100 {
		t.Fatalf("chain = %+v, want completed at 100%%", chain)
	}

	result := taskResult[map[string]any](t, chain)
	if result["operation"] != "copy" || result["target_shelf"] != secondShelfID || result["total"] != float64(2) {
		t.Errorf("result = %#v, want a copy of two books to the second shelf", result)
	}

	// The source keeps both originals.
	sourceBooks := getJSON[[]server.Book](t, env, booksURL())
	if len(sourceBooks) != 2 {
		t.Errorf("source books = %#v, want the two originals", sourceBooks)
	}

	// The target holds fresh-ID copies under the remapped folders.
	targetBooks := getJSON[[]server.Book](t, env, secondShelfBooksURL())
	topCopies := booksUnderFolder(targetBooks, shelf.FolderPath{"imported"})
	deepCopies := booksUnderFolder(targetBooks, shelf.FolderPath{"imported", "sci-fi"})
	if len(topCopies) != 1 || topCopies[0] == top.Meta.ID {
		t.Errorf("target [imported] = %v, want one fresh-ID copy of %q", topCopies, top.Meta.ID)
	}
	if len(deepCopies) != 1 || deepCopies[0] == deep.Meta.ID {
		t.Errorf("target [imported sci-fi] = %v, want one fresh-ID copy of %q", deepCopies, deep.Meta.ID)
	}
}

// A move transfer keeps every book's ID, publishes the subtree on the target, and
// clears the source folder. With no target_folder it lands under the source folder's
// own name.
func TestAPIFolderTransferMoveContract(t *testing.T) {
	env := newAPITestEnv(t, withSecondShelf(t.TempDir()))
	top := importTextBook(t, env, "Top", "fiction", "top.txt", "a")
	deep := importTextBook(t, env, "Deep", "fiction/sci-fi", "deep.txt", "b")

	accepted := submitTaskChain(t, env, folderTransfersURL(), []byte(`{
		"mode": "move",
		"source_folder": ["fiction"],
		"target_shelf": "`+secondShelfID+`"
	}`), http.StatusAccepted)
	chain := waitForTaskChain(t, env, accepted.TaskChainID)
	if chain.Status != "completed" || chain.Percentage != 100 {
		t.Fatalf("chain = %+v, want completed at 100%%", chain)
	}

	// The source shelf no longer lists either book.
	sourceBooks := getJSON[[]server.Book](t, env, booksURL())
	if len(sourceBooks) != 0 {
		t.Errorf("source books = %#v, want none after a move", sourceBooks)
	}

	// The target lists both under their original IDs, defaulting to the source
	// folder name.
	targetBooks := getJSON[[]server.Book](t, env, secondShelfBooksURL())
	if got := booksUnderFolder(targetBooks, shelf.FolderPath{"fiction"}); len(got) != 1 || got[0] != top.Meta.ID {
		t.Errorf("target [fiction] = %v, want [%s]", got, top.Meta.ID)
	}
	if got := booksUnderFolder(targetBooks, shelf.FolderPath{"fiction", "sci-fi"}); len(got) != 1 || got[0] != deep.Meta.ID {
		t.Errorf("target [fiction sci-fi] = %v, want [%s]", got, deep.Meta.ID)
	}
}

// A transfer onto a folder the target shelf already holds is refused with a 409
// that names the conflict, before any work is scheduled.
func TestAPIFolderTransferFolderConflictContract(t *testing.T) {
	env := newAPITestEnv(t, withSecondShelf(t.TempDir()))
	importTextBook(t, env, "Source", "fiction", "src.txt", "a")
	// The target already holds the destination folder, so the transfer must refuse
	// to land on it rather than merge into it.
	assertStatus(t, env.post(shelfIDURL(secondShelfID, "folders", "taken"), nil), http.StatusNoContent)

	rec := env.post(folderTransfersURL(), strings.NewReader(`{
		"mode": "copy",
		"source_folder": ["fiction"],
		"target_shelf": "`+secondShelfID+`",
		"target_folder": ["taken"]
	}`))
	assertStatus(t, rec, http.StatusConflict)
	if strings.Contains(rec.Body.String(), "taskchain_id") {
		t.Errorf("body = %s, want a refusal rather than a queued chain", rec.Body.String())
	}
	body := decodeJSON[folderTransferConflictBody](t, rec)
	if body.Error != "target_folder_conflict" {
		t.Errorf("error = %q, want target_folder_conflict", body.Error)
	}
}

// The transfer preflight forces a walk of both shelves, but it is not the walk
// the rescan rate limit governs: that budget belongs to the button a user
// presses, and a run of transfers must not leave it answering 429 for something
// the user never pressed.
func TestAPIFolderTransferPreflightDoesNotSpendTheRescanRateLimitContract(t *testing.T) {
	env := newAPITestEnv(t, withSecondShelf(t.TempDir()))
	importTextBook(t, env, "Source", "fiction", "src.txt", "a")
	assertStatus(t, env.post(shelfIDURL(secondShelfID, "folders", "taken"), nil), http.StatusNoContent)

	// Refused after the preflight, so each round costs both walks without
	// scheduling any work — the cheapest way to run the preflight many times.
	body := `{
		"mode": "copy",
		"source_folder": ["fiction"],
		"target_shelf": "` + secondShelfID + `",
		"target_folder": ["taken"]
	}`
	for i := range 12 {
		rec := env.post(folderTransfersURL(), strings.NewReader(body))
		if rec.Code != http.StatusConflict {
			t.Fatalf("transfer %d: status = %d, want 409", i+1, rec.Code)
		}
	}

	assertStatus(t, env.post(scansURL(), nil), http.StatusOK)
}

// A move whose books collide with IDs the target already holds is refused with a
// 409 that lists every colliding ID, before any work is scheduled.
func TestAPIFolderTransferBookIDConflictContract(t *testing.T) {
	env := newAPITestEnv(t, withSecondShelf(t.TempDir()))
	original := importTextBook(t, env, "Twice", "fiction", "twice.txt", "body")
	id := original.Meta.ID

	// Move the book across with the single-book endpoint, then restore the source's
	// trashed copy, so the ID lives on both shelves.
	moved := submitTaskChain(t, env, bookTransfersURL(id), []byte(`{
		"mode": "move",
		"target_shelf": "`+secondShelfID+`"
	}`), http.StatusAccepted)
	if chain := waitForTaskChain(t, env, moved.TaskChainID); chain.Status != "completed" {
		t.Fatalf("single-book move chain = %+v, want completed", chain)
	}
	assertStatus(t, env.post(shelfURL("trash", "books", id, "restore"), nil), http.StatusNoContent)

	rec := env.post(folderTransfersURL(), strings.NewReader(`{
		"mode": "move",
		"source_folder": ["fiction"],
		"target_shelf": "`+secondShelfID+`",
		"target_folder": ["elsewhere"]
	}`))
	assertStatus(t, rec, http.StatusConflict)
	if strings.Contains(rec.Body.String(), "taskchain_id") {
		t.Errorf("body = %s, want a refusal rather than a queued chain", rec.Body.String())
	}
	body := decodeJSON[folderTransferConflictBody](t, rec)
	if body.Error != "book_id_conflict" {
		t.Errorf("error = %q, want book_id_conflict", body.Error)
	}
	if len(body.ConflictingBookIDs) != 1 || body.ConflictingBookIDs[0] != id {
		t.Errorf("conflicting_book_ids = %v, want [%s]", body.ConflictingBookIDs, id)
	}
}

// Re-submitting a transfer already in flight is a 409 that points at the running
// chain, so the client can attach to it instead of starting a second.
func TestAPIFolderTransferDuplicateRunningChainContract(t *testing.T) {
	env := newAPITestEnv(t, withSecondShelf(t.TempDir()))
	importTextBook(t, env, "Queued", "fiction", "queued.txt", "body")

	body := []byte(`{
		"mode": "copy",
		"source_folder": ["fiction"],
		"target_shelf": "` + secondShelfID + `",
		"target_folder": ["imported"]
	}`)
	assertDuplicateChainConflict(t, env, func(wantStatus int) taskChainSubmitResponse {
		return submitTaskChain(t, env, folderTransfersURL(), body, wantStatus)
	})
}

// A transfer into a read-only target shelf is refused with 409, the status every
// write to a read-only shelf gets, and nothing is scheduled.
func TestAPIFolderTransferReadOnlyTargetContract(t *testing.T) {
	env := newAPITestEnv(t, withReadOnlySecondShelf(t.TempDir()))
	importTextBook(t, env, "No Room", "fiction", "noroom.txt", "body")

	rec := env.post(folderTransfersURL(), strings.NewReader(`{
		"mode": "copy",
		"source_folder": ["fiction"],
		"target_shelf": "`+secondShelfID+`",
		"target_folder": ["imported"]
	}`))
	assertStatus(t, rec, http.StatusConflict)
	if strings.Contains(rec.Body.String(), "taskchain_id") {
		t.Errorf("body = %s, want a refusal rather than a queued chain", rec.Body.String())
	}
}

// The endpoint rejects malformed requests before scheduling anything.
func TestAPIFolderTransferBadRequestContract(t *testing.T) {
	env := newAPITestEnv(t, withSecondShelf(t.TempDir()))
	importTextBook(t, env, "Present", "fiction", "present.txt", "body")

	cases := []struct {
		name string
		body string
		want int
	}{
		{"missing target shelf", `{"mode":"copy","source_folder":["fiction"],"target_shelf":"nope"}`, http.StatusNotFound},
		{"bad mode", `{"mode":"duplicate","source_folder":["fiction"],"target_shelf":"` + secondShelfID + `"}`, http.StatusBadRequest},
		{"missing source folder", `{"mode":"copy","target_shelf":"` + secondShelfID + `"}`, http.StatusBadRequest},
		{"same shelf", `{"mode":"copy","source_folder":["fiction"],"target_shelf":"` + defaultShelfID + `"}`, http.StatusBadRequest},
		{"missing source folder on shelf", `{"mode":"copy","source_folder":["ghost"],"target_shelf":"` + secondShelfID + `"}`, http.StatusNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assertStatus(t, env.post(folderTransfersURL(), strings.NewReader(tc.body)), tc.want)
		})
	}
}
