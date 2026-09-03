package server

import (
	"net/http"

	"github.com/voilelab/plainshelf/server/task"
)

// POST /api/shelves/{shelf_id}/content-stat-refreshes
func (h *batchHandlers) refreshContentStats(w http.ResponseWriter, r *http.Request) {
	shelfData, ok := h.resolveShelf(w, r)
	if !ok {
		return
	}
	if h.rejectReadOnlyShelf(w, r, shelfData) {
		return
	}

	h.submitTaskChain(w, r,
		task.NewRefreshContentStatsChain(shelfData.ID, shelfData.Shelf, h.requestLogger(r)),
		"failed to schedule content stats refresh task")
}
