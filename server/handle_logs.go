package server

import (
	"encoding/json"
	"net/http"

	"github.com/voilelab/plainshelf/internal/logutil"
)

type LogFileEntry = logutil.Entry

// GET /api/logs
func (app *App) HandleAPIGetLogs(w http.ResponseWriter, r *http.Request) {
	logs, err := logutil.ListLogFiles(app.conf.Logger.LogFile)
	if err != nil {
		app.Error("failed to list log files", "error", err)
		http.Error(w, "failed to list log files", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	err = json.NewEncoder(w).Encode(logs)
	if err != nil {
		app.Error("failed to encode response", "error", err)
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
		return
	}
}
