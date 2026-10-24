package platform_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/voilelab/plainshelf/server/contract/apitest"

	"github.com/voilelab/plainshelf/internal/taskutil"
	"github.com/voilelab/plainshelf/server"
)

type contractTask struct{}

func (t *contractTask) Run(ctx context.Context) error { return nil }
func (t *contractTask) Name() string                  { return "contract_task" }
func (t *contractTask) Title() string                 { return "Contract task" }
func (t *contractTask) Description() string           { return "deleted 1 of 2 books" }
func (t *contractTask) Percentage() float64           { return 50 }
func (t *contractTask) Status() taskutil.Status       { return taskutil.StatusRunning }

func submitContractChain(t *testing.T, env *apitest.Env) *taskutil.TaskChain {
	t.Helper()

	chain, err := env.App.TaskChains().Submit(&taskutil.TaskChain{
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
	env := apitest.New(t)
	chain := submitContractChain(t, env)

	rec := env.Get(apitest.TaskChainURL(chain.ID))
	apitest.AssertStatus(t, rec, http.StatusOK)
	apitest.AssertJSONContentType(t, rec)

	resp := apitest.DecodeJSON[server.TaskChain](t, rec)
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

	t.Run("schema", func(t *testing.T) {
		payload := apitest.DecodeJSON[map[string]any](t, rec)
		for _, key := range []string{"id", "name", "title", "description", "status", "percentage", "created_at", "tasks"} {
			if _, ok := payload[key]; !ok {
				t.Errorf("response is missing the %q field: %v", key, payload)
			}
		}

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
	})
}

func TestAPIGetTaskChainNotFoundContract(t *testing.T) {
	env := apitest.New(t)

	rec := env.Get(apitest.TaskChainURL("does-not-exist"))
	apitest.AssertStatus(t, rec, http.StatusNotFound)
}

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

func TestAPICancelTaskChainRequiresTokenContract(t *testing.T) {
	env := apitest.New(t)

	rec := env.DoRaw(httptest.NewRequest(http.MethodPost, apitest.TaskChainCancelURL("any-id"), nil))
	apitest.AssertStatus(t, rec, http.StatusUnauthorized)
}

func TestAPICancelTaskChainReadOnlyContract(t *testing.T) {
	env := apitest.New(t)
	env.SetReadOnly(t, true)

	rec := env.Post(apitest.TaskChainCancelURL("any-id"), nil)
	apitest.AssertStatus(t, rec, http.StatusForbidden)
}

func TestAPICancelTaskChainStopsRunningChainContract(t *testing.T) {
	env := apitest.New(t)

	task := &cancelSignalTask{started: make(chan struct{})}
	chain, err := env.App.TaskChains().Submit(&taskutil.TaskChain{
		Name:  "cancel_chain",
		Tasks: []taskutil.Task{task},
	})
	if err != nil {
		t.Fatalf("Submit task chain: %v", err)
	}

	<-task.started
	rec := env.Post(apitest.TaskChainCancelURL(chain.ID), nil)
	apitest.AssertStatus(t, rec, http.StatusOK)
	apitest.AssertJSONContentType(t, rec)

	resp := apitest.DecodeJSON[server.TaskChain](t, rec)
	if resp.ID != chain.ID {
		t.Errorf("id = %q, want %q", resp.ID, chain.ID)
	}

	settled := apitest.WaitForTaskChain(t, env, chain.ID)
	if settled.Status != "failed" {
		t.Errorf("status after cancel = %q, want failed", settled.Status)
	}
}

func TestAPICancelTaskChainNotFoundContract(t *testing.T) {
	env := apitest.New(t)

	rec := env.Post(apitest.TaskChainCancelURL("does-not-exist"), nil)
	apitest.AssertStatus(t, rec, http.StatusNotFound)
}

func TestAPICancelTaskChainTerminalIsNoOpContract(t *testing.T) {
	env := apitest.New(t)
	chain := submitContractChain(t, env)

	done, err := env.App.TaskChains().Submit(&taskutil.TaskChain{
		Name:  "done_chain",
		Tasks: []taskutil.Task{&doneTask{}},
	})
	if err != nil {
		t.Fatalf("Submit done chain: %v", err)
	}
	settled := apitest.WaitForTaskChain(t, env, done.ID)
	if settled.Status != "completed" {
		t.Fatalf("precondition: done chain status = %q, want completed", settled.Status)
	}

	rec := env.Post(apitest.TaskChainCancelURL(done.ID), nil)
	apitest.AssertStatus(t, rec, http.StatusOK)
	resp := apitest.DecodeJSON[server.TaskChain](t, rec)
	if resp.Status != "completed" {
		t.Errorf("status after cancelling a finished chain = %q, want completed", resp.Status)
	}

	if _, ok := env.App.TaskChains().Get(chain.ID); !ok {
		t.Errorf("cancelling a terminal chain disturbed an unrelated chain")
	}
}

type doneTask struct{}

func (t *doneTask) Run(ctx context.Context) error { return nil }
func (t *doneTask) Name() string                  { return "done" }
func (t *doneTask) Title() string                 { return "done" }
func (t *doneTask) Description() string           { return "done" }
func (t *doneTask) Percentage() float64           { return 100 }
func (t *doneTask) Status() taskutil.Status       { return taskutil.StatusCompleted }
