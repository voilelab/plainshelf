package server

import (
	"errors"
	"math"
	"net/http"
	"strings"

	"github.com/voilelab/plainshelf/server/task"
	"github.com/voilelab/plainshelf/shelf"
)

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
	SeriesName   *string          `json:"series_name"`
	StartIndex   *float64         `json:"start_index"`
	Clear        bool             `json:"clear"`
}

func (r *bookBatchRequest) hasSeriesFields() bool {
	return r.SeriesName != nil || r.StartIndex != nil || r.Clear
}

func (r *bookBatchRequest) seriesSpec(count int) (task.BookBatchSeries, error) {
	if r.TargetFolder != nil {
		return task.BookBatchSeries{}, errors.New("target_folder is only valid for move")
	}
	if r.Clear {
		if r.SeriesName != nil || r.StartIndex != nil {
			return task.BookBatchSeries{}, errors.New("clear excludes series_name and start_index")
		}
		return task.BookBatchSeries{Clear: true}, nil
	}
	if r.SeriesName == nil || strings.TrimSpace(*r.SeriesName) == "" {
		return task.BookBatchSeries{}, errors.New("series_name must not be blank")
	}
	if r.StartIndex == nil || *r.StartIndex < 0 || math.IsInf(*r.StartIndex+float64(count-1), 0) {
		return task.BookBatchSeries{}, errors.New("start_index must be a finite, non-negative number")
	}
	return task.BookBatchSeries{Name: strings.TrimSpace(*r.SeriesName), StartIndex: *r.StartIndex}, nil
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

func (h *batchHandlers) bookBatch(w http.ResponseWriter, r *http.Request) {
	shelfData, ok := h.resolveShelf(w, r)
	if !ok {
		return
	}
	if h.rejectReadOnlyShelf(w, r, shelfData) {
		return
	}

	var request bookBatchRequest
	if !decodeStrictJSON(w, r, &request) {
		return
	}

	request.Operation = strings.TrimSpace(request.Operation)
	switch request.Operation {
	case task.BookBatchOperationMove, task.BookBatchOperationTrash, task.BookBatchOperationSetSeries:
	default:
		http.Error(w, "invalid operation", http.StatusBadRequest)
		return
	}
	ids, err := normalizeBookBatchIDs(request.BookIDs)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	var series task.BookBatchSeries
	if request.Operation == task.BookBatchOperationSetSeries {
		series, err = request.seriesSpec(len(ids))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	} else if request.hasSeriesFields() {
		http.Error(w, "series fields are only valid for set_series", http.StatusBadRequest)
		return
	} else if request.Operation == task.BookBatchOperationMove {
		if request.TargetFolder == nil {
			http.Error(w, "target_folder is required for move", http.StatusBadRequest)
			return
		}
		if err := shelfData.ValidateFolderPath(request.TargetFolder); err != nil {
			http.Error(w, "invalid target_folder", http.StatusBadRequest)
			return
		}
	} else if request.TargetFolder != nil {
		http.Error(w, "target_folder is only valid for move", http.StatusBadRequest)
		return
	}

	visible, err := h.visibility(shelfData).visibleBooks()
	if err != nil {
		h.writeErr(w, r, err, "failed to list books")
		return
	}

	h.submitTaskChain(w, r,
		task.NewBookBatchChain(shelfData.ID, shelfData.Shelf, h.requestLogger(r), task.BookBatchSpec{
			Operation:    request.Operation,
			BookIDs:      ids,
			TargetFolder: request.TargetFolder,
			Series:       series,
		}, visible),
		"failed to schedule book batch task")
}
