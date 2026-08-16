package contract_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/voilelab/plainshelf/server"
	"github.com/voilelab/plainshelf/server/task"
)

func TestAPITrashLifecycleContract(t *testing.T) {
	env := newAPITestEnv(t)
	created := importTextBook(t, env, "Trash API", "origin/layer", "trash.txt", "body")

	rec := env.do(httptest.NewRequest(http.MethodPost, "/api/shelves/default_shelf/books/"+created.Meta.ID+"/trash", nil))
	assertStatus(t, rec, http.StatusNoContent)

	rec = env.do(httptest.NewRequest(http.MethodGet, "/api/shelves/default_shelf/books", nil))
	assertStatus(t, rec, http.StatusOK)
	if books := decodeJSON[[]server.Book](t, rec); len(books) != 0 {
		t.Fatalf("active books after trash = %d, want 0", len(books))
	}

	rec = env.do(httptest.NewRequest(http.MethodGet, "/api/shelves/default_shelf/trash/books", nil))
	assertStatus(t, rec, http.StatusOK)
	trashed := decodeJSON[[]map[string]any](t, rec)
	if len(trashed) != 1 {
		t.Fatalf("trashed books = %d, want 1", len(trashed))
	}
	if id, _ := trashed[0]["id"].(string); id != created.Meta.ID {
		t.Fatalf("trashed id = %q, want %q", id, created.Meta.ID)
	}

	rec = env.do(httptest.NewRequest(http.MethodPost, "/api/shelves/default_shelf/trash/books/"+created.Meta.ID+"/restore", nil))
	assertStatus(t, rec, http.StatusNoContent)

	rec = env.do(httptest.NewRequest(http.MethodGet, "/api/shelves/default_shelf/books", nil))
	assertStatus(t, rec, http.StatusOK)
	if books := decodeJSON[[]server.Book](t, rec); len(books) != 1 {
		t.Fatalf("active books after restore = %d, want 1", len(books))
	}

	rec = env.do(httptest.NewRequest(http.MethodDelete, "/api/shelves/default_shelf/books/"+created.Meta.ID, nil))
	assertStatus(t, rec, http.StatusNoContent)
	rec = env.do(httptest.NewRequest(http.MethodDelete, "/api/shelves/default_shelf/trash/books/"+created.Meta.ID, nil))
	assertStatus(t, rec, http.StatusNoContent)
	rec = env.do(httptest.NewRequest(http.MethodPost, "/api/shelves/default_shelf/trash/books/"+created.Meta.ID+"/restore", nil))
	assertStatus(t, rec, http.StatusNotFound)
}

func emptyTrash(t *testing.T, env *apiTestEnv, wantStatus int) taskChainSubmitResponse {
	t.Helper()

	rec := env.do(httptest.NewRequest(http.MethodPost, "/api/shelves/default_shelf/trash/empty", nil))
	assertStatus(t, rec, wantStatus)
	assertJSONContentType(t, rec)

	resp := decodeJSON[taskChainSubmitResponse](t, rec)
	if resp.TaskChainID == "" {
		t.Fatalf("response is missing taskchain_id: %s", rec.Body.String())
	}
	return resp
}

func TestAPIEmptyTrashContract(t *testing.T) {
	env := newAPITestEnv(t)

	for _, title := range []string{"First", "Second"} {
		created := importTextBook(t, env, title, "", title+".txt", "body")
		rec := env.do(httptest.NewRequest(http.MethodPost, "/api/shelves/default_shelf/books/"+created.Meta.ID+"/trash", nil))
		assertStatus(t, rec, http.StatusNoContent)
	}

	accepted := emptyTrash(t, env, http.StatusAccepted)

	chain := waitForTaskChain(t, env, accepted.TaskChainID)
	if chain.Status != "completed" {
		t.Fatalf("chain status = %q, want completed: %+v", chain.Status, chain)
	}
	if chain.Percentage != 100 {
		t.Errorf("percentage = %v, want 100", chain.Percentage)
	}
	if chain.Name != task.EmptyTrashTaskName {
		t.Errorf("name = %q, want %q", chain.Name, task.EmptyTrashTaskName)
	}

	rec := env.do(httptest.NewRequest(http.MethodGet, "/api/shelves/default_shelf/trash/books", nil))
	assertStatus(t, rec, http.StatusOK)
	if trashed := decodeJSON[[]map[string]any](t, rec); len(trashed) != 0 {
		t.Errorf("trashed books after empty = %d, want 0", len(trashed))
	}
}

func TestAPIEmptyTrashOnEmptyTrashContract(t *testing.T) {
	env := newAPITestEnv(t)

	accepted := emptyTrash(t, env, http.StatusAccepted)

	chain := waitForTaskChain(t, env, accepted.TaskChainID)
	if chain.Status != "completed" {
		t.Errorf("chain status = %q, want completed for an already empty trash", chain.Status)
	}
	if chain.Percentage != 100 {
		t.Errorf("percentage = %v, want 100", chain.Percentage)
	}
}

// A second request while a sweep is still in flight must point the client at
// the existing chain instead of queueing a redundant one.
func TestAPIEmptyTrashConflictReportsRunningChainContract(t *testing.T) {
	env := newAPITestEnv(t)

	// Block the worker so the empty-trash chain stays queued and therefore
	// non-terminal for the duration of the test.
	release := blockWorker(t, env)

	first := emptyTrash(t, env, http.StatusAccepted)
	conflict := emptyTrash(t, env, http.StatusConflict)

	if conflict.TaskChainID != first.TaskChainID {
		t.Errorf("conflict taskchain_id = %q, want the running chain %q", conflict.TaskChainID, first.TaskChainID)
	}

	release()
	waitForTaskChain(t, env, first.TaskChainID)

	// Once the sweep has finished, a fresh request is accepted again.
	next := emptyTrash(t, env, http.StatusAccepted)
	if next.TaskChainID == first.TaskChainID {
		t.Errorf("expected a new chain after the previous one finished")
	}
}

func TestAPIEmptyTrashRejectsUnknownShelfContract(t *testing.T) {
	env := newAPITestEnv(t)

	rec := env.do(httptest.NewRequest(http.MethodPost, "/api/shelves/missing_shelf/trash/empty", nil))
	assertStatus(t, rec, http.StatusNotFound)
}

// The endpoint mutates the shelf, so it must sit inside the local_token
// boundary and be refused in read-only mode.
func TestAPIEmptyTrashRequiresTokenContract(t *testing.T) {
	env := newAPITestEnv(t)

	rec := env.doRaw(httptest.NewRequest(http.MethodPost, "/api/shelves/default_shelf/trash/empty", nil))
	assertStatus(t, rec, http.StatusUnauthorized)
}

func TestAPIEmptyTrashRejectedInReadOnlyModeContract(t *testing.T) {
	env := newAPITestEnv(t)
	env.app.Conf().ReadOnly = true

	rec := env.do(httptest.NewRequest(http.MethodPost, "/api/shelves/default_shelf/trash/empty", nil))
	assertStatus(t, rec, http.StatusForbidden)
}
