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

	// Mode names the HTTP surface this server mounts, so a client can tell a
	// reading binary from a library server that happens to be read-only. The two
	// differ in what exists, not only in what is refused: a reader serves no
	// trash, log, setting or task routes, and the frontend has to keep the pages
	// that call them out of reach rather than let them fail.
	//
	// Always present, and "full" rather than "" on an ordinary server, so a
	// client reads a name instead of inferring one from an absent field.
	Mode string `json:"mode"`
}

// GET /api/mode
func (h *metaHandlers) getMode(w http.ResponseWriter, r *http.Request) {
	h.writeJSON(w, http.StatusOK, modeResponse{
		ReadOnly: h.conf.readOnly(),
		Mode:     modeName(h.conf.Mode),
	})
}

// modeName is the wire name of a server mode. Only the empty full-API value
// needs translating; every other mode is already spelled the way clients read
// it.
func modeName(mode ServerMode) string {
	if mode == ServerModeFull {
		return "full"
	}
	return string(mode)
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
