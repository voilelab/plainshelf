package contract_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/voilelab/plainshelf/internal/taskutil"
	"github.com/voilelab/plainshelf/server"
)

// contractTask reports fixed values so the endpoint's output does not depend on
// when the worker happens to run it.
type contractTask struct{}

func (t *contractTask) Run(ctx context.Context) error { return nil }
func (t *contractTask) Name() string                  { return "contract_task" }
func (t *contractTask) Title() string                 { return "Contract task" }
func (t *contractTask) Description() string           { return "deleted 1 of 2 books" }
func (t *contractTask) Percentage() float64           { return 50 }
func (t *contractTask) Status() taskutil.Status       { return taskutil.StatusRunning }

func submitContractChain(t *testing.T, env *apiTestEnv) *taskutil.TaskChain {
	t.Helper()

	chain, err := env.app.TaskChains().Submit(&taskutil.TaskChain{
		Name:        "contract_chain",
		Title:       "Contract chain",
		Description: "a chain used by the API contract test",
		Tasks:       []taskutil.Task{&contractTask{}},
	})
	if err != nil {
		t.Fatalf("Submit task chain: %v", err)
	}
	return chain
}

func TestAPIGetTaskChainContract(t *testing.T) {
	env := newAPITestEnv(t)
	chain := submitContractChain(t, env)

	rec := env.get(taskChainURL(chain.ID))
	assertStatus(t, rec, http.StatusOK)
	assertJSONContentType(t, rec)

	resp := decodeJSON[server.TaskChain](t, rec)
	if resp.ID != chain.ID {
		t.Errorf("id = %q, want %q", resp.ID, chain.ID)
	}
	if resp.Name != "contract_chain" {
		t.Errorf("name = %q, want contract_chain", resp.Name)
	}
	if resp.Title != "Contract chain" {
		t.Errorf("title = %q, want Contract chain", resp.Title)
	}
	if resp.Status != "running" {
		t.Errorf("status = %q, want running", resp.Status)
	}
	if resp.Percentage != 50 {
		t.Errorf("percentage = %v, want 50", resp.Percentage)
	}
	if len(resp.Tasks) != 1 {
		t.Fatalf("tasks length = %d, want 1", len(resp.Tasks))
	}
	if resp.Tasks[0].Name != "contract_task" || resp.Tasks[0].Status != "running" || resp.Tasks[0].Percentage != 50 {
		t.Errorf("unexpected task payload: %+v", resp.Tasks[0])
	}
}

// TestAPIGetTaskChainSchemaContract pins the wire field names. Decoding into the
// Go response struct alone would pass no matter how the JSON tags change.
func TestAPIGetTaskChainSchemaContract(t *testing.T) {
	env := newAPITestEnv(t)
	chain := submitContractChain(t, env)

	payload := getJSON[map[string]any](t, env, taskChainURL(chain.ID))
	for _, key := range []string{"id", "name", "title", "description", "status", "percentage", "created_at", "tasks"} {
		if _, ok := payload[key]; !ok {
			t.Errorf("response is missing the %q field: %v", key, payload)
		}
	}

	// status must be the readable name, not the underlying enum integer.
	if status, ok := payload["status"].(string); !ok || status != "running" {
		t.Errorf("status = %v, want the string \"running\"", payload["status"])
	}

	tasks, ok := payload["tasks"].([]any)
	if !ok || len(tasks) != 1 {
		t.Fatalf("tasks = %v, want a single-element array", payload["tasks"])
	}
	task, ok := tasks[0].(map[string]any)
	if !ok {
		t.Fatalf("task = %v, want an object", tasks[0])
	}
	for _, key := range []string{"name", "title", "description", "status", "percentage"} {
		if _, ok := task[key]; !ok {
			t.Errorf("task is missing the %q field: %v", key, task)
		}
	}
}

func TestAPIGetTaskChainNotFoundContract(t *testing.T) {
	env := newAPITestEnv(t)

	rec := env.get(taskChainURL("does-not-exist"))
	assertStatus(t, rec, http.StatusNotFound)
}

// cancelSignalTask runs until its context is cancelled, then settles failed. It
// signals once it is running so a test can cancel it deterministically without a
// timer, mirroring how a real transfer stops between books once its context ends.
type cancelSignalTask struct {
	started chan struct{}
	once    sync.Once

	mu     sync.Mutex
	status taskutil.Status
}

func (t *cancelSignalTask) Run(ctx context.Context) error {
	t.setStatus(taskutil.StatusRunning)
	t.once.Do(func() { close(t.started) })
	<-ctx.Done()
	t.setStatus(taskutil.StatusFailed)
	return ctx.Err()
}

