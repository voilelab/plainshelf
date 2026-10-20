package server

import (
	"net/http"

	"github.com/voilelab/plainshelf/internal/version"
)

type metaHandlers struct {
	*apiCore

	conf *AppConf
}

type modeResponse struct {
	ReadOnly bool `json:"read_only"`
}

func (h *metaHandlers) getMode(w http.ResponseWriter, r *http.Request) {
	h.writeJSON(w, http.StatusOK, modeResponse{ReadOnly: h.conf.ReadOnly})
}

type versionResponse struct {
	Version string `json:"version"`
}

func (h *metaHandlers) getVersion(w http.ResponseWriter, r *http.Request) {
	h.writeJSON(w, http.StatusOK, versionResponse{Version: version.Version})
}

func (h *metaHandlers) health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("1"))
}
