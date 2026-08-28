package server

import (
	"net/http"

	"github.com/voilelab/plainshelf/internal/taskutil"
	"github.com/voilelab/plainshelf/internal/util"
)

// taskHandlers reports on the chains the other groups submit.
type taskHandlers struct {
	*taskSubmitter
}

type Task struct {
	Name        string  `json:"name"`
	Title       string  `json:"title"`
	Description string  `json:"description,omitempty"`
	Status      string  `json:"status"`
	Percentage  float64 `json:"percentage"`
	Result      any     `json:"result,omitempty"`
}

type TaskChain struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Title       string        `json:"title"`
	Description string        `json:"description,omitempty"`
	Status      string        `json:"status"`
	Percentage  float64       `json:"percentage"`
	CreatedAt   util.JSONTime `json:"created_at,omitzero"`
	Tasks       []Task        `json:"tasks"`
}

// newTaskChainResponse snapshots a chain that is still being worked on, so the
// order the fields are read in is part of the contract.
//
// Every status is read before the result it describes, never after. A task only
// settles once its result is complete and never mutates it afterwards, so a
// status read first can be stale relative to the result but never ahead of it:
// the response may say "running" next to an already-final result, which costs a
// client one more poll, but it can never report a terminal status next to a
// result that is still filling in. Reading the result first allowed exactly
// that — a chain reporting partially_completed with an empty failure list,
// which a client polling for a terminal status would take as the final answer.
func newTaskChainResponse(chain *taskutil.TaskChain) TaskChain {
	chainStatus := chain.Status().String()

	tasks := make([]Task, 0, len(chain.Tasks))
	for _, task := range chain.Tasks {
		status := task.Status().String()

		var result any
		if provider, ok := task.(taskutil.ResultProvider); ok {
			result = provider.Result()
		}
		tasks = append(tasks, Task{
			Name:        task.Name(),
			Title:       task.Title(),
			Description: task.Description(),
			Status:      status,
			Percentage:  task.Percentage(),
			Result:      result,
		})
	}

	return TaskChain{
		ID:          chain.ID,
		Name:        chain.Name,
		Title:       chain.Title,
		Description: chain.Description,
		Status:      chainStatus,
		Percentage:  chain.Percentage(),
		CreatedAt:   util.JSONTime(chain.CreatedAt),
		Tasks:       tasks,
	}
}

// GET /api/taskchains/{taskchain_id}
func (h *taskHandlers) getTaskChain(w http.ResponseWriter, r *http.Request) {
	taskChainID, err := readTaskChainID(r)
	if err != nil {
		http.Error(w, "invalid taskchain_id", http.StatusBadRequest)
		return
	}

	chain, exists := h.pool.Get(taskChainID)
	if !exists {
		http.Error(w, "task chain not found", http.StatusNotFound)
		return
	}

	h.writeJSON(w, http.StatusOK, newTaskChainResponse(chain))
}

// POST /api/taskchains/{taskchain_id}/cancel
//
// Cancel is a mutating method, so it stays behind the local_token boundary and
// is refused by read-only mode - which is the intended behaviour: a read-only
// server refuses the transfer that would start a chain, so there is never an
// in-progress chain to cancel there.
//
// The response is the chain's current state, exactly like getTaskChain. A cancel
// stops the work between task boundaries rather than instantly, so the caller
// polls the same GET endpoint to watch it settle; cancelling a chain that has
// already finished is a no-op that still reports the settled state.
func (h *taskHandlers) cancelTaskChain(w http.ResponseWriter, r *http.Request) {
	taskChainID, err := readTaskChainID(r)
	if err != nil {
		http.Error(w, "invalid taskchain_id", http.StatusBadRequest)
		return
	}

	chain, result := h.pool.Cancel(taskChainID)
	if result == taskutil.CancelNotFound {
		http.Error(w, "task chain not found", http.StatusNotFound)
		return
	}

	h.writeJSON(w, http.StatusOK, newTaskChainResponse(chain))
}
