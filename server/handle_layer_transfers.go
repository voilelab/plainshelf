package server

import (
	"net/http"
	"sort"
	"strings"

	"github.com/voilelab/plainshelf/server/task"
	"github.com/voilelab/plainshelf/shelf"
)

// layerTransferHandlers serves the endpoint that copies or moves a whole layer -
// the folder and every book and sub-folder beneath it - from one shelf to
// another. Like the single-book transfer it starts a background chain, because a
// layer can hold hundreds of megabytes and one end may be a network mount, so the
// request must not block on the copy.
type layerTransferHandlers struct {
	*taskSubmitter
}

// layerTransferRequest names the source layer and the destination of a
// cross-shelf layer transfer. It mirrors the single-book transfer body, with the
// source layer moved into the body because a layer path holds slashes and does
// not sit cleanly in a path segment - the same reason the same-shelf layer move
// carries its layer in the body. TargetLayer is a pointer so an omitted layer
// (transfer keeping the same layer name) is distinct from an empty one.
type layerTransferRequest struct {
	Mode        string        `json:"mode"`
	SourceLayer shelf.Layers  `json:"source_layer"`
	TargetShelf string        `json:"target_shelf"`
	TargetLayer *shelf.Layers `json:"target_layer"`
}

// layerTransferConflict is the 409 body a pre-flight refusal answers with. Error
// names which conflict it is so the frontend can tell the two apart, and a book-ID
// conflict lists every colliding ID at once so the user sees the whole set rather
// than one failed transfer at a time.
type layerTransferConflict struct {
	Error              string   `json:"error"`
	Message            string   `json:"message"`
	ConflictingBookIDs []string `json:"conflicting_book_ids,omitempty"`
}

const (
	layerTransferConflictLayer  = "target_layer_conflict"
	layerTransferConflictBookID = "book_id_conflict"
)

