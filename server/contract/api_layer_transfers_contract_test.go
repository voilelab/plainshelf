package contract_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/voilelab/plainshelf/server"
	"github.com/voilelab/plainshelf/shelf"
)

// layerTransfersURL is the cross-shelf layer transfer endpoint on the default
// (source) shelf. secondShelfID and its helpers come from the book-transfer
// contract test in this same package.
func layerTransfersURL() string {
	return shelfURL("layer-transfers")
}

// layerTransferConflictBody mirrors the 409 body a pre-flight refusal answers
// with, so a test can tell the two conflict kinds apart and read the colliding
// IDs. Restating it here pins the wire shape the frontend relies on.
type layerTransferConflictBody struct {
	Error              string   `json:"error"`
	Message            string   `json:"message"`
	ConflictingBookIDs []string `json:"conflicting_book_ids"`
}

// booksUnderLayer returns the IDs of the shelf's books that sit exactly under
// layer, so a test can assert the set that landed there.
func booksUnderLayer(books []server.Book, layer shelf.FolderPath) []string {
	var ids []string
	for _, book := range books {
		if book.Layer.Equal(layer) {
			ids = append(ids, book.Meta.ID)
		}
	}
	return ids
}

// A copy transfer reproduces the source subtree on the target with fresh IDs and
// leaves the source shelf untouched.
func TestAPILayerTransferCopyContract(t *testing.T) {
	env := newAPITestEnv(t, withSecondShelf(t.TempDir()))
	top := importTextBook(t, env, "Top", "fiction", "top.txt", "a")
	deep := importTextBook(t, env, "Deep", "fiction/sci-fi", "deep.txt", "b")

	accepted := submitTaskChain(t, env, layerTransfersURL(), []byte(`{
		"mode": "copy",
		"source_layer": ["fiction"],
		"target_shelf": "`+secondShelfID+`",
		"target_layer": ["imported"]
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

	// The target holds fresh-ID copies under the remapped layers.
	targetBooks := getJSON[[]server.Book](t, env, secondShelfBooksURL())
	topCopies := booksUnderLayer(targetBooks, shelf.FolderPath{"imported"})
	deepCopies := booksUnderLayer(targetBooks, shelf.FolderPath{"imported", "sci-fi"})
	if len(topCopies) != 1 || topCopies[0] == top.Meta.ID {
		t.Errorf("target [imported] = %v, want one fresh-ID copy of %q", topCopies, top.Meta.ID)
	}
	if len(deepCopies) != 1 || deepCopies[0] == deep.Meta.ID {
		t.Errorf("target [imported sci-fi] = %v, want one fresh-ID copy of %q", deepCopies, deep.Meta.ID)
	}
}

// A move transfer keeps every book's ID, publishes the subtree on the target, and
// clears the source layer. With no target_layer it lands under the source layer's
// own name.
func TestAPILayerTransferMoveContract(t *testing.T) {
	env := newAPITestEnv(t, withSecondShelf(t.TempDir()))
	top := importTextBook(t, env, "Top", "fiction", "top.txt", "a")
	deep := importTextBook(t, env, "Deep", "fiction/sci-fi", "deep.txt", "b")

	accepted := submitTaskChain(t, env, layerTransfersURL(), []byte(`{
		"mode": "move",
		"source_layer": ["fiction"],
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
	// layer name.
	targetBooks := getJSON[[]server.Book](t, env, secondShelfBooksURL())
	if got := booksUnderLayer(targetBooks, shelf.FolderPath{"fiction"}); len(got) != 1 || got[0] != top.Meta.ID {
		t.Errorf("target [fiction] = %v, want [%s]", got, top.Meta.ID)
	}
	if got := booksUnderLayer(targetBooks, shelf.FolderPath{"fiction", "sci-fi"}); len(got) != 1 || got[0] != deep.Meta.ID {
		t.Errorf("target [fiction sci-fi] = %v, want [%s]", got, deep.Meta.ID)
	}
}

// A transfer onto a layer the target shelf already holds is refused with a 409
// that names the conflict, before any work is scheduled.
func TestAPILayerTransferLayerConflictContract(t *testing.T) {
	env := newAPITestEnv(t, withSecondShelf(t.TempDir()))
	importTextBook(t, env, "Source", "fiction", "src.txt", "a")
	// The target already holds the destination layer, so the transfer must refuse
	// to land on it rather than merge into it.
	assertStatus(t, env.post(shelfIDURL(secondShelfID, "layers", "taken"), nil), http.StatusNoContent)

	rec := env.post(layerTransfersURL(), strings.NewReader(`{
		"mode": "copy",
		"source_layer": ["fiction"],
		"target_shelf": "`+secondShelfID+`",
		"target_layer": ["taken"]
	}`))
	assertStatus(t, rec, http.StatusConflict)
	if strings.Contains(rec.Body.String(), "taskchain_id") {
		t.Errorf("body = %s, want a refusal rather than a queued chain", rec.Body.String())
	}
	body := decodeJSON[layerTransferConflictBody](t, rec)
	if body.Error != "target_layer_conflict" {
		t.Errorf("error = %q, want target_layer_conflict", body.Error)
	}
}

// A move whose books collide with IDs the target already holds is refused with a
// 409 that lists every colliding ID, before any work is scheduled.
func TestAPILayerTransferBookIDConflictContract(t *testing.T) {
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

	rec := env.post(layerTransfersURL(), strings.NewReader(`{
		"mode": "move",
		"source_layer": ["fiction"],
		"target_shelf": "`+secondShelfID+`",
		"target_layer": ["elsewhere"]
	}`))
	assertStatus(t, rec, http.StatusConflict)
	if strings.Contains(rec.Body.String(), "taskchain_id") {
		t.Errorf("body = %s, want a refusal rather than a queued chain", rec.Body.String())
	}
	body := decodeJSON[layerTransferConflictBody](t, rec)
	if body.Error != "book_id_conflict" {
		t.Errorf("error = %q, want book_id_conflict", body.Error)
	}
	if len(body.ConflictingBookIDs) != 1 || body.ConflictingBookIDs[0] != id {
		t.Errorf("conflicting_book_ids = %v, want [%s]", body.ConflictingBookIDs, id)
	}
}

// Re-submitting a transfer already in flight is a 409 that points at the running
// chain, so the client can attach to it instead of starting a second.
func TestAPILayerTransferDuplicateRunningChainContract(t *testing.T) {
	env := newAPITestEnv(t, withSecondShelf(t.TempDir()))
	importTextBook(t, env, "Queued", "fiction", "queued.txt", "body")
	release := blockWorker(t, env)

	body := []byte(`{
		"mode": "copy",
		"source_layer": ["fiction"],
		"target_shelf": "` + secondShelfID + `",
		"target_layer": ["imported"]
	}`)
	first := submitTaskChain(t, env, layerTransfersURL(), body, http.StatusAccepted)
	duplicate := submitTaskChain(t, env, layerTransfersURL(), body, http.StatusConflict)
	if duplicate.TaskChainID != first.TaskChainID {
		t.Errorf("duplicate id = %q, want %q", duplicate.TaskChainID, first.TaskChainID)
	}
	release()
	waitForTaskChain(t, env, first.TaskChainID)
}

// A transfer into a read-only target shelf is refused with 409, the status every
// write to a read-only shelf gets, and nothing is scheduled.
func TestAPILayerTransferReadOnlyTargetContract(t *testing.T) {
	env := newAPITestEnv(t, withReadOnlySecondShelf(t.TempDir()))
	importTextBook(t, env, "No Room", "fiction", "noroom.txt", "body")

	rec := env.post(layerTransfersURL(), strings.NewReader(`{
		"mode": "copy",
		"source_layer": ["fiction"],
		"target_shelf": "`+secondShelfID+`",
		"target_layer": ["imported"]
	}`))
	assertStatus(t, rec, http.StatusConflict)
	if strings.Contains(rec.Body.String(), "taskchain_id") {
		t.Errorf("body = %s, want a refusal rather than a queued chain", rec.Body.String())
	}
}

// The endpoint rejects malformed requests before scheduling anything.
func TestAPILayerTransferBadRequestContract(t *testing.T) {
	env := newAPITestEnv(t, withSecondShelf(t.TempDir()))
	importTextBook(t, env, "Present", "fiction", "present.txt", "body")

	cases := []struct {
		name string
		body string
		want int
	}{
		{"missing target shelf", `{"mode":"copy","source_layer":["fiction"],"target_shelf":"nope"}`, http.StatusNotFound},
		{"bad mode", `{"mode":"duplicate","source_layer":["fiction"],"target_shelf":"` + secondShelfID + `"}`, http.StatusBadRequest},
		{"missing source layer", `{"mode":"copy","target_shelf":"` + secondShelfID + `"}`, http.StatusBadRequest},
		{"same shelf", `{"mode":"copy","source_layer":["fiction"],"target_shelf":"` + defaultShelfID + `"}`, http.StatusBadRequest},
		{"missing source layer on shelf", `{"mode":"copy","source_layer":["ghost"],"target_shelf":"` + secondShelfID + `"}`, http.StatusNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assertStatus(t, env.post(layerTransfersURL(), strings.NewReader(tc.body)), tc.want)
		})
	}
}

// The transfer endpoint stays behind both write gates: the token requirement and
// the read-only refusal.
func TestAPILayerTransferIsGatedContract(t *testing.T) {
	env := newAPITestEnv(t, withSecondShelf(t.TempDir()))
	importTextBook(t, env, "Gated", "fiction", "gated.txt", "body")

	assertMutationGated(t, env, http.MethodPost, layerTransfersURL(),
		[]byte(`{"mode":"copy","source_layer":["fiction"],"target_shelf":"`+secondShelfID+`","target_layer":["imported"]}`))
}
