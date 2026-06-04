package server

import (
	"encoding/json"
	"net/http"
)

// GET /api/shelves/{shelf_id}/layers
func (app *App) HandleAPIGetLayers(w http.ResponseWriter, r *http.Request) {
	shelfID, err := readShelfID(r)
	if err != nil {
		http.Error(w, "invalid shelf ID", http.StatusBadRequest)
		return
	}

	shelf, exists := app.shelves[shelfID]
	if !exists {
		http.Error(w, "shelf not found", http.StatusNotFound)
		return
	}

	layers, err := shelf.GetAllLayers()
	if err != nil {
		app.Error("failed to get layers", "error", err)
		http.Error(w, "failed to get layers", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	err = json.NewEncoder(w).Encode(layers)
	if err != nil {
		app.Error("failed to encode response", "error", err)
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
		return
	}
}

// POST /api/shelves/{shelf_id}/layers/{layer_path}
func (app *App) HandleAPICreateLayer(w http.ResponseWriter, r *http.Request) {
	shelfID, err := readShelfID(r)
	if err != nil {
		http.Error(w, "invalid shelf ID", http.StatusBadRequest)
		return
	}

	shelf, exists := app.shelves[shelfID]
	if !exists {
		http.Error(w, "shelf not found", http.StatusNotFound)
		return
	}

	layerParts, err := readLayerParts(r)
	if err != nil {
		http.Error(w, "invalid layer path", http.StatusBadRequest)
		return
	}

	err = shelf.NewLayer(layerParts)
	if err != nil {
		app.Error("failed to create layer", "error", err)
		http.Error(w, "failed to create layer", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// DELETE /api/shelves/{shelf_id}/layers/{layer_path}
func (app *App) HandleAPIDeleteLayer(w http.ResponseWriter, r *http.Request) {
	shelfID, err := readShelfID(r)
	if err != nil {
		http.Error(w, "invalid shelf ID", http.StatusBadRequest)
		return
	}

	shelf, exists := app.shelves[shelfID]
	if !exists {
		http.Error(w, "shelf not found", http.StatusNotFound)
		return
	}

	layerParts, err := readLayerParts(r)
	if err != nil {
		http.Error(w, "invalid layer path", http.StatusBadRequest)
		return
	}

	err = shelf.DeleteLayer(layerParts)
	if err != nil {
		app.Error("failed to delete layer", "error", err)
		http.Error(w, "failed to delete layer", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
