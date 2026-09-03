package server

import (
	"errors"
	"math"
	"net/http"
	"strconv"

	"github.com/voilelab/plainshelf/shelf"
)

// ScanResponse reports the walk a rescan performed, so the client can tell the
// user what the button actually found rather than only that it finished.
type ScanResponse struct {
	// ScanID names this walk, so two of them can be told apart in a log.
	ScanID string `json:"scan_id"`

	// ScannedAt is the Unix time the walk began, matching the meaning the
	// exported book cache gives its own timestamp.
	ScannedAt int64 `json:"scanned_at"`

	BookCount   int `json:"book_count"`
	FolderCount int `json:"folder_count"`
}

// ScanConflictResponse names the rescan already walking the shelf, so a client
// refused with 409 can tell its user which one to wait for.
//
// A separate type from ScanResponse rather than a partly filled one: the counts
// belong to a walk this request did not perform, and zeroes in their place read
// as "found nothing". This is the same split the task routes make for
// ErrTaskChainRunning.
type ScanConflictResponse struct {
	ScanID string `json:"scan_id"`
}

// ScanRateLimitResponse answers a rescan refused for asking too often.
//
// Its own type, and its own status, because 409 and 429 ask the client for
// different things: 409 says another walk is running and will end on its own,
// 429 says this client is the problem. A client that saw one status for both
// would tell its user to wait for a walk that does not exist.
type ScanRateLimitResponse struct {
	// RetryAfterSeconds mirrors the Retry-After header, so a browser client that
	// cannot read the header from a cross-status fetch still has the number.
	RetryAfterSeconds int `json:"retry_after_seconds"`

	// Message is the human-readable reason, since this body is the endpoint's
	// own shape rather than writeErr's error envelope.
	Message string `json:"message"`
}

// POST /api/shelves/{shelf_id}/scans
//
// Walks the shelf now and rebuilds its book cache, for a user who has just put
// a book into books/ from outside PlainShelf and does not want to wait out
// scan_interval. It is the answer for every shelf, including the SMB and cloud
// mounts where no filesystem change notification would arrive.
//
// A read, despite the method: nothing is written to the shelf, which is why the
// read-only gate lets it through (see App.rejectReadOnlyWrite). POST rather
// than GET because it does real work and must not be cached or prefetched.
//
// Synchronous rather than a task chain, like the export beside it: the counts
// it reports are the whole point of the call, and a chain would make the
// client poll for them.
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
		// Rounded up to whole seconds and never below one: Retry-After carries no
		// finer unit, and a "0" would invite the immediate retry this is refusing.
		retryAfter := max(1, int(math.Ceil(result.RetryAfter.Seconds())))
		h.Info("refused a rescan for exceeding the rate limit", "shelf_id", shelfData.ID, "retry_after", retryAfter)
		w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
		h.writeJSON(w, http.StatusTooManyRequests, ScanRateLimitResponse{
			RetryAfterSeconds: retryAfter,
			Message:           "too many rescans; this shelf walks on request only so often, please retry shortly",
		})
	case err != nil:
		h.writeErr(w, err, "failed to rescan shelf")
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
