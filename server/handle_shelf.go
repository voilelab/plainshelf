package server

import (
	"cmp"
	"net/http"
	"slices"
	"time"

	"github.com/voilelab/plainshelf/shelf"
)

type shelfHandlers struct {
	*apiCore
}

type ShelfInfo struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	ReadOnly bool   `json:"read_only"`

	Kind         string            `json:"kind"`
	Capabilities ShelfCapabilities `json:"capabilities"`
	Offline      *ShelfOffline     `json:"offline,omitzero"`
}

type ShelfCapabilities struct {
	Rescan      bool `json:"rescan"`
	Download    bool `json:"download"`
	Fingerprint bool `json:"fingerprint"`
	CharCount   bool `json:"char_count"`
}

type ShelfOffline struct {
	SnapshotAt time.Time `json:"snapshot_at"`
}

func newShelfInfo(shelfData shelf.ShelfData) ShelfInfo {
	info := ShelfInfo{
		ID:       shelfData.ID,
		Name:     shelfData.Name,
		ReadOnly: shelfData.ReadOnly(),
		Kind:     "local",
		Capabilities: ShelfCapabilities{
			Rescan:      true,
			Download:    shelfData.CanDownload(),
			Fingerprint: true,
			CharCount:   true,
		},
	}
	if !shelfData.IsVirtual() {
		return info
	}

	info.Kind = "virtual"
	info.Capabilities.Fingerprint = false
	info.Capabilities.CharCount = false
	if at, ok := shelfData.SnapshotAt(); ok {
		info.Offline = &ShelfOffline{SnapshotAt: at.UTC()}
	}
	return info
}

func (h *shelfHandlers) getShelves(w http.ResponseWriter, _ *http.Request) {
	shelves := h.shelves.GetAllShelves()
	shelfInfos := make([]ShelfInfo, 0, len(shelves))
	for _, shelf := range shelves {
		shelfInfos = append(shelfInfos, newShelfInfo(shelf))
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
