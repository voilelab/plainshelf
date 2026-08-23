package server

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/voilelab/plainshelf/shelf"
)

// layerHandlers serves the folder tree the books are filed under.
type layerHandlers struct {
	*apiCore
}

// GET /api/shelves/{shelf_id}/layers
func (h *layerHandlers) getLayers(w http.ResponseWriter, r *http.Request) {
	shelfData, ok := h.resolveShelf(w, r)
	if !ok {
		return
	}

	layers, err := shelfData.GetAllFolders()
	if err != nil {
		h.writeErr(w, err, "failed to get layers")
		return
	}

	h.writeJSON(w, http.StatusOK, layers)
}

// POST /api/shelves/{shelf_id}/layers/{layer_path}
func (h *layerHandlers) createLayer(w http.ResponseWriter, r *http.Request) {
	shelfData, ok := h.resolveShelf(w, r)
	if !ok {
		return
	}

	layerParts, err := readLayerParts(r)
	if err != nil {
		http.Error(w, "invalid layer path", http.StatusBadRequest)
		return
	}

	parent := append(shelf.FolderPath(nil), layerParts[:len(layerParts)-1]...)
	name := layerParts[len(layerParts)-1]
	if err := shelfData.NewFolder(parent, name); err != nil {
		h.writeErr(w, err, "failed to create layer")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

type renameLayerRequest struct {
	Name string `json:"name"`
}

// PATCH /api/shelves/{shelf_id}/layers/{layer_path}
func (h *layerHandlers) renameLayer(w http.ResponseWriter, r *http.Request) {
	shelfData, ok := h.resolveShelf(w, r)
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

	if err := shelfData.RenameFolder(layerParts, newName); err != nil {
		h.writeErrStatus(w, err, "failed to rename layer", http.StatusConflict)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

type moveLayerRequest struct {
	Layer       shelf.FolderPath `json:"layer"`
	TargetLayer shelf.FolderPath `json:"target_layer"`
}

// POST /api/shelves/{shelf_id}/layer-moves
func (h *layerHandlers) moveLayer(w http.ResponseWriter, r *http.Request) {
	shelfData, ok := h.resolveShelf(w, r)
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

	if err := shelfData.MoveFolder(req.Layer, req.TargetLayer); err != nil {
		h.writeErrStatus(w, err, "failed to move layer", http.StatusConflict)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// DELETE /api/shelves/{shelf_id}/layers/{layer_path}
func (h *layerHandlers) deleteLayer(w http.ResponseWriter, r *http.Request) {
	shelfData, ok := h.resolveShelf(w, r)
	if !ok {
		return
	}

	layerParts, err := readLayerParts(r)
	if err != nil {
		http.Error(w, "invalid layer path", http.StatusBadRequest)
		return
	}

	if err := shelfData.DeleteFolder(layerParts); err != nil {
		h.writeErr(w, err, "failed to delete layer")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
