package server

import (
	"net/http"
)

type modeResponse struct {
	ReadOnly bool `json:"read_only"`
}

// HandleGetMode returns server runtime mode flags.
func (app *App) HandleGetMode(w http.ResponseWriter, r *http.Request) {
	app.writeJSON(w, http.StatusOK, modeResponse{ReadOnly: app.conf.ReadOnly})
}
