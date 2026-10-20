package server

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"net/http"
	"strings"

	"github.com/voilelab/plainshelf/internal/util"
	"github.com/voilelab/plainshelf/shelf"
)

type folderHandlers struct {
	*apiCore
}

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

	folders, err = h.visibility(shelfData).filterFolders(folders)
	if err != nil {
		h.writeErr(w, r, err, "failed to get folders")
		return
	}

	h.writeJSON(w, http.StatusOK, folders)
}

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

type nsfwRevealConflict struct {
	Error   string `json:"error"`
	Message string `json:"message"`

	HiddenBooks int `json:"hidden_books"`
}

const nsfwRevealConflictKind = "nsfw_reveal_requires_confirmation"

func confirmedReveal(r *http.Request) bool {
	return r.URL.Query().Get("confirm") == "1"
}

func (c *apiCore) validFolderPaths(
	w http.ResponseWriter, r *http.Request, shelfData *shelf.ShelfData, paths ...shelf.FolderPath,
) bool {
	for _, path := range paths {
		if err := shelfData.ValidateFolderPath(path); err != nil {
			c.writeErrStatus(w, r, err, "invalid folder name", http.StatusBadRequest)
			return false
		}
	}
	return true
}

func (c *apiCore) refreshBeforeReveal(r *http.Request, shelfData *shelf.ShelfData) {
	if confirmedReveal(r) || !c.visibility(shelfData).couldReveal() {
		return
	}
	bestEffortRescanForPreflight(shelfData)
}

func (c *apiCore) refuseUnconfirmedReveal(
	w http.ResponseWriter, r *http.Request, shelfData *shelf.ShelfData, change folderChange,
) bool {
	if confirmedReveal(r) {
		return false
	}

	reveal, err := c.visibility(shelfData).revealedBy(change)
	if err != nil {
		c.writeErr(w, r, err, "failed to read the shelf")
		return true
	}
	if !reveal.any() {
		return false
	}

	c.writeJSON(w, http.StatusConflict, nsfwRevealConflict{
		Error: nsfwRevealConflictKind,
		Message: "this folder holds content the shelf marks as adult and the show_nsfw setting is hiding; " +
			"the change would take it out of the marked folder and make it visible. Retry with confirm=1 to go ahead.",
		HiddenBooks: reveal.Books,
	})
	return true
}

type renameFolderRequest struct {
	Name string `json:"name"`
}

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

	renamed := append(append(shelf.FolderPath(nil), folderParts[:len(folderParts)-1]...), newName)

	if !h.validFolderPaths(w, r, shelfData, folderParts, renamed) {
		return
	}

	if h.rejectReadOnlyShelf(w, r, shelfData) {
		return
	}

	h.refreshBeforeReveal(r, shelfData)
	if h.refuseUnconfirmedReveal(w, r, shelfData, folderMovedTo(folderParts, renamed)) {
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

	moved := append(append(shelf.FolderPath(nil), req.TargetFolder...), req.Folder[len(req.Folder)-1])

	if !h.validFolderPaths(w, r, shelfData, req.Folder, req.TargetFolder) {
		return
	}

	if h.rejectReadOnlyShelf(w, r, shelfData) {
		return
	}

	h.refreshBeforeReveal(r, shelfData)
	if h.refuseUnconfirmedReveal(w, r, shelfData, folderMovedTo(req.Folder, moved)) {
		return
	}

	if err := shelfData.MoveFolder(req.Folder, req.TargetFolder); err != nil {
		h.writeErrStatus(w, r, err, "failed to move folder", http.StatusConflict)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

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

type folderMetaResponse struct {
	Icons []string `json:"icons"`

	Folders map[string]folderMetaJSON `json:"folders"`
}

type folderMetaJSON struct {
	SortKey *float64 `json:"sort_key,omitzero"`

	Icon string `json:"icon"`

	Version string `json:"version"`

	ReadOnly bool `json:"read_only"`
}

func (h *folderHandlers) getFolderMeta(w http.ResponseWriter, r *http.Request) {
	shelfData, ok := h.resolveShelf(w, r)
	if !ok {
		return
	}

	visible, err := h.visibleFolders(shelfData)
	if err != nil {
		h.writeErr(w, r, err, "failed to get folder metadata")
		return
	}
	metas, err := shelfData.ListFolderMeta()
	if err != nil {
		h.writeErr(w, r, err, "failed to get folder metadata")
		return
	}

	resp := folderMetaResponse{Icons: shelf.FolderIcons(), Folders: map[string]folderMetaJSON{}}
	for name, meta := range metas {

		if !visible[name] {
			continue
		}
		entry := folderMetaJSON{Icon: meta.DisplayIcon(), Version: meta.Version, ReadOnly: meta.ReadOnly}
		if meta.HasSortKey {
			key := meta.SortKey
			entry.SortKey = &key
		}
		resp.Folders[name] = entry
	}
	h.writeJSON(w, http.StatusOK, resp)
}

type setFolderMetaRequest struct {
	SortKey jsontext.Value `json:"sort_key"`

	Icon jsontext.Value `json:"icon"`

	Version string `json:"version"`
}

func (h *folderHandlers) setFolderMeta(w http.ResponseWriter, r *http.Request) {
	shelfData, ok := h.resolveShelf(w, r)
	if !ok {
		return
	}

	folderParts, err := readFolderParts(r)
	if err != nil || len(folderParts) == 0 || strings.Join(folderParts, "") == "" {
		http.Error(w, "invalid folder path", http.StatusBadRequest)
		return
	}

	var req setFolderMetaRequest
	if err := json.UnmarshalRead(r.Body, &req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	var sortKey float64
	var icon string
	if len(req.SortKey) > 0 {
		if len(req.Icon) > 0 || strings.TrimSpace(string(req.SortKey)) == "null" || json.Unmarshal(req.SortKey, &sortKey) != nil {
			http.Error(w, "send a finite sort_key alone; null and combined icon writes are not supported", http.StatusBadRequest)
			return
		}
	} else if len(req.Icon) > 0 && json.Unmarshal(req.Icon, &icon) != nil {
		http.Error(w, "invalid icon", http.StatusBadRequest)
		return
	}

	if !h.validFolderPaths(w, r, shelfData, folderParts) {
		return
	}
	if h.rejectReadOnlyShelf(w, r, shelfData) {
		return
	}

	visible, err := h.visibleFolders(shelfData)
	if err != nil {
		h.writeErr(w, r, err, "failed to set folder metadata")
		return
	}
	if !visible[shelf.FolderPath(folderParts).String()] {
		h.writeErr(w, r, shelf.ErrFolderNotFound, "failed to set folder metadata")
		return
	}

	if len(req.SortKey) > 0 {
		err = shelfData.SetFolderSortKey(folderParts, sortKey, req.Version)
	} else {
		err = shelfData.SetFolderIcon(folderParts, icon, req.Version)
	}
	if err != nil {
		h.writeErr(w, r, err, "failed to set folder metadata")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *folderHandlers) visibleFolders(shelfData *shelf.ShelfData) (map[string]bool, error) {
	folders, err := shelfData.GetAllFolders()
	if err != nil {
		return nil, util.Errorf("%w", err)
	}
	folders, err = h.visibility(shelfData).filterFolders(folders)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	visible := make(map[string]bool, len(folders))
	for _, folder := range folders {
		visible[folder.String()] = true
	}
	return visible, nil
}
