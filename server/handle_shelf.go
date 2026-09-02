package server

import (
	"cmp"
	"net/http"
	"slices"
)

// shelfHandlers serves what a client needs to know about a shelf itself.
type shelfHandlers struct {
	*apiCore
}

// ShelfInfo is what a client is told about a shelf before it asks for its
// contents. read_only is here so the UI can drop the write affordances a
// read-only shelf has no use for, instead of offering them and answering 409
// when one is pressed. It reports the shelf as opened, so a server-wide
// read_only shows up on every shelf (applyAppReadOnly), not only on the ones
// configured that way.
type ShelfInfo struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	ReadOnly bool   `json:"read_only"`
}

// GET /api/shelves
func (h *shelfHandlers) getShelves(w http.ResponseWriter, _ *http.Request) {
	shelves := h.shelves.GetAllShelves()
	shelfInfos := make([]ShelfInfo, 0, len(shelves))
	for _, shelf := range shelves {
		shelfInfos = append(shelfInfos, ShelfInfo{
			ID:       shelf.ID,
			Name:     shelf.Name,
			ReadOnly: shelf.ReadOnly(),
		})
	}
	slices.SortFunc(shelfInfos, func(a, b ShelfInfo) int {
		return cmp.Compare(a.ID, b.ID)
	})

	h.writeJSON(w, http.StatusOK, shelfInfos)
}

type ShelfStatusResponse struct {
	Ready bool   `json:"ready"`
	Error string `json:"error,omitempty"`
}

// GET /api/shelves/{shelf_id}/status
func (h *shelfHandlers) getShelfStatus(w http.ResponseWriter, r *http.Request) {
	shelfData, ok := h.resolveShelf(w, r)
	if !ok {
		return
	}

	resp := ShelfStatusResponse{Ready: shelfData.IsReady()}
	if initErr := shelfData.InitErr(); initErr != nil {
		resp.Error = initErr.Error()
	}

	h.writeJSON(w, http.StatusOK, resp)
}
