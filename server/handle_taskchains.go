package server

import (
	"encoding/json"
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

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err := json.NewEncoder(w).Encode(newTaskChainResponse(chain)); err != nil {
		app.Error("failed to encode response", "error", err)
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
		return
	}
}
