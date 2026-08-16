package server

import (
	"net/http"

	"github.com/voilelab/plainshelf/internal/version"
)

// metaHandlers answers what a client asks before it asks for anything else:
// which build this is, and whether it will accept writes.
type metaHandlers struct {
	*apiCore

	conf *AppConf
}

type modeResponse struct {
	ReadOnly bool `json:"read_only"`
}

// GET /api/mode
func (h *metaHandlers) getMode(w http.ResponseWriter, r *http.Request) {
	h.writeJSON(w, http.StatusOK, modeResponse{ReadOnly: h.conf.ReadOnly})
}

type versionResponse struct {
	Version string `json:"version"`
}

// GET /api/version
func (h *metaHandlers) getVersion(w http.ResponseWriter, r *http.Request) {
	h.writeJSON(w, http.StatusOK, versionResponse{Version: version.Version})
}

// GET /health
func (h *metaHandlers) health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("1"))
}
