package server

import (
	"net/http"

	"github.com/voilelab/plainshelf/internal/version"
)

type versionResponse struct {
	Version string `json:"version"`
}

// HandleGetVersion returns the running server version.
func (app *App) HandleGetVersion(w http.ResponseWriter, r *http.Request) {
	app.writeJSON(w, http.StatusOK, versionResponse{Version: version.Version})
}
