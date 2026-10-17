package server

import (
	"cmp"
	"net/http"
	"slices"
	"time"

	"github.com/voilelab/plainshelf/shelf"
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

	// Kind, Capabilities and Offline are described in
	// docs/concepts/platform-capabilities.md#shelf-capabilities. read_only
	// stays the one answer to "may I show write controls".
	Kind         string            `json:"kind"`
	Capabilities ShelfCapabilities `json:"capabilities"`
	Offline      *ShelfOffline     `json:"offline,omitzero"`
}

// ShelfCapabilities is an object rather than flat booleans so a capability can
// grow detail later; a client treats an unknown member as absent.
type ShelfCapabilities struct {
	Rescan      bool `json:"rescan"`
	Download    bool `json:"download"`
	Fingerprint bool `json:"fingerprint"`
	CharCount   bool `json:"char_count"`
}

// ShelfOffline is present only on a virtual shelf that has a listing.
type ShelfOffline struct {
	// SnapshotAt is when the walk behind the listing began, shown as "last
	// updated" while the source is unreachable.
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

	// A virtual shelf is listed from the source's export, which carries no
	// character counts, and fingerprinting it would read every source.
	info.Kind = "virtual"
	info.Capabilities.Fingerprint = false
	info.Capabilities.CharCount = false
	if at, ok := shelfData.SnapshotAt(); ok {
		info.Offline = &ShelfOffline{SnapshotAt: at.UTC()}
	}
	return info
}

// GET /api/shelves
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
