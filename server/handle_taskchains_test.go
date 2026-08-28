package server

import (
	"context"
	"testing"

	"github.com/voilelab/plainshelf/internal/taskutil"
)

// settlingTask reproduces the window the response builder has to read across: a
// task that is still running when the snapshot begins and has settled with a
// result by the time it is asked for one. Result() is what advances it, so the
// interleaving is deterministic instead of depending on the scheduler.
type settlingTask struct {
	settled bool
}

func (t *settlingTask) Run(context.Context) error { return nil }
func (t *settlingTask) Name() string              { return "settling" }
func (t *settlingTask) Title() string             { return "Settling" }
func (t *settlingTask) Description() string       { return "" }
func (t *settlingTask) Percentage() float64       { return 100 }

func (t *settlingTask) Status() taskutil.Status {
	if t.settled {
		return taskutil.StatusPartiallyCompleted
	}
	return taskutil.StatusRunning
}

// Result settles the task as a side effect, standing in for the worker
// goroutine recording its last failure while the response is being built.
func (t *settlingTask) Result() any {
	if !t.settled {
		t.settled = true
		return []string{}
	}
	return []string{"missing-book"}
}

// A client polls this endpoint until the status is terminal and then reads the
// result, so pairing a terminal status with a result that is still filling in
// hands it an answer that is final and wrong — a batch reported as
// partially_completed with nothing listed as having failed.
func TestTaskChainResponseNeverPairsTerminalStatusWithAnUnsettledResult(t *testing.T) {
	chain := &taskutil.TaskChain{
		ID:    "chain-1",
		Name:  "book_batch",
		Tasks: []taskutil.Task{&settlingTask{}},
	}

	response := newTaskChainResponse(chain)

	if len(response.Tasks) != 1 {
		t.Fatalf("tasks = %#v, want one", response.Tasks)
	}
	if response.Status == "partially_completed" || response.Tasks[0].Status == "partially_completed" {
		failures, ok := response.Tasks[0].Result.([]string)
		if !ok || len(failures) == 0 {
			t.Fatalf("status = %q/%q with result %#v, want the settled result alongside a terminal status",
				response.Status, response.Tasks[0].Status, response.Tasks[0].Result)
		}
	}
}
