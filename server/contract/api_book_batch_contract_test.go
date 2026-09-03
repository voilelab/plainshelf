package contract_test

import (
	"encoding/json/v2"
	"net/http"
	"testing"
)

func bookBatchURL() string {
	return shelfURL("book-batches")
}

func submitBookBatch(t *testing.T, env *apiTestEnv, payload any, wantStatus int) taskChainSubmitResponse {
	t.Helper()

	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal batch request: %v", err)
	}
	return submitTaskChain(t, env, bookBatchURL(), body, wantStatus)
}

func TestAPIBookBatchMoveContract(t *testing.T) {
	env := newAPITestEnv(t)
	first := importTextBook(t, env, "First", "source", "first.txt", "one")
	second := importTextBook(t, env, "Second", "source", "second.txt", "two")

	accepted := submitBookBatch(t, env, map[string]any{
		"operation":     "move",
		"book_ids":      []string{first.Meta.ID, second.Meta.ID, first.Meta.ID},
		"target_folder": []string{"target"},
	}, http.StatusAccepted)
	chain := waitForTaskChain(t, env, accepted.TaskChainID)
	if chain.Status != "completed" || chain.Percentage != 100 {
		t.Fatalf("chain = %+v, want completed at 100%%", chain)
	}

	result := taskResult[map[string]any](t, chain)
	if result["operation"] != "move" || result["total"] != float64(2) {
		t.Errorf("result = %#v, want move total 2", result)
	}
	succeeded, ok := result["succeeded_ids"].([]any)
	if !ok || len(succeeded) != 2 {
		t.Fatalf("succeeded_ids = %#v, want two ids", result["succeeded_ids"])
	}
	failures, ok := result["failures"].([]any)
	if !ok || len(failures) != 0 {
		t.Fatalf("failures = %#v, want an empty array", result["failures"])
	}

	for _, id := range []string{first.Meta.ID, second.Meta.ID} {
		shelfData, exists := env.app.ShelfManager().GetShelf(defaultShelfID)
		if !exists {
			t.Fatal("default shelf disappeared")
		}
		listing, err := shelfData.GetBookListing(id)
		if err != nil {
			t.Fatalf("GetBookListing(%s): %v", id, err)
		}
		if !listing.Folders.Equal([]string{"target"}) {
			t.Errorf("book %s folders = %v, want target", id, listing.Folders)
		}
	}
}

func TestAPIBookBatchPartialFailureContract(t *testing.T) {
	env := newAPITestEnv(t)
	book := importTextBook(t, env, "Keep", "", "keep.txt", "body")

	accepted := submitBookBatch(t, env, map[string]any{
		"operation": "trash",
		"book_ids":  []string{book.Meta.ID, "missing-book"},
	}, http.StatusAccepted)
	chain := waitForTaskChain(t, env, accepted.TaskChainID)
	if chain.Status != "partially_completed" || chain.Percentage != 100 {
		t.Fatalf("chain = %+v, want partially_completed at 100%%", chain)
	}

	result := taskResult[map[string]any](t, chain)
	failures, ok := result["failures"].([]any)
	if !ok || len(failures) != 1 {
		t.Fatalf("failures = %#v, want one", result["failures"])
	}
	failure, ok := failures[0].(map[string]any)
	if !ok {
		t.Fatalf("failure = %#v, want an object", failures[0])
	}
	if failure["book_id"] != "missing-book" || failure["code"] != "not_found" {
		t.Errorf("failure = %#v, want missing-book/not_found", failure)
	}
}

func TestAPIBookBatchValidationContract(t *testing.T) {
	env := newAPITestEnv(t)

	tooMany := make([]string, 200+1)
	for i := range tooMany {
		tooMany[i] = string(rune(i + 1))
	}

	cases := []struct {
		name    string
		payload any
	}{
		{"empty", map[string]any{"operation": "trash", "book_ids": []string{}}},
		{"unknown operation", map[string]any{"operation": "archive", "book_ids": []string{"book"}}},
		{"move without target", map[string]any{"operation": "move", "book_ids": []string{"book"}}},
		{"trash with target", map[string]any{"operation": "trash", "book_ids": []string{"book"}, "target_folder": []string{}}},
		{"invalid target", map[string]any{"operation": "move", "book_ids": []string{"book"}, "target_folder": []string{".."}}},
		{"too many", map[string]any{"operation": "trash", "book_ids": tooMany}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			submitBookBatch(t, env, tc.payload, http.StatusBadRequest)
		})
	}
}

func TestAPIBookBatchDuplicateRunningChainContract(t *testing.T) {
	env := newAPITestEnv(t)
	book := importTextBook(t, env, "Queued", "", "queued.txt", "body")

	payload := map[string]any{"operation": "trash", "book_ids": []string{book.Meta.ID}}
	assertDuplicateChainConflict(t, env, func(wantStatus int) taskChainSubmitResponse {
		return submitBookBatch(t, env, payload, wantStatus)
	})
}
