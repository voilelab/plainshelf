package contract_test

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"io"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/voilelab/plainshelf/internal/taskutil"
	"github.com/voilelab/plainshelf/server"
)

// TaskChainSubmitResponse is the body every endpoint that starts background work
// answers with: the chain the client should poll.
type TaskChainSubmitResponse struct {
	TaskChainID string `json:"taskchain_id"`
}

// SubmitTaskChain posts to an endpoint that starts background work and asserts
// the status the caller expects. 202 and 409 both name a chain — a conflict
// points at the one already running — so both are decoded and required to carry
// an ID. Any other status is a rejected request with no chain to report.
func SubmitTaskChain(t *testing.T, env *Env, url string, body []byte, wantStatus int) TaskChainSubmitResponse {
	t.Helper()

	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}

	rec := env.Post(url, reader)
	AssertStatus(t, rec, wantStatus)
	if wantStatus != http.StatusAccepted && wantStatus != http.StatusConflict {
		return TaskChainSubmitResponse{}
	}
	AssertJSONContentType(t, rec)

	resp := DecodeJSON[TaskChainSubmitResponse](t, rec)
	if resp.TaskChainID == "" {
		t.Fatalf("response is missing taskchain_id: %s", rec.Body.String())
	}
	return resp
}

// WaitForTaskChain polls the task chain endpoint until the chain reaches a
// terminal status, mirroring what the trash page does.
func WaitForTaskChain(t *testing.T, env *Env, taskChainID string) server.TaskChain {
	t.Helper()

	deadline := time.Now().Add(10 * time.Second)
	for {
		chain := GetJSON[server.TaskChain](t, env, TaskChainURL(taskChainID))

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

// AssertDuplicateChainConflict pins the contract every endpoint that starts one
// shelf-wide chain shares: while a chain is in flight a second request is
// answered 409 naming that same chain instead of queueing a redundant one.
// submit posts to the endpoint under test and asserts the status it is handed.
// The chain is left finished, and its response returned, so a caller whose
// endpoint can be asked again afterwards can assert it gets a fresh chain.
func AssertDuplicateChainConflict(t *testing.T, env *Env, submit func(wantStatus int) TaskChainSubmitResponse) TaskChainSubmitResponse {
	t.Helper()

	// Block the worker so the first chain stays queued, and therefore
	// non-terminal, for the duration of the test.
	release := blockWorker(t, env)

	first := submit(http.StatusAccepted)
	if conflict := submit(http.StatusConflict); conflict.TaskChainID != first.TaskChainID {
		t.Errorf("conflict taskchain_id = %q, want the running chain %q", conflict.TaskChainID, first.TaskChainID)
	}

	release()
	WaitForTaskChain(t, env, first.TaskChainID)
	return first
}

func TaskChainURL(taskChainID string) string {
	return "/api/taskchains/" + taskChainID
}

func TaskChainCancelURL(taskChainID string) string {
	return "/api/taskchains/" + taskChainID + "/cancel"
}

// taskResult decodes the single task's result through JSON, which is the shape a
// client sees. Decoding into map[string]any therefore pins the wire field names,
// while decoding into the task's own result type reads them back conveniently.
func taskResult[T any](t *testing.T, chain server.TaskChain) T {
	t.Helper()

	if len(chain.Tasks) != 1 {
		t.Fatalf("chain has %d tasks, want 1", len(chain.Tasks))
	}
	raw, err := json.Marshal(chain.Tasks[0].Result)
	if err != nil {
		t.Fatalf("re-encode task result: %v", err)
	}
	var out T
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("decode task result %q: %v", raw, err)
	}
	return out
}

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

func (t *gateTask) Name() string { return "gate" }

func (t *gateTask) Title() string { return "gate" }

func (t *gateTask) Description() string { return "gate" }

func (t *gateTask) Percentage() float64 { return 0 }

func (t *gateTask) Status() taskutil.Status { return taskutil.StatusRunning }

// blockWorker occupies the single worker goroutine so that chains submitted
// afterwards stay queued. The returned function releases it.
func blockWorker(t *testing.T, env *Env) func() {
	t.Helper()

	gate := &gateTask{started: make(chan struct{}), release: make(chan struct{})}
	if _, err := env.App.TaskChains().Submit(&taskutil.TaskChain{
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
