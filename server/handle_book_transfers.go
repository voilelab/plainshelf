package server

import (
	"errors"
	"net/http"
	"strings"

	"github.com/voilelab/plainshelf/server/task"
	"github.com/voilelab/plainshelf/shelf"
)

// bookTransferHandlers serves the endpoint that copies or moves a single book
// from one shelf to another. It starts a background chain because the copy can be
// large and one end may be a network mount, so the request must not block on it.
type bookTransferHandlers struct {
	*taskSubmitter
}

// bookTransferRequest names the destination of a cross-shelf transfer. TargetFolder
// is a pointer so an omitted folder ("transfer in place") is distinct from an empty
// one (the target shelf's root), matching how the copy endpoint reads its folder.
type bookTransferRequest struct {
	Mode         string            `json:"mode"`
	TargetShelf  string            `json:"target_shelf"`
	TargetFolder *shelf.FolderPath `json:"target_folder"`
}

// POST /api/shelves/{shelf_id}/books/{book_id}/transfers
func (h *bookTransferHandlers) transferBook(w http.ResponseWriter, r *http.Request) {
	sourceShelf, ok := h.resolveShelf(w, r)
	if !ok {
		return
	}

	bookID, ok := resolveBookID(w, r)
	if !ok {
		return
	}

	var request bookTransferRequest
	if !decodeStrictJSON(w, r, &request) {
		return
	}

	operation := strings.TrimSpace(request.Mode)
	if operation != task.BookTransferOperationCopy && operation != task.BookTransferOperationMove {
		http.Error(w, `mode must be "copy" or "move"`, http.StatusBadRequest)
		return
	}

	targetShelfID := strings.TrimSpace(request.TargetShelf)
	if targetShelfID == "" {
		http.Error(w, "target_shelf is required", http.StatusBadRequest)
		return
	}

	// A source that is also the target belongs to the same-shelf endpoints, and
	// naming one shelf twice would deadlock the transfer on its own lock.
	if targetShelfID == sourceShelf.ID {
		http.Error(w, "source and target are the same shelf; copy within a shelf with .../copies and move between folders with PATCH .../books/{book_id}", http.StatusBadRequest)
		return
	}

	targetShelf, ok := h.shelves.GetShelf(targetShelfID)
	if !ok {
		http.Error(w, "target shelf not found", http.StatusNotFound)
		return
	}

	// The source book must exist before anything is scheduled: a transfer of a
	// missing book is a 404 now, not a task that fails later.
	listing, ok := h.lookupBookListing(w, r, sourceShelf, bookID)
	if !ok {
		return
	}

	// Default to the source book's own folder so a plain "move it to that shelf"
	// needs no folder in the body, mirroring the copy endpoint.
	targetFolder := append(shelf.FolderPath(nil), listing.Folders...)
	if request.TargetFolder != nil {
		targetFolder = append(shelf.FolderPath(nil), (*request.TargetFolder)...)
	}
	if err := targetShelf.ValidateFolderPath(targetFolder); err != nil {
		h.writeErrStatus(w, r, err, "invalid target_folder", http.StatusBadRequest)
		return
	}

	// A write to a read-only target is refused here rather than in a task the
	// caller would otherwise have to read to learn the work never happened.
	if h.rejectReadOnlyShelf(w, r, targetShelf) {
		return
	}

	// A move ends by deleting the source, so a read-only source is refused too:
	// scheduling it would publish the copy on the target and then fail to remove
	// the source, leaving the book on both shelves. A copy only reads the source,
	// so a read-only source is fine for it.
	if operation == task.BookTransferOperationMove && h.rejectReadOnlyShelf(w, r, sourceShelf) {
		return
	}

	// A move keeps the source ID, so refuse up front when the target already lists
	// a book under it; the transfer re-checks under lock (and also guards its
	// trash) before publishing.
	if operation == task.BookTransferOperationMove {
		if _, err := targetShelf.GetBook(bookID); err == nil {
			h.writeErr(w, r, shelf.ErrBookIDConflict, "failed to check the target shelf")
			return
		} else if !errors.Is(err, shelf.ErrBookNotFound) {
			h.writeErr(w, r, err, "failed to check the target shelf")
			return
		}
	}

	h.submitTaskChain(w, r,
		task.NewBookTransferChain(sourceShelf.ID, sourceShelf.Shelf, targetShelf.ID, targetShelf.Shelf, h.requestLogger(r), operation, bookID, targetFolder),
		"failed to schedule book transfer task")
}
