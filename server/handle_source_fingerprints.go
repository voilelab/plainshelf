package server

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/voilelab/plainshelf/server/task"
)

// POST /api/shelves/{shelf_id}/source-fingerprints
//
// A truthy force query parameter rebuilds every source's fingerprint, ignoring
// the cache, for when a stat comparison has missed a content change. Without it
// the sweep stays incremental and skips whatever the cache can already answer.
func (h *batchHandlers) fingerprintSources(w http.ResponseWriter, r *http.Request) {
	shelfData, ok := h.resolveShelf(w, r)
	if !ok {
		return
	}
	// Before force is even read: force must not offer a way around the read-only
	// boundary, so the shelf is gated whatever the flag says.
	if h.rejectReadOnlyShelf(w, r, shelfData) {
		return
	}

	force, ok := parseForce(w, r)
	if !ok {
		return
	}

	// The sweep runs after this response, so what it may read is decided here,
	// the same way the content-stats sweep decides it.
	visible, err := h.visibility(shelfData).visibleBookIDs()
	if err != nil {
		h.writeErr(w, r, err, "failed to list books")
		return
	}

	h.submitTaskChain(w, r,
		task.NewFingerprintSourcesChain(shelfData.ID, shelfData.Shelf, force, h.requestLogger(r), visible),
		"failed to schedule source fingerprint task")
}

// parseForce reads the optional force query parameter, defaulting to false when
// it is absent and rejecting a value that is not a boolean.
func parseForce(w http.ResponseWriter, r *http.Request) (force, ok bool) {
	raw := strings.TrimSpace(r.URL.Query().Get("force"))
	if raw == "" {
		return false, true
	}
	force, err := strconv.ParseBool(raw)
	if err != nil {
		http.Error(w, "force must be true or false", http.StatusBadRequest)
		return false, false
	}
	return force, true
}
