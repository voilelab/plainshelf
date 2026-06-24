package server

import (
	"encoding/json"
	"net/http"

	"github.com/voilelab/plainshelf/internal/version"
)

type versionResponse struct {
	Version string `json:"version"`
}

// HandleGetVersion returns the running server version.
func (app *App) HandleGetVersion(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(versionResponse{Version: version.Version})
}
