package server

import (
	"net/http"

	"github.com/voilelab/plainshelf/server/task"
)

func (h *batchHandlers) refreshContentStats(w http.ResponseWriter, r *http.Request) {
	shelfData, ok := h.resolveShelf(w, r)
	if !ok {
		return
	}
	if h.rejectReadOnlyShelf(w, r, shelfData) {
		return
	}

	visible, err := h.visibility(shelfData).visibleBookIDs()
	if err != nil {
		h.writeErr(w, r, err, "failed to list books")
		return
	}

	h.submitTaskChain(w, r,
		task.NewRefreshContentStatsChain(shelfData.ID, shelfData.Shelf, h.requestLogger(r), visible),
		"failed to schedule content stats refresh task")
}
