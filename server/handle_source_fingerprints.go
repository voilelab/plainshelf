package server

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/voilelab/plainshelf/server/task"
)

func (h *batchHandlers) fingerprintSources(w http.ResponseWriter, r *http.Request) {
	shelfData, ok := h.resolveShelf(w, r)
	if !ok {
		return
	}

	if h.rejectReadOnlyShelf(w, r, shelfData) {
		return
	}

	force, ok := parseForce(w, r)
	if !ok {
		return
	}

	visible, err := h.visibility(shelfData).visibleBookIDs()
	if err != nil {
		h.writeErr(w, r, err, "failed to list books")
		return
	}

	h.submitTaskChain(w, r,
		task.NewFingerprintSourcesChain(shelfData.ID, shelfData.Shelf, force, h.requestLogger(r), visible),
		"failed to schedule source fingerprint task")
}

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
