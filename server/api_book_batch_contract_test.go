package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
)

func submitBookBatch(t *testing.T, env *apiTestEnv, payload any, wantStatus int) taskChainSubmitResponse {
	t.Helper()
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal batch request: %v", err)
	}
	rec := env.do(httptest.NewRequest(http.MethodPost, "/api/shelves/default_shelf/book-batches", bytes.NewReader(body)))
	assertStatus(t, rec, wantStatus)
	if wantStatus != http.StatusAccepted && wantStatus != http.StatusConflict {
		return taskChainSubmitResponse{}
	}
	assertJSONContentType(t, rec)
	return decodeJSON[taskChainSubmitResponse](t, rec)
}

func taskBatchResult(t *testing.T, chain TaskChain) map[string]any {
	t.Helper()
	if len(chain.Tasks) != 1 {
		t.Fatalf("task count = %d, want 1", len(chain.Tasks))
	}
	result, ok := chain.Tasks[0].Result.(bookBatchResult)
	if ok {
		body, _ := json.Marshal(result)
		var generic map[string]any
		_ = json.Unmarshal(body, &generic)
		return generic
	}
	generic, ok := chain.Tasks[0].Result.(map[string]any)
	if !ok {
		t.Fatalf("task result = %#v, want object", chain.Tasks[0].Result)
	}
	return generic
}

func TestAPIBookBatchMoveContract(t *testing.T) {
	env := newAPITestEnv(t)
	first := importTextBook(t, env, "First", "source", "first.txt", "one")
	second := importTextBook(t, env, "Second", "source", "second.txt", "two")

	accepted := submitBookBatch(t, env, map[string]any{
		"operation":    "move",
		"book_ids":     []string{first.Meta.ID, second.Meta.ID, first.Meta.ID},
		"target_layer": []string{"target"},
	}, http.StatusAccepted)
	chain := waitForTaskChain(t, env, accepted.TaskChainID)
	if chain.Status != "completed" || chain.Percentage != 100 {
		t.Fatalf("chain = %+v, want completed at 100%%", chain)
	}

	result := taskBatchResult(t, chain)
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
		shelfData, exists := env.app.shelfManager.GetShelf("default_shelf")
		if !exists {
			t.Fatal("default shelf disappeared")
		}
		book, err := shelfData.GetBook(id)
		if err != nil {
			t.Fatalf("GetBook(%s): %v", id, err)
		}
		if !book.Layers().Equal([]string{"target"}) {
			t.Errorf("book %s layers = %v, want target", id, book.Layers())
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

	result := taskBatchResult(t, chain)
	failures, ok := result["failures"].([]any)
	if !ok || len(failures) != 1 {
		t.Fatalf("failures = %#v, want one", result["failures"])
	}
	failure := failures[0].(map[string]any)
	if failure["book_id"] != "missing-book" || failure["code"] != "not_found" {
		t.Errorf("failure = %#v, want missing-book/not_found", failure)
	}
}

func TestAPIBookBatchValidationContract(t *testing.T) {
	env := newAPITestEnv(t)

	cases := []struct {
		name    string
		payload any
	}{
		{"empty", map[string]any{"operation": "trash", "book_ids": []string{}}},
		{"unknown operation", map[string]any{"operation": "archive", "book_ids": []string{"book"}}},
		{"move without target", map[string]any{"operation": "move", "book_ids": []string{"book"}}},
		{"trash with target", map[string]any{"operation": "trash", "book_ids": []string{"book"}, "target_layer": []string{}}},
		{"invalid target", map[string]any{"operation": "move", "book_ids": []string{"book"}, "target_layer": []string{".."}}},
	}
	tooMany := make([]string, maxBookBatchSize+1)
	for i := range tooMany {
		tooMany[i] = string(rune(i + 1))
	}
	cases = append(cases, struct {
		name    string
		payload any
	}{"too many", map[string]any{"operation": "trash", "book_ids": tooMany}})

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			submitBookBatch(t, env, tc.payload, http.StatusBadRequest)
		})
	}
}

func TestAPIBookBatchDuplicateRunningChainContract(t *testing.T) {
	env := newAPITestEnv(t)
	book := importTextBook(t, env, "Queued", "", "queued.txt", "body")
	release := blockWorker(t, env)
	payload := map[string]any{"operation": "trash", "book_ids": []string{book.Meta.ID}}
	first := submitBookBatch(t, env, payload, http.StatusAccepted)
	duplicate := submitBookBatch(t, env, payload, http.StatusConflict)
	if duplicate.TaskChainID != first.TaskChainID {
		t.Errorf("duplicate id = %q, want %q", duplicate.TaskChainID, first.TaskChainID)
	}
	release()
	waitForTaskChain(t, env, first.TaskChainID)
}

func TestAPIBookBatchRequiresTokenAndWritableModeContract(t *testing.T) {
	env := newAPITestEnv(t)
	body := []byte(`{"operation":"trash","book_ids":["book"]}`)

	rec := env.doRaw(httptest.NewRequest(http.MethodPost, "/api/shelves/default_shelf/book-batches", bytes.NewReader(body)))
	assertStatus(t, rec, http.StatusUnauthorized)

	env.app.conf.ReadOnly = true
	rec = env.do(httptest.NewRequest(http.MethodPost, "/api/shelves/default_shelf/book-batches", bytes.NewReader(body)))
	assertStatus(t, rec, http.StatusForbidden)
}

func TestNormalizeBookBatchIDsPreservesFirstOccurrence(t *testing.T) {
	got, err := normalizeBookBatchIDs([]string{" b ", "a", "b"})
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}
	if !slices.Equal(got, []string{"b", "a"}) {
		t.Errorf("got %v, want [b a]", got)
	}
}
