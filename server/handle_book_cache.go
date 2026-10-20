package server

import "net/http"

type BookCacheExportResponse struct {
	Timestamp int64 `json:"timestamp"`
}

func (h *shelfHandlers) exportBookCache(w http.ResponseWriter, r *http.Request) {
	shelfData, ok := h.resolveShelf(w, r)
	if !ok {
		return
	}

	scannedAt, err := shelfData.ExportBookCache()
	if err != nil {

		h.writeErr(w, r, err, "failed to export book cache")
		return
	}

	h.writeJSON(w, http.StatusOK, BookCacheExportResponse{Timestamp: scannedAt.Unix()})
}
