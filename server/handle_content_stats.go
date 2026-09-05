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

	// The sweep runs after this response, so what it may touch is decided here,
	// while the request that asked for it is still the one being answered.
	visible, err := h.visibility(shelfData).visibleBookIDs()
	if err != nil {
		h.writeErr(w, r, err, "failed to list books")
		return
	}

	h.submitTaskChain(w, r,
		task.NewRefreshContentStatsChain(shelfData.ID, shelfData.Shelf, h.requestLogger(r), visible),
		"failed to schedule content stats refresh task")
}
