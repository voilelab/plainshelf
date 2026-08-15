package server

import (
	"net/http"
	"sort"
)

type ShelfInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// GET /api/shelves
func (app *App) HandleGetShelves(w http.ResponseWriter, _ *http.Request) {
	shelves := app.shelfManager.GetAllShelves()
	shelfInfos := make([]ShelfInfo, 0, len(shelves))
	for _, shelf := range shelves {
		shelfInfos = append(shelfInfos, ShelfInfo{
			ID:   shelf.ID,
			Name: shelf.Name,
		})
	}
	sort.Slice(shelfInfos, func(i, j int) bool {
		return shelfInfos[i].ID < shelfInfos[j].ID
	})

	app.writeJSON(w, http.StatusOK, shelfInfos)
}

type ShelfStatusResponse struct {
	Ready bool   `json:"ready"`
	Error string `json:"error,omitempty"`
}

// GET /api/shelves/{shelf_id}/status
func (app *App) HandleAPIGetShelfStatus(w http.ResponseWriter, r *http.Request) {
	shelfData, ok := app.resolveShelf(w, r)
	if !ok {
		return
	}

	resp := ShelfStatusResponse{Ready: shelfData.IsReady()}
	if initErr := shelfData.InitErr(); initErr != nil {
		resp.Error = initErr.Error()
	}

	app.writeJSON(w, http.StatusOK, resp)
}
