package server

import (
	"context"
	"testing"

	"github.com/voilelab/plainshelf/internal/taskutil"
)

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

func (t *settlingTask) Result() any {
	if !t.settled {
		t.settled = true
		return []string{}
	}
	return []string{"missing-book"}
}

func TestTaskChainTerminalResponseIsSettled(t *testing.T) {
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
