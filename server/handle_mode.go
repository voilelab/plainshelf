package server

import (
	"encoding/json"
	"net/http"
)

type modeResponse struct {
	ReadOnly bool `json:"read_only"`
}

// HandleGetMode returns server runtime mode flags.
func (app *App) HandleGetMode(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(modeResponse{ReadOnly: app.conf.ReadOnly})
}
