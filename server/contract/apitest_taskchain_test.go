package contract_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/voilelab/plainshelf/internal/taskutil"
	"github.com/voilelab/plainshelf/server"
)

// taskChainSubmitResponse is the body every endpoint that starts background work
// answers with: the chain the client should poll.
type taskChainSubmitResponse struct {
	TaskChainID string `json:"taskchain_id"`
}

// submitTaskChain posts to an endpoint that starts background work and asserts
// the status the caller expects. 202 and 409 both name a chain — a conflict
// points at the one already running — so both are decoded and required to carry
// an ID. Any other status is a rejected request with no chain to report.
func submitTaskChain(t *testing.T, env *apiTestEnv, url string, body []byte, wantStatus int) taskChainSubmitResponse {
	t.Helper()

	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}

	rec := env.post(url, reader)
	assertStatus(t, rec, wantStatus)
	if wantStatus != http.StatusAccepted && wantStatus != http.StatusConflict {
		return taskChainSubmitResponse{}
	}
	assertJSONContentType(t, rec)

	resp := decodeJSON[taskChainSubmitResponse](t, rec)
	if resp.TaskChainID == "" {
		t.Fatalf("response is missing taskchain_id: %s", rec.Body.String())
	}
	return resp
}

// waitForTaskChain polls the task chain endpoint until the chain reaches a
// terminal status, mirroring what the trash page does.
func waitForTaskChain(t *testing.T, env *apiTestEnv, taskChainID string) server.TaskChain {
	t.Helper()

	deadline := time.Now().Add(10 * time.Second)
	for {
		chain := getJSON[server.TaskChain](t, env, taskChainURL(taskChainID))

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

func taskChainURL(taskChainID string) string {
	return "/api/taskchains/" + taskChainID
}

func taskChainCancelURL(taskChainID string) string {
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
func blockWorker(t *testing.T, env *apiTestEnv) func() {
	t.Helper()

	gate := &gateTask{started: make(chan struct{}), release: make(chan struct{})}
	if _, err := env.app.TaskChains().Submit(&taskutil.TaskChain{
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
