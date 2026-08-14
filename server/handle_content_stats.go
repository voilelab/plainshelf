package server

import (
	"net/http"

	"github.com/voilelab/plainshelf/server/task"
)

// POST /api/shelves/{shelf_id}/content-stat-refreshes
func (app *App) HandleAPIRefreshContentStats(w http.ResponseWriter, r *http.Request) {
	shelfData, ok := app.resolveShelf(w, r)
	if !ok {
		return
	}

	app.submitTaskChain(w,
		task.NewRefreshContentStatsChain(shelfData.ID, shelfData.Shelf, &app.Logger),
		"failed to schedule content stats refresh task")
}