// POST /api/shelves/{shelf_id}/layer-transfers
func (h *layerTransferHandlers) transferLayer(w http.ResponseWriter, r *http.Request) {
	sourceShelf, ok := h.resolveShelf(w, r)
	if !ok {
		return
	}

	var request layerTransferRequest
	if !decodeStrictJSON(w, r, &request) {
		return
	}

	operation := strings.TrimSpace(request.Mode)
	if operation != task.BookTransferOperationCopy && operation != task.BookTransferOperationMove {
		http.Error(w, `mode must be "copy" or "move"`, http.StatusBadRequest)
		return
	}

	sourceLayer := append(shelf.Layers(nil), request.SourceLayer...)
	if len(sourceLayer) == 0 {
		http.Error(w, "source_layer is required", http.StatusBadRequest)
		return
	}
	if err := shelf.ValidateLayers(sourceLayer); err != nil {
		h.writeErrStatus(w, err, "invalid source_layer", http.StatusBadRequest)
		return
	}

	targetShelfID := strings.TrimSpace(request.TargetShelf)
	if targetShelfID == "" {
		http.Error(w, "target_shelf is required", http.StatusBadRequest)
		return
	}

	// A source that is also the target belongs to the same-shelf layer endpoints,
	// and naming one shelf twice would deadlock the transfer on its own lock.
	if targetShelfID == sourceShelf.ID {
		http.Error(w, "source and target are the same shelf; move a layer within a shelf with POST .../layer-moves", http.StatusBadRequest)
		return
	}

	targetShelf, ok := h.shelves.GetShelf(targetShelfID)
	if !ok {
		http.Error(w, "target shelf not found", http.StatusNotFound)
		return
	}

	// Default to the source layer's own name so a plain "move that folder to the
	// other shelf" needs no target layer in the body, mirroring the single-book
	// endpoint's default to the source book's layer.
	targetLayer := append(shelf.Layers(nil), sourceLayer...)
	if request.TargetLayer != nil {
		targetLayer = append(shelf.Layers(nil), (*request.TargetLayer)...)
	}
	if len(targetLayer) == 0 {
		http.Error(w, "target_layer cannot be the root layer", http.StatusBadRequest)
		return
	}
	if err := shelf.ValidateLayers(targetLayer); err != nil {
		h.writeErrStatus(w, err, "invalid target_layer", http.StatusBadRequest)
		return
	}

	// A write to a read-only target is refused here rather than in a task the
	// caller would otherwise have to read to learn the work never happened.
	if h.rejectReadOnlyShelf(w, targetShelf) {
		return
	}

	// A move ends by deleting from the source, so a read-only source is refused
	// too. A copy only reads the source, so a read-only source is fine for it.
	if operation == task.BookTransferOperationMove && h.rejectReadOnlyShelf(w, sourceShelf) {
		return
	}

	// Force a fresh scan of both shelves before resolving anything. A plain listing
	// only schedules a refresh and answers from the cache, so a book or layer added
	// to a shared or network shelf since its last scan would be missed - the plan
	// would skip those books and the conflict checks would not see an externally
	// created layer. This mirrors the shelf-level layer-transfer preflight, which
	// forces the same scan. ErrRescanInProgress means another scan is already
	// refreshing the cache, so the following reads are as fresh as one here.
	rescanForTransfer(sourceShelf)
	rescanForTransfer(targetShelf)

	// Resolve the transfer plan from a single snapshot of the source: the layers
	// to reproduce and the books to carry. The same snapshot screens for conflicts
	// below, so the 409 check and the scheduled work see the same set.
	sourceLayers, err := sourceShelf.GetAllLayers()
	if err != nil {
		h.writeErr(w, err, "failed to read the source shelf")
		return
	}
	subLayers := layersUnder(sourceLayers, sourceLayer)

	listings, err := sourceShelf.ListBooksWithCharCount()
	if err != nil {
		h.writeErr(w, err, "failed to read the source shelf")
		return
	}
	var books []task.LayerTransferBook
	for _, listing := range listings {
		if layerHasPrefix(listing.Layers, sourceLayer) {
			books = append(books, task.LayerTransferBook{
				ID:          listing.Book.ID(),
				SourceLayer: append(shelf.Layers(nil), listing.Layers...),
			})
		}
	}

	// A transfer of a layer that holds no folder and no book is a 404 now, not a
	// task that copies nothing later - the same way a missing book is a 404.
	if len(subLayers) == 0 && len(books) == 0 {
		http.Error(w, "source layer not found", http.StatusNotFound)
		return
	}

	// A layer transfer builds the destination subtree fresh, so it refuses to land
	// on a layer the target already holds rather than merging into it.
	targetLayers, err := targetShelf.GetAllLayers()
	if err != nil {
		h.writeErr(w, err, "failed to read the target shelf")
		return
	}
	if len(layersUnder(targetLayers, targetLayer)) > 0 {
		h.writeJSON(w, http.StatusConflict, layerTransferConflict{
			Error:   layerTransferConflictLayer,
			Message: "the target shelf already holds a layer with this name",
		})
		return
	}

	// A move keeps every book's ID, so the target must hold none of them. Collect
	// the whole colliding set rather than stopping at the first, so the frontend
	// can list them all. A copy draws fresh IDs, so it never collides this way.
	if operation == task.BookTransferOperationMove {
		existing, err := targetBookIDs(targetShelf)
		if err != nil {
			h.writeErr(w, err, "failed to read the target shelf")
			return
		}
		var conflicts []string
		for _, book := range books {
			if existing[book.ID] {
				conflicts = append(conflicts, book.ID)
			}
		}
		if len(conflicts) > 0 {
			sort.Strings(conflicts)
			h.writeJSON(w, http.StatusConflict, layerTransferConflict{
				Error:              layerTransferConflictBookID,
				Message:            "the target shelf already holds books with these IDs; the move would overwrite them",
				ConflictingBookIDs: conflicts,
			})
			return
		}
	}

	h.submitTaskChain(w,
		task.NewLayerTransferChain(sourceShelf.ID, sourceShelf.Shelf, targetShelf.ID, targetShelf.Shelf, h.Logger, operation, sourceLayer, targetLayer, books, subLayers),
		"failed to schedule layer transfer task")
}

// layersUnder returns every layer in all that is root itself or sits beneath it.
// An empty result means the target holds nothing under that path.
func layersUnder(all []shelf.Layers, root shelf.Layers) []shelf.Layers {
	var under []shelf.Layers
	for _, l := range all {
		if layerHasPrefix(l, root) {
			under = append(under, append(shelf.Layers(nil), l...))
		}
	}
	return under
}

// layerHasPrefix reports whether layer is prefix itself or sits beneath it.
func layerHasPrefix(layer, prefix shelf.Layers) bool {
	if len(layer) < len(prefix) {
		return false
	}
	for i := range prefix {
		if layer[i] != prefix[i] {
			return false
		}
	}
	return true
}

// rescanForTransfer forces a shelf to rebuild its book cache now, so the plan and
// the conflict checks read an authoritative listing rather than a throttled one.
// It is best-effort: a rescan already in progress is refreshing the cache anyway,
// and any other failure is left for the reads that follow to surface, since they
// answer with a real error a caller can act on.
func rescanForTransfer(shelfData *shelf.ShelfData) {
	_, _ = shelfData.Rescan()
}

// targetBookIDs is the set of book IDs the target shelf already holds, whether
// listed or sitting in its trash - the two places a move's kept ID could collide.
func targetBookIDs(target *shelf.ShelfData) (map[string]bool, error) {
	ids := map[string]bool{}

	books, err := target.ListBooks()
	if err != nil {
		return nil, err
	}
	for _, book := range books {
		ids[book.ID()] = true
	}

	trashed, err := target.ListTrashedBookIDs()
	if err != nil {
		return nil, err
	}
	for _, id := range trashed {
		ids[id] = true
	}

	return ids, nil
}
