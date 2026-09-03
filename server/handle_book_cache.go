package server

import "net/http"

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
		// Through the error table rather than a 500 for everything that is not
		// ErrShelfInitializing: a read-only shelf refuses this write, and that
		// is a 409 the caller can act on, not a server fault.
		h.writeErr(w, r, err, "failed to export book cache")
		return
	}

	h.writeJSON(w, http.StatusOK, BookCacheExportResponse{Timestamp: scannedAt.Unix()})
}
