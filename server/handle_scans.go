package server

import (
	"errors"
	"math"
	"net/http"
	"strconv"

	"github.com/voilelab/plainshelf/shelf"
)

type ScanResponse struct {
	ScanID string `json:"scan_id"`

	ScannedAt int64 `json:"scanned_at"`

	BookCount   int `json:"book_count"`
	FolderCount int `json:"folder_count"`
}

type ScanConflictResponse struct {
	ScanID string `json:"scan_id"`
}

type ScanRateLimitResponse struct {
	RetryAfterSeconds int `json:"retry_after_seconds"`

	Message string `json:"message"`
}

func (h *shelfHandlers) rescanShelf(w http.ResponseWriter, r *http.Request) {
	shelfData, ok := h.resolveShelf(w, r)
	if !ok {
		return
	}

	result, err := shelfData.Rescan()
	switch {
	case errors.Is(err, shelf.ErrRescanInProgress):
		h.writeJSON(w, http.StatusConflict, ScanConflictResponse{ScanID: result.ID})
	case errors.Is(err, shelf.ErrRescanRateLimited):
		// RFC 9110 section 10.2.3 defines Retry-After delay-seconds as an integer.
		retryAfter := max(1, int(math.Ceil(result.RetryAfter.Seconds())))
		h.Info("refused a rescan for exceeding the rate limit", "shelf_id", shelfData.ID, "retry_after", retryAfter)
		w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
		h.writeJSON(w, http.StatusTooManyRequests, ScanRateLimitResponse{
			RetryAfterSeconds: retryAfter,
			Message:           "too many rescans; this shelf walks on request only so often, please retry shortly",
		})
	case err != nil:
		h.writeErr(w, r, err, "failed to rescan shelf")
	default:
		h.Info("rescanned shelf", "shelf_id", shelfData.ID, "scan_id", result.ID, "books", result.BookCount, "folders", result.FolderCount)
		h.writeJSON(w, http.StatusOK, ScanResponse{
			ScanID:      result.ID,
			ScannedAt:   result.StartedAt.Unix(),
			BookCount:   result.BookCount,
			FolderCount: result.FolderCount,
		})
	}
}
