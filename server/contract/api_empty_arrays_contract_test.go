package contract_test

import (
	"net/http"
	"testing"
)

// The API answers an empty array-valued field with [] and never with null.
//
// Before json/v2 this held only where a handler remembered to build an empty
// slice instead of leaving it nil, so every client carried a `?? []` guard for
// the fields where it did not. The encoder now decides it for all of them, which
// is only worth relying on if the wire bytes are pinned - hence these assertions
// sit on the response body rather than on a decoded Go value, whose nil and
// empty slices are indistinguishable once unmarshalled.

// chainBody polls a chain to a terminal status and returns the raw body of the
// final poll, which is what a client parses.
func chainBody(t *testing.T, env *apiTestEnv, taskChainID string) []byte {
	t.Helper()

	waitForTaskChain(t, env, taskChainID)

	rec := env.get(taskChainURL(taskChainID))
	assertStatus(t, rec, http.StatusOK)
	return rec.Body.Bytes()
}

// A batch whose books all succeed leaves failures empty; one whose books all
// fail leaves succeeded_ids empty. Both fields are asserted on both chains, so
// each is covered in its empty case.
func TestAPIBookBatchEmptyArraysAreNotNullContract(t *testing.T) {
	env := newAPITestEnv(t)
	book := importTextBook(t, env, "Kept", "", "kept.txt", "body")

	succeeding := submitBookBatch(t, env, map[string]any{
		"operation": "trash",
		"book_ids":  []string{book.Meta.ID},
	}, http.StatusAccepted)
	failing := submitBookBatch(t, env, map[string]any{
		"operation": "trash",
		"book_ids":  []string{"missing-book"},
	}, http.StatusAccepted)

	for name, chainID := range map[string]string{
		"every book succeeded": succeeding.TaskChainID,
		"every book failed":    failing.TaskChainID,
	} {
		t.Run(name, func(t *testing.T) {
			body := chainBody(t, env, chainID)
			assertJSONArray(t, body, "tasks")
			assertJSONArray(t, body, "tasks", "0", "result", "succeeded_ids")
			assertJSONArray(t, body, "tasks", "0", "result", "failures")
		})
	}
}

// A folder transfer that moves every book reports no failures.
func TestAPIFolderTransferEmptyArraysAreNotNullContract(t *testing.T) {
	env := newAPITestEnv(t, withSecondShelf(t.TempDir()))
	importTextBook(t, env, "Top", "fiction", "top.txt", "a")

	accepted := submitTaskChain(t, env, folderTransfersURL(), []byte(`{
		"mode": "move",
		"source_folder": ["fiction"],
		"target_shelf": "`+secondShelfID+`"
	}`), http.StatusAccepted)

	body := chainBody(t, env, accepted.TaskChainID)
	assertJSONArray(t, body, "tasks")
	assertJSONArray(t, body, "tasks", "0", "result", "succeeded_ids")
	assertJSONArray(t, body, "tasks", "0", "result", "failures")
}

// The two shelf-wide sweeps report no failures on a shelf holding one healthy
// book, which is the empty case for the only array either result carries.
func TestAPISweepEmptyFailuresAreNotNullContract(t *testing.T) {
	env := newAPITestEnv(t)
	importTextBook(t, env, "Healthy", "", "healthy.txt", "body")

	for name, url := range map[string]string{
		"content stat refresh": contentStatsURL(),
		"source fingerprints":  sourceFingerprintsURL(),
	} {
		t.Run(name, func(t *testing.T) {
			accepted := submitTaskChain(t, env, url, nil, http.StatusAccepted)
			body := chainBody(t, env, accepted.TaskChainID)
			assertJSONArray(t, body, "tasks")
			assertJSONArray(t, body, "tasks", "0", "result", "failures")
		})
	}
}

// A book filed at the shelf root has no folder, and an imported text file names
// no authors. Both reach the client as [].
func TestAPIBookEmptyArraysAreNotNullContract(t *testing.T) {
	env := newAPITestEnv(t)
	importTextBook(t, env, "Rootless", "", "rootless.txt", "body")

	rec := env.get(booksURL())
	assertStatus(t, rec, http.StatusOK)

	body := rec.Body.Bytes()
	assertJSONArray(t, body, "0", "folder")
	assertJSONArray(t, body, "0", "meta", "authors")
}

// An empty shelf reports one folder - the root, whose path has no segments. The
// nesting makes it the sharpest case in this file: the outer list is non-empty,
// so only the inner empty path can regress to null.
func TestAPIFoldersOfEmptyShelfAreNotNullContract(t *testing.T) {
	env := newAPITestEnv(t)

	rec := env.get(shelfURL("folders"))
	assertStatus(t, rec, http.StatusOK)

	body := rec.Body.Bytes()
	assertJSONArray(t, body, "0")
	if got := rec.Body.String(); got != "[[]]\n" {
		t.Fatalf("folders body = %q, want %q", got, "[[]]\n")
	}
}
