package server

import (
	"errors"
	"net/http"
	"strings"

	"github.com/voilelab/plainshelf/internal/taskutil"
	"github.com/voilelab/plainshelf/shelf"
)

const (
	bookBatchOperationMove  = "move"
	bookBatchOperationTrash = "trash"
	maxBookBatchSize        = 200
)

type bookBatchRequest struct {
	Operation   string       `json:"operation"`
	BookIDs     []string     `json:"book_ids"`
	TargetLayer shelf.Layers `json:"target_layer"`
}

type bookBatchResponse struct {
	TaskChainID string `json:"taskchain_id"`
}

func normalizeBookBatchIDs(ids []string) ([]string, error) {
	seen := make(map[string]struct{}, len(ids))
	result := make([]string, 0, len(ids))
	for _, raw := range ids {
		id := strings.TrimSpace(raw)
		if id == "" {
			return nil, errors.New("book_ids must not contain empty values")
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	if len(result) == 0 {
		return nil, errors.New("book_ids must not be empty")
	}
	if len(result) > maxBookBatchSize {
		return nil, errors.New("too many book_ids")
	}
	return result, nil
}

// POST /api/shelves/{shelf_id}/book-batches
func (app *App) HandleAPIBookBatch(w http.ResponseWriter, r *http.Request) {
	shelfData, ok := app.resolveShelf(w, r)
	if !ok {
		return
	}

	var request bookBatchRequest
	if !decodeStrictJSON(w, r, &request) {
		return
	}

	request.Operation = strings.TrimSpace(request.Operation)
	if request.Operation != bookBatchOperationMove && request.Operation != bookBatchOperationTrash {
		http.Error(w, "invalid operation", http.StatusBadRequest)
		return
	}
	ids, err := normalizeBookBatchIDs(request.BookIDs)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if request.Operation == bookBatchOperationMove {
		if request.TargetLayer == nil {
			http.Error(w, "target_layer is required for move", http.StatusBadRequest)
			return
		}
		if err := shelf.ValidateLayers(request.TargetLayer); err != nil {
			http.Error(w, "invalid target_layer", http.StatusBadRequest)
			return
		}
	} else if request.TargetLayer != nil {
		http.Error(w, "target_layer is only valid for move", http.StatusBadRequest)
		return
	}

	chain, err := app.taskChains.Submit(newBookBatchChain(shelfData.ID, shelfData.Shelf, &app.Logger, request.Operation, ids, request.TargetLayer))
	switch {
	case errors.Is(err, taskutil.ErrTaskChainRunning):
		app.writeBookBatchResponse(w, chain.ID, http.StatusConflict)
	case errors.Is(err, taskutil.ErrWorkerBusy):
		w.Header().Set("Retry-After", "5")
		http.Error(w, "background worker is busy", http.StatusServiceUnavailable)
	case err != nil:
		app.Error("failed to schedule book batch task", "error", err)
		http.Error(w, "failed to schedule book batch task", http.StatusInternalServerError)
	default:
		app.writeBookBatchResponse(w, chain.ID, http.StatusAccepted)
	}
}

func (app *App) writeBookBatchResponse(w http.ResponseWriter, taskChainID string, status int) {
	app.writeJSON(w, status, bookBatchResponse{TaskChainID: taskChainID})
}
