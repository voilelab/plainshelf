package server

import (
	"errors"
	"net/http"

	"github.com/voilelab/plainshelf/internal/taskutil"
	"github.com/voilelab/plainshelf/internal/util"
)

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

func newTaskChainResponse(chain *taskutil.TaskChain) TaskChain {
	tasks := make([]Task, 0, len(chain.Tasks))
	for _, task := range chain.Tasks {
		var result any
		if provider, ok := task.(taskutil.ResultProvider); ok {
			result = provider.Result()
		}
		tasks = append(tasks, Task{
			Name:        task.Name(),
			Title:       task.Title(),
			Description: task.Description(),
			Status:      task.Status().String(),
			Percentage:  task.Percentage(),
			Result:      result,
		})
	}

	return TaskChain{
		ID:          chain.ID,
		Name:        chain.Name,
		Title:       chain.Title,
		Description: chain.Description,
		Status:      chain.Status().String(),
		Percentage:  chain.Percentage(),
		CreatedAt:   util.JSONTime(chain.CreatedAt),
		Tasks:       tasks,
	}
}

type taskChainSubmitResponse struct {
	TaskChainID string `json:"taskchain_id"`
}

// submitTaskChain answers 202 with the new chain's ID, or 409 with the ID of
// the chain already in flight so the client can attach to it instead.
func (app *App) submitTaskChain(w http.ResponseWriter, chain *taskutil.TaskChain, fallback string) {
	submitted, err := app.taskChains.Submit(chain)
	switch {
	case errors.Is(err, taskutil.ErrTaskChainRunning):
		app.writeJSON(w, http.StatusConflict, taskChainSubmitResponse{TaskChainID: submitted.ID})
	case err != nil:
		app.writeErr(w, err, fallback)
	default:
		app.writeJSON(w, http.StatusAccepted, taskChainSubmitResponse{TaskChainID: submitted.ID})
	}
}

// GET /api/taskchains/{taskchain_id}
func (app *App) HandleAPIGetTaskChain(w http.ResponseWriter, r *http.Request) {
	taskChainID, err := readTaskChainID(r)
	if err != nil {
		http.Error(w, "invalid taskchain_id", http.StatusBadRequest)
		return
	}

	chain, exists := app.taskChains.Get(taskChainID)
	if !exists {
		http.Error(w, "task chain not found", http.StatusNotFound)
		return
	}

	app.writeJSON(w, http.StatusOK, newTaskChainResponse(chain))
}
