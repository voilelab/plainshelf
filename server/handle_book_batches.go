package server

import (
	"errors"
	"net/http"
	"strings"

	"github.com/voilelab/plainshelf/server/task"
	"github.com/voilelab/plainshelf/shelf"
)

// batchHandlers serves the endpoints that start a sweep over many books.
type batchHandlers struct {
	*taskSubmitter
}

const (
	maxBookBatchSize = 200
)

type bookBatchRequest struct {
	Operation    string           `json:"operation"`
	BookIDs      []string         `json:"book_ids"`
	TargetFolder shelf.FolderPath `json:"target_folder"`
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
func (h *batchHandlers) bookBatch(w http.ResponseWriter, r *http.Request) {
	shelfData, ok := h.resolveShelf(w, r)
	if !ok {
		return
	}
	if h.rejectReadOnlyShelf(w, shelfData) {
		return
	}

	var request bookBatchRequest
	if !decodeStrictJSON(w, r, &request) {
		return
	}

	request.Operation = strings.TrimSpace(request.Operation)
	if request.Operation != task.BookBatchOperationMove && request.Operation != task.BookBatchOperationTrash {
		http.Error(w, "invalid operation", http.StatusBadRequest)
		return
	}
	ids, err := normalizeBookBatchIDs(request.BookIDs)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if request.Operation == task.BookBatchOperationMove {
		if request.TargetFolder == nil {
			http.Error(w, "target_folder is required for move", http.StatusBadRequest)
			return
		}
		if err := shelf.ValidateFolderPath(request.TargetFolder); err != nil {
			http.Error(w, "invalid target_folder", http.StatusBadRequest)
			return
		}
	} else if request.TargetFolder != nil {
		http.Error(w, "target_folder is only valid for move", http.StatusBadRequest)
		return
	}

	h.submitTaskChain(w,
		task.NewBookBatchChain(shelfData.ID, shelfData.Shelf, h.Logger, request.Operation, ids, request.TargetFolder),
		"failed to schedule book batch task")
}
