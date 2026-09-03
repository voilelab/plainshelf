package server

import (
	"encoding/json/v2"
	"net/http"
	"strings"

	"github.com/voilelab/plainshelf/shelf"
)

// folderHandlers serves the folder tree the books are filed under.
type folderHandlers struct {
	*apiCore
}

// GET /api/shelves/{shelf_id}/folders
func (h *folderHandlers) getFolders(w http.ResponseWriter, r *http.Request) {
	shelfData, ok := h.resolveShelf(w, r)
	if !ok {
		return
	}

	folders, err := shelfData.GetAllFolders()
	if err != nil {
		h.writeErr(w, r, err, "failed to get folders")
		return
	}

	h.writeJSON(w, http.StatusOK, folders)
}

// POST /api/shelves/{shelf_id}/folders/{folder_path}
func (h *folderHandlers) createFolder(w http.ResponseWriter, r *http.Request) {
	shelfData, ok := h.resolveShelf(w, r)
	if !ok {
		return
	}

	folderParts, err := readFolderParts(r)
	if err != nil {
		http.Error(w, "invalid folder path", http.StatusBadRequest)
		return
	}

	parent := append(shelf.FolderPath(nil), folderParts[:len(folderParts)-1]...)
	name := folderParts[len(folderParts)-1]
	if err := shelfData.NewFolder(parent, name); err != nil {
		h.writeErr(w, r, err, "failed to create folder")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

type renameFolderRequest struct {
	Name string `json:"name"`
}

// PATCH /api/shelves/{shelf_id}/folders/{folder_path}
func (h *folderHandlers) renameFolder(w http.ResponseWriter, r *http.Request) {
	shelfData, ok := h.resolveShelf(w, r)
	if !ok {
		return
	}

	folderParts, err := readFolderParts(r)
	if err != nil || len(folderParts) == 0 || strings.Join(folderParts, "") == "" {
		http.Error(w, "invalid folder path", http.StatusBadRequest)
		return
	}

	var req renameFolderRequest
	if err := json.UnmarshalRead(r.Body, &req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	newName := strings.TrimSpace(req.Name)
	if newName == "" || strings.Contains(newName, "/") {
		http.Error(w, "invalid folder name", http.StatusBadRequest)
		return
	}

	if err := shelfData.RenameFolder(folderParts, newName); err != nil {
		h.writeErrStatus(w, r, err, "failed to rename folder", http.StatusConflict)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

type moveFolderRequest struct {
	Folder       shelf.FolderPath `json:"folder"`
	TargetFolder shelf.FolderPath `json:"target_folder"`
}

// POST /api/shelves/{shelf_id}/folder-moves
func (h *folderHandlers) moveFolder(w http.ResponseWriter, r *http.Request) {
	shelfData, ok := h.resolveShelf(w, r)
	if !ok {
		return
	}

	var req moveFolderRequest
	if err := json.UnmarshalRead(r.Body, &req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if len(req.Folder) == 0 {
		http.Error(w, "invalid folder path", http.StatusBadRequest)
		return
	}

	if err := shelfData.MoveFolder(req.Folder, req.TargetFolder); err != nil {
		h.writeErrStatus(w, r, err, "failed to move folder", http.StatusConflict)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// DELETE /api/shelves/{shelf_id}/folders/{folder_path}
func (h *folderHandlers) deleteFolder(w http.ResponseWriter, r *http.Request) {
	shelfData, ok := h.resolveShelf(w, r)
	if !ok {
		return
	}

	folderParts, err := readFolderParts(r)
	if err != nil {
		http.Error(w, "invalid folder path", http.StatusBadRequest)
		return
	}

	if err := shelfData.DeleteFolder(folderParts); err != nil {
		h.writeErr(w, r, err, "failed to delete folder")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
