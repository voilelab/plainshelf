package server

import (
	"net/http"

	"github.com/voilelab/plainshelf/server/task"
)

// fingerprintHandlers owns the similarity fingerprint endpoints.
type fingerprintHandlers struct {
	*taskSubmitter
}

// POST /api/shelves/{shelf_id}/fingerprint-refreshes
//
// The sweep's product is a file under app/, so a read-only shelf is refused here
// rather than answered with 202: a caller that got a chain ID would have to read
// the task report to learn that the work never happened.
func (h *fingerprintHandlers) refreshFingerprints(w http.ResponseWriter, r *http.Request) {
	shelfData, ok := h.resolveShelf(w, r)
	if !ok {
		return
	}
	if h.rejectReadOnlyShelf(w, shelfData) {
		return
	}

	h.submitTaskChain(w,
		task.NewFingerprintSourcesChain(shelfData.ID, shelfData.Shelf, h.Logger),
		"failed to schedule fingerprint task")
}
