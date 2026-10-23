package server

import (
	"errors"
	"net/http"
	"strings"

	"github.com/voilelab/plainshelf/server/task"
	"github.com/voilelab/plainshelf/shelf"
)

type bookTransferHandlers struct {
	*taskSubmitter
}

type bookTransferRequest struct {
	Mode         string            `json:"mode"`
	TargetShelf  string            `json:"target_shelf"`
	TargetFolder *shelf.FolderPath `json:"target_folder"`
}

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

	if targetShelfID == sourceShelf.ID {
		http.Error(w, "source and target are the same shelf; copy within a shelf with .../copies and move between folders with PATCH .../books/{book_id}", http.StatusBadRequest)
		return
	}

	targetShelf, ok := h.shelves.GetShelf(targetShelfID)
	if !ok {
		http.Error(w, "target shelf not found", http.StatusNotFound)
		return
	}

	listing, ok := h.lookupVisibleBookListing(w, r, sourceShelf, bookID)
	if !ok {
		return
	}

	targetFolder := append(shelf.FolderPath(nil), listing.Folders...)
	if request.TargetFolder != nil {
		targetFolder = append(shelf.FolderPath(nil), (*request.TargetFolder)...)
	}
	if err := targetShelf.ValidateFolderPath(targetFolder); err != nil {
		h.writeErrStatus(w, r, err, "invalid target_folder", http.StatusBadRequest)
		return
	}

	if h.rejectReadOnlyShelf(w, r, targetShelf) {
		return
	}

	if operation == task.BookTransferOperationMove && h.rejectReadOnlyShelf(w, r, sourceShelf) {
		return
	}

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
