package books_test

import (
	"bytes"
	"encoding/json/v2"
	"net/http"
	"os"
	"testing"

	"github.com/voilelab/plainshelf/server/contract/apitest"
	"github.com/voilelab/plainshelf/shelf"
)

func submitBookBatch(t *testing.T, env *apitest.Env, payload any, wantStatus int) apitest.TaskChainSubmitResponse {
	t.Helper()

	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal batch request: %v", err)
	}
	return apitest.SubmitTaskChain(t, env, apitest.BookBatchURL(), body, wantStatus)
}

func TestAPIBookBatchMoveContract(t *testing.T) {
	env := apitest.New(t)
	first := apitest.ImportTextBook(t, env, "First", "source", "first.txt", "one")
	second := apitest.ImportTextBook(t, env, "Second", "source", "second.txt", "two")

	accepted := submitBookBatch(t, env, map[string]any{
		"operation":     "move",
		"book_ids":      []string{first.Meta.ID, second.Meta.ID, first.Meta.ID},
		"target_folder": []string{"target"},
	}, http.StatusAccepted)
	chain := apitest.WaitForTaskChain(t, env, accepted.TaskChainID)
	if chain.Status != "completed" || chain.Percentage != 100 {
		t.Fatalf("chain = %+v, want completed at 100%%", chain)
	}

	result := apitest.TaskResult[map[string]any](t, chain)
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
		shelfData, exists := env.App.ShelfManager().GetShelf(apitest.DefaultShelfID)
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
	env := apitest.New(t)
	book := apitest.ImportTextBook(t, env, "Keep", "", "keep.txt", "body")

	accepted := submitBookBatch(t, env, map[string]any{
		"operation": "trash",
		"book_ids":  []string{book.Meta.ID, "missing-book"},
	}, http.StatusAccepted)
	chain := apitest.WaitForTaskChain(t, env, accepted.TaskChainID)
	if chain.Status != "partially_completed" || chain.Percentage != 100 {
		t.Fatalf("chain = %+v, want partially_completed at 100%%", chain)
	}

	result := apitest.TaskResult[map[string]any](t, chain)
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
	env := apitest.New(t)

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

func TestAPIBookBatchDuplicateChain(t *testing.T) {
	env := apitest.New(t)
	book := apitest.ImportTextBook(t, env, "Queued", "", "queued.txt", "body")

	payload := map[string]any{"operation": "trash", "book_ids": []string{book.Meta.ID}}
	apitest.AssertDuplicateChainConflict(t, env, func(wantStatus int) apitest.TaskChainSubmitResponse {
		return submitBookBatch(t, env, payload, wantStatus)
	})
}

func waitBookBatch(t *testing.T, env *apitest.Env, payload any, wantStatus string) map[string]any {
	t.Helper()

	accepted := submitBookBatch(t, env, payload, http.StatusAccepted)
	chain := apitest.WaitForTaskChain(t, env, accepted.TaskChainID)
	if chain.Status != wantStatus || chain.Percentage != 100 {
		t.Fatalf("chain = %+v, want %s at 100%%", chain, wantStatus)
	}
	return apitest.TaskResult[map[string]any](t, chain)
}

func TestAPIBookBatchSetSeriesContract(t *testing.T) {
	env := apitest.New(t)
	first := apitest.ImportTextBook(t, env, "Saga 1", "", "s1.txt", "one").Meta.ID
	second := apitest.ImportTextBook(t, env, "Saga 2", "", "s2.txt", "two").Meta.ID
	tenth := apitest.ImportTextBook(t, env, "Saga 10", "", "s10.txt", "ten").Meta.ID

	result := waitBookBatch(t, env, map[string]any{
		"operation":   "set_series",
		"book_ids":    []string{first, second, tenth},
		"series_name": "  Saga ",
		"start_index": 1,
	}, "completed")
	if result["operation"] != "set_series" || result["total"] != float64(3) {
		t.Errorf("result = %#v, want set_series total 3", result)
	}

	for id, index := range map[string]float64{first: 1, second: 2, tenth: 3} {
		assertSeries(t, id, listedSeries(t, env, id), &shelf.BookSeries{Name: "Saga", Index: index})
	}
}

func TestAPIBookBatchClearSeriesContract(t *testing.T) {
	env := apitest.New(t)
	first := apitest.ImportTextBook(t, env, "Saga 1", "", "s1.txt", "one").Meta.ID
	second := apitest.ImportTextBook(t, env, "Saga 2", "", "s2.txt", "two").Meta.ID
	patchBookOK(t, env, first, `{"series":{"name":"Saga","index":1}}`)

	waitBookBatch(t, env, map[string]any{
		"operation": "set_series",
		"book_ids":  []string{first, second},
		"clear":     true,
	}, "completed")

	for _, id := range []string{first, second} {
		assertSeries(t, id, listedSeries(t, env, id), nil)
		written, err := os.ReadFile(env.BookMetaPath(t, id))
		if err != nil {
			t.Fatalf("read book.json: %v", err)
		}
		if bytes.Contains(written, []byte(`"series"`)) {
			t.Errorf("book %s book.json still carries series:\n%s", id, written)
		}
	}
}

func TestAPIBookBatchSetSeriesRejectsInvalid(t *testing.T) {
	env := apitest.New(t)
	bookID := apitest.ImportTextBook(t, env, "Saga", "", "saga.txt", "body").Meta.ID
	patchBookOK(t, env, bookID, `{"series":{"name":"Saga","index":2}}`)
	metaPath := env.BookMetaPath(t, bookID)
	before, err := os.ReadFile(metaPath)
	if err != nil {
		t.Fatalf("read book.json: %v", err)
	}

	ids := []string{bookID}
	cases := []struct {
		name    string
		payload map[string]any
	}{
		{"blank name", map[string]any{"operation": "set_series", "book_ids": ids, "series_name": " ", "start_index": 1}},
		{"missing name", map[string]any{"operation": "set_series", "book_ids": ids, "start_index": 1}},
		{"negative start", map[string]any{"operation": "set_series", "book_ids": ids, "series_name": "X", "start_index": -1}},
		{"missing start", map[string]any{"operation": "set_series", "book_ids": ids, "series_name": "X"}},
		{"clear with name", map[string]any{"operation": "set_series", "book_ids": ids, "clear": true, "series_name": "X"}},
		{"with target", map[string]any{"operation": "set_series", "book_ids": ids, "series_name": "X", "start_index": 1, "target_folder": []string{}}},
		{"trash with series", map[string]any{"operation": "trash", "book_ids": ids, "series_name": "X", "start_index": 1}},
		{"move with clear", map[string]any{"operation": "move", "book_ids": ids, "target_folder": []string{"x"}, "clear": true}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			submitBookBatch(t, env, tc.payload, http.StatusBadRequest)
		})
	}

	after, err := os.ReadFile(metaPath)
	if err != nil {
		t.Fatalf("read book.json: %v", err)
	}
	if !bytes.Equal(before, after) {
		t.Errorf("book.json changed after rejected batches:\nbefore %s\nafter  %s", before, after)
	}
	assertSeries(t, "after rejected batches", listedSeries(t, env, bookID), &shelf.BookSeries{Name: "Saga", Index: 2})
}

func TestAPIBookBatchSetSeriesPartialFailure(t *testing.T) {
	env := apitest.New(t)
	bookID := apitest.ImportTextBook(t, env, "Saga 2", "", "saga.txt", "body").Meta.ID

	result := waitBookBatch(t, env, map[string]any{
		"operation":   "set_series",
		"book_ids":    []string{"missing-book", bookID},
		"series_name": "Saga",
		"start_index": 1,
	}, "partially_completed")

	failures, ok := result["failures"].([]any)
	if !ok || len(failures) != 1 {
		t.Fatalf("failures = %#v, want one", result["failures"])
	}
	failure, ok := failures[0].(map[string]any)
	if !ok || failure["book_id"] != "missing-book" || failure["code"] != "not_found" {
		t.Errorf("failure = %#v, want missing-book/not_found", failures[0])
	}
	assertSeries(t, "written book", listedSeries(t, env, bookID), &shelf.BookSeries{Name: "Saga", Index: 2})
}
