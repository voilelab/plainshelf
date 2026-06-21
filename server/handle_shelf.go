package server

import (
	"encoding/json"
	"net/http"
	"sort"
)

type ShelfInfo struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Path         string `json:"path"`
	ScanInterval string `json:"scan_interval"`
}

// GET /api/shelves
func (app *App) HandleGetShelves(w http.ResponseWriter, _ *http.Request) {
	shelves := app.shelfManager.GetAllShelves()
	shelfInfos := make([]ShelfInfo, 0, len(shelves))
	for _, shelf := range shelves {
		shelfInfos = append(shelfInfos, ShelfInfo{
			ID:           shelf.ID,
			Name:         shelf.Name,
			Path:         shelf.LibRoot,
			ScanInterval: shelf.GetScanInterval().String(),
		})
	}
	sort.Slice(shelfInfos, func(i, j int) bool {
		return shelfInfos[i].ID < shelfInfos[j].ID
	})

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	err := json.NewEncoder(w).Encode(shelfInfos)
	if err != nil {
		app.Error("failed to encode response", "error", err)
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
		return
	}
}

type ShelfStatusResponse struct {
	Ready bool   `json:"ready"`
	Error string `json:"error,omitempty"`
}

// GET /api/shelves/{shelf_id}/status
func (app *App) HandleAPIGetShelfStatus(w http.ResponseWriter, r *http.Request) {
	shelfID, err := readShelfID(r)
	if err != nil {
		http.Error(w, "invalid shelf_id", http.StatusBadRequest)
		return
	}

	shelfData, ok := app.shelfManager.GetShelf(shelfID)
	if !ok {
		http.Error(w, "shelf not found", http.StatusNotFound)
		return
	}

	resp := ShelfStatusResponse{Ready: shelfData.IsReady()}
	if initErr := shelfData.InitErr(); initErr != nil {
		resp.Error = initErr.Error()
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		app.Error("failed to encode response", "error", err)
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
	}
}

type updateShelfRequest struct {
	Name         string `json:"name"`
	ScanInterval string `json:"scan_interval"`
}

// PATCH /api/shelves/{shelf_id}
func (app *App) HandleAPIUpdateShelf(w http.ResponseWriter, r *http.Request) {
	shelfID, err := readShelfID(r)
	if err != nil {
		http.Error(w, "invalid shelf_id", http.StatusBadRequest)
		return
	}

	if _, ok := app.shelfManager.GetShelf(shelfID); !ok {
		http.Error(w, "shelf not found", http.StatusNotFound)
		return
	}

	var req updateShelfRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if err := app.shelfManager.UpdateShelf(shelfID, req.Name, req.ScanInterval); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
