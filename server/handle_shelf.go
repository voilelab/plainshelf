package server

import (
	"encoding/json"
	"net/http"
	"sort"
)

type ShelfInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// GET /api/shelves
func (app *App) HandleGetShelves(w http.ResponseWriter, _ *http.Request) {
	shelves := make([]ShelfInfo, 0, len(app.shelves))
	for _, shelf := range app.shelves {
		shelves = append(shelves, ShelfInfo{
			ID:   shelf.ID,
			Name: shelf.Name,
		})
	}
	sort.Slice(shelves, func(i, j int) bool { return shelves[i].ID < shelves[j].ID })

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	err := json.NewEncoder(w).Encode(shelves)
	if err != nil {
		app.Error("failed to encode response", "error", err)
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
		return
	}
}
