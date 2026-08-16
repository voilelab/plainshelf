package contract_test

import (
	"context"
	"net/http"
	"net/http/httptest"
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

	rec := env.do(httptest.NewRequest(http.MethodGet, "/api/taskchains/"+chain.ID, nil))
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

	rec := env.do(httptest.NewRequest(http.MethodGet, "/api/taskchains/"+chain.ID, nil))
	assertStatus(t, rec, http.StatusOK)

	payload := decodeJSON[map[string]any](t, rec)
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

	rec := env.do(httptest.NewRequest(http.MethodGet, "/api/taskchains/does-not-exist", nil))
	assertStatus(t, rec, http.StatusNotFound)
}
