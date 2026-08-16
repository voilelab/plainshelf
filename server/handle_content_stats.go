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

	h.submitTaskChain(w,
		task.NewRefreshContentStatsChain(shelfData.ID, shelfData.Shelf, h.Logger),
		"failed to schedule content stats refresh task")
}
