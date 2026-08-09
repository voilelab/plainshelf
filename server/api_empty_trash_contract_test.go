package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/voilelab/plainshelf/internal/taskutil"
)

// gateTask occupies the worker until it is released.
type gateTask struct {
	started chan struct{}
	release chan struct{}
	once    sync.Once
}

func (t *gateTask) Run(ctx context.Context) error {
	t.once.Do(func() { close(t.started) })
	select {
	case <-t.release:
	case <-ctx.Done():
	}
	return nil
}

func (t *gateTask) Name() string            { return "gate" }
func (t *gateTask) Title() string           { return "gate" }
func (t *gateTask) Description() string     { return "gate" }
func (t *gateTask) Percentage() float64     { return 0 }
func (t *gateTask) Status() taskutil.Status { return taskutil.StatusRunning }

// blockWorker occupies the single worker goroutine so that chains submitted
// afterwards stay queued. The returned function releases it.
func blockWorker(t *testing.T, env *apiTestEnv) func() {
	t.Helper()

	gate := &gateTask{started: make(chan struct{}), release: make(chan struct{})}
	if _, err := env.app.taskChains.Submit(&taskutil.TaskChain{
		Name:  "gate_chain",
		Tasks: []taskutil.Task{gate},
	}); err != nil {
		t.Fatalf("Submit gate chain: %v", err)
	}

	<-gate.started

	var once sync.Once
	release := func() { once.Do(func() { close(gate.release) }) }
	t.Cleanup(release)
	return release
}

// waitForTaskChain polls the task chain endpoint until the chain reaches a
// terminal status, mirroring what the trash page does.
func waitForTaskChain(t *testing.T, env *apiTestEnv, taskChainID string) TaskChain {
	t.Helper()

	deadline := time.Now().Add(10 * time.Second)
	for {
		rec := env.do(httptest.NewRequest(http.MethodGet, "/api/taskchains/"+taskChainID, nil))
		assertStatus(t, rec, http.StatusOK)
		chain := decodeJSON[TaskChain](t, rec)

		switch chain.Status {
		case "completed", "partially_completed", "failed":
			return chain
		}

		if time.Now().After(deadline) {
			t.Fatalf("task chain %s did not finish, last status %q", taskChainID, chain.Status)
		}
		time.Sleep(5 * time.Millisecond)
	}
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
	if chain.Name != emptyTrashTaskName {
		t.Errorf("name = %q, want %q", chain.Name, emptyTrashTaskName)
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
	env.app.conf.ReadOnly = true

	rec := env.do(httptest.NewRequest(http.MethodPost, "/api/shelves/default_shelf/trash/empty", nil))
	assertStatus(t, rec, http.StatusForbidden)
}
