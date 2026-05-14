package txtlibsrv

import (
	"encoding/json"
	"net/http"
	"strings"
)

// GET /api/layers
func (app *App) HandleAPIGetLayers(w http.ResponseWriter, r *http.Request) {
	layers, err := app.shelf.GetAllLayers()
	if err != nil {
		http.Error(w, "failed to get layers", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	err = json.NewEncoder(w).Encode(layers)
	if err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
		return
	}
}

// POST /api/layers/{layer_path}
func (app *App) HandleAPICreateLayer(w http.ResponseWriter, r *http.Request) {
	layerPath := strings.TrimSpace(r.PathValue("layer_path"))
	if layerPath == "" {
		http.Error(w, "layer path cannot be empty", http.StatusBadRequest)
		return
	}

	layerParts := strings.Split(layerPath, "/")

	err := app.shelf.NewLayer(layerParts)
	if err != nil {
		http.Error(w, "failed to create layer", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// DELETE /api/layers/{layer_path}
func (app *App) HandleAPIDeleteLayer(w http.ResponseWriter, r *http.Request) {
	layerPath := strings.TrimSpace(r.PathValue("layer_path"))
	if layerPath == "" {
		http.Error(w, "layer path cannot be empty", http.StatusBadRequest)
		return
	}

	layerParts := strings.Split(layerPath, "/")

	err := app.shelf.DeleteLayer(layerParts)
	if err != nil {
		http.Error(w, "failed to delete layer", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
