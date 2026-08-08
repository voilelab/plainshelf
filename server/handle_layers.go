package server

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/voilelab/plainshelf/shelf"
)

// GET /api/shelves/{shelf_id}/layers
func (app *App) HandleAPIGetLayers(w http.ResponseWriter, r *http.Request) {
	shelfData, ok := app.resolveShelf(w, r)
	if !ok {
		return
	}

	layers, err := shelfData.GetAllLayers()
	if err != nil {
		app.Error("failed to get layers", "error", err)
		http.Error(w, "failed to get layers", http.StatusInternalServerError)
		return
	}

	app.writeJSON(w, http.StatusOK, layers)
}

// POST /api/shelves/{shelf_id}/layers/{layer_path}
func (app *App) HandleAPICreateLayer(w http.ResponseWriter, r *http.Request) {
	shelfData, ok := app.resolveShelf(w, r)
	if !ok {
		return
	}

	layerParts, err := readLayerParts(r)
	if err != nil {
		http.Error(w, "invalid layer path", http.StatusBadRequest)
		return
	}

	if err := shelfData.NewLayer(layerParts); err != nil {
		app.Error("failed to create layer", "error", err)
		http.Error(w, "failed to create layer", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

type renameLayerRequest struct {
	Name string `json:"name"`
}

// PATCH /api/shelves/{shelf_id}/layers/{layer_path}
func (app *App) HandleAPIRenameLayer(w http.ResponseWriter, r *http.Request) {
	shelfData, ok := app.resolveShelf(w, r)
	if !ok {
		return
	}

	layerParts, err := readLayerParts(r)
	if err != nil || len(layerParts) == 0 || strings.Join(layerParts, "") == "" {
		http.Error(w, "invalid layer path", http.StatusBadRequest)
		return
	}

	var req renameLayerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	newName := strings.TrimSpace(req.Name)
	if newName == "" || strings.Contains(newName, "/") {
		http.Error(w, "invalid layer name", http.StatusBadRequest)
		return
	}

	newLayer := append(shelf.Layers(nil), layerParts[:len(layerParts)-1]...)
	newLayer = append(newLayer, newName)
	if err := shelfData.RenameLayer(layerParts, newLayer); err != nil {
		app.Error("failed to rename layer", "error", err)
		http.Error(w, "failed to rename layer", http.StatusConflict)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

type moveLayerRequest struct {
	Layer       shelf.Layers `json:"layer"`
	TargetLayer shelf.Layers `json:"target_layer"`
}

// POST /api/shelves/{shelf_id}/layer-moves
func (app *App) HandleAPIMoveLayer(w http.ResponseWriter, r *http.Request) {
	shelfData, ok := app.resolveShelf(w, r)
	if !ok {
		return
	}

	var req moveLayerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if len(req.Layer) == 0 {
		http.Error(w, "invalid layer path", http.StatusBadRequest)
		return
	}

	if err := shelfData.MoveLayer(req.Layer, req.TargetLayer); err != nil {
		app.Error("failed to move layer", "error", err)
		http.Error(w, "failed to move layer", http.StatusConflict)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// DELETE /api/shelves/{shelf_id}/layers/{layer_path}
func (app *App) HandleAPIDeleteLayer(w http.ResponseWriter, r *http.Request) {
	shelfData, ok := app.resolveShelf(w, r)
	if !ok {
		return
	}

	layerParts, err := readLayerParts(r)
	if err != nil {
		http.Error(w, "invalid layer path", http.StatusBadRequest)
		return
	}

	if err := shelfData.DeleteLayer(layerParts); err != nil {
		app.Error("failed to delete layer", "error", err)
		http.Error(w, "failed to delete layer", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