func (t *cancelSignalTask) setStatus(s taskutil.Status) {
	t.mu.Lock()
	t.status = s
	t.mu.Unlock()
}

func (t *cancelSignalTask) Name() string        { return "cancel_signal" }
func (t *cancelSignalTask) Title() string       { return "cancel signal" }
func (t *cancelSignalTask) Description() string { return "cancel signal" }
func (t *cancelSignalTask) Percentage() float64 { return 0 }
func (t *cancelSignalTask) Status() taskutil.Status {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.status
}

// TestAPICancelTaskChainRequiresTokenContract keeps the cancel route behind the
// same local_token boundary as every other mutating request: without a token it
// is rejected before it can reach a chain.
func TestAPICancelTaskChainRequiresTokenContract(t *testing.T) {
	env := newAPITestEnv(t)

	// doRaw sends the request without the token do() would attach.
	rec := env.doRaw(httptest.NewRequest(http.MethodPost, taskChainCancelURL("any-id"), nil))
	assertStatus(t, rec, http.StatusUnauthorized)
}

// TestAPICancelTaskChainReadOnlyContract pins the decision recorded on the
// ticket: cancel is a mutating method and read-only mode refuses it, the same as
// the transfer that would have started the chain.
func TestAPICancelTaskChainReadOnlyContract(t *testing.T) {
	env := newAPITestEnv(t)
	env.setReadOnly(t, true)

	rec := env.post(taskChainCancelURL("any-id"), nil)
	assertStatus(t, rec, http.StatusForbidden)
}

func TestAPICancelTaskChainStopsRunningChainContract(t *testing.T) {
	env := newAPITestEnv(t)

	task := &cancelSignalTask{started: make(chan struct{})}
	chain, err := env.app.TaskChains().Submit(&taskutil.TaskChain{
		Name:  "cancel_chain",
		Tasks: []taskutil.Task{task},
	})
	if err != nil {
		t.Fatalf("Submit task chain: %v", err)
	}

	<-task.started
	rec := env.post(taskChainCancelURL(chain.ID), nil)
	assertStatus(t, rec, http.StatusOK)
	assertJSONContentType(t, rec)

	resp := decodeJSON[server.TaskChain](t, rec)
	if resp.ID != chain.ID {
		t.Errorf("id = %q, want %q", resp.ID, chain.ID)
	}

	// The chain settles into a terminal status once the cancel reaches the task.
	settled := waitForTaskChain(t, env, chain.ID)
	if settled.Status != "failed" {
		t.Errorf("status after cancel = %q, want failed", settled.Status)
	}
}

func TestAPICancelTaskChainNotFoundContract(t *testing.T) {
	env := newAPITestEnv(t)

	rec := env.post(taskChainCancelURL("does-not-exist"), nil)
	assertStatus(t, rec, http.StatusNotFound)
}

// TestAPICancelTaskChainTerminalIsNoOpContract cancels a chain that has already
// finished: the endpoint answers 200 with the settled state rather than an error,
// and the status is unchanged.
func TestAPICancelTaskChainTerminalIsNoOpContract(t *testing.T) {
	env := newAPITestEnv(t)
	chain := submitContractChain(t, env)

	// submitContractChain's task reports running forever, so drive a terminal
	// chain through the real worker instead: a chain of a task that just returns.
	done, err := env.app.TaskChains().Submit(&taskutil.TaskChain{
		Name:  "done_chain",
		Tasks: []taskutil.Task{&doneTask{}},
	})
	if err != nil {
		t.Fatalf("Submit done chain: %v", err)
	}
	settled := waitForTaskChain(t, env, done.ID)
	if settled.Status != "completed" {
		t.Fatalf("precondition: done chain status = %q, want completed", settled.Status)
	}

	rec := env.post(taskChainCancelURL(done.ID), nil)
	assertStatus(t, rec, http.StatusOK)
	resp := decodeJSON[server.TaskChain](t, rec)
	if resp.Status != "completed" {
		t.Errorf("status after cancelling a finished chain = %q, want completed", resp.Status)
	}

	// The unrelated running chain is untouched by the terminal cancel above.
	if _, ok := env.app.TaskChains().Get(chain.ID); !ok {
		t.Errorf("cancelling a terminal chain disturbed an unrelated chain")
	}
}

// doneTask runs to completion immediately, so a chain of it settles as soon as
// the worker picks it up.
type doneTask struct{}

func (t *doneTask) Run(ctx context.Context) error { return nil }
func (t *doneTask) Name() string                  { return "done" }
func (t *doneTask) Title() string                 { return "done" }
func (t *doneTask) Description() string           { return "done" }
func (t *doneTask) Percentage() float64           { return 100 }
func (t *doneTask) Status() taskutil.Status       { return taskutil.StatusCompleted }
