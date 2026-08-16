package server

import (
	"errors"
	"net/http"

	"github.com/voilelab/plainshelf/shelf"
)

// BookCacheExportResponse reports the walk the export recorded.
type BookCacheExportResponse struct {
	// Timestamp is the Unix time the shelf was walked, matching the value
	// written into the exported file.
	Timestamp int64 `json:"timestamp"`
}

// POST /api/shelves/{shelf_id}/book-cache-exports
//
// Rescans the shelf and rewrites its exported book cache immediately, for a
// user who has just changed something and wants a mobile client to see it
// without waiting for the next interval.
//
// Synchronous rather than a task chain: it is one rescan plus one file write,
// and reporting the recorded timestamp back is the whole point of the call.
func (h *shelfHandlers) exportBookCache(w http.ResponseWriter, r *http.Request) {
	shelfData, ok := h.resolveShelf(w, r)
	if !ok {
		return
	}

	scannedAt, err := shelfData.ExportBookCache()
	if err != nil {
		if errors.Is(err, shelf.ErrShelfInitializing) {
			http.Error(w, "shelf is still initializing", http.StatusServiceUnavailable)
			return
		}
		h.Error("failed to export book cache", "shelf_id", shelfData.ID, "error", err)
		http.Error(w, "failed to export book cache", http.StatusInternalServerError)
		return
	}

	h.writeJSON(w, http.StatusOK, BookCacheExportResponse{Timestamp: scannedAt.Unix()})
}
