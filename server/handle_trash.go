package server

import (
	"net/http"
	"slices"

	"github.com/voilelab/plainshelf/internal/util"
	"github.com/voilelab/plainshelf/server/task"
	"github.com/voilelab/plainshelf/shelf"
)

type trashHandlers struct {
	*taskSubmitter
}

const TrashListingPartialHeader = "X-PlainShelf-Trash-Partial"

type TrashedBook struct {
	ID             string           `json:"id"`
	Title          string           `json:"title"`
	Authors        []string         `json:"authors,omitempty"`
	OriginalPath   string           `json:"original_path,omitempty"`
	OriginalFolder shelf.FolderPath `json:"original_folder,omitempty"`
	DeletedAt      util.JSONTime    `json:"deleted_at,omitzero"`
}

func (h *trashHandlers) trashBook(w http.ResponseWriter, r *http.Request) {
	shelfData, ok := h.resolveShelf(w, r)
	if !ok {
		return
	}

	bookID, ok := resolveBookID(w, r)
	if !ok {
		return
	}

	if _, ok := h.lookupVisibleBookListing(w, r, shelfData, bookID); !ok {
		return
	}

	if err := shelfData.MoveBookToTrash(bookID); err != nil {
		h.writeErr(w, r, err, "failed to trash book")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *trashHandlers) getTrashedBooks(w http.ResponseWriter, r *http.Request) {
	shelfData, ok := h.resolveShelf(w, r)
	if !ok {
		return
	}

	books, err := shelfData.ListTrashedBooks()
	if err != nil {
		h.writeErr(w, r, err, "failed to list trashed books")
		return
	}

	visibility := h.visibility(shelfData)
	resp := make([]TrashedBook, 0, len(books))
	for _, b := range books {
		if !visibility.allowsTrashed(b) {
			continue
		}
		resp = append(resp, TrashedBook{
			ID:             b.ID,
			Title:          b.Title,
			Authors:        slices.Clone(b.Authors),
			OriginalPath:   b.OriginalPath,
			OriginalFolder: slices.Clone(b.OriginalFolder),
			DeletedAt:      b.DeletedAt,
		})
	}

	if len(resp) < len(books) {
		w.Header().Set(TrashListingPartialHeader, "true")
	}

	h.writeJSON(w, http.StatusOK, resp)
}

func (h *trashHandlers) emptyTrash(w http.ResponseWriter, r *http.Request) {
	shelfData, ok := h.resolveShelf(w, r)
	if !ok {
		return
	}
	if h.rejectReadOnlyShelf(w, r, shelfData) {
		return
	}

	h.submitTaskChain(w, r,
		task.NewEmptyTrashChain(shelfData.ID, shelfData.Shelf, h.requestLogger(r)),
		"failed to schedule empty trash task")
}

func (h *trashHandlers) lookupTrashedBook(w http.ResponseWriter, r *http.Request, shelfData *shelf.ShelfData, bookID string) bool {
	book, err := shelfData.GetTrashedBook(bookID)
	if err != nil {
		h.writeErr(w, r, err, "failed to get trashed book")
		return false
	}

	if !h.visibility(shelfData).allowsTrashed(book) {
		h.writeErr(w, r, shelf.ErrTrashedBookNotFound, "failed to get trashed book")
		return false
	}

	return true
}

func (h *trashHandlers) restoreTrashedBook(w http.ResponseWriter, r *http.Request) {
	shelfData, ok := h.resolveShelf(w, r)
	if !ok {
		return
	}

	bookID, ok := resolveBookID(w, r)
	if !ok {
		return
	}

	if !h.lookupTrashedBook(w, r, shelfData, bookID) {
		return
	}

	if err := shelfData.RestoreTrashedBook(bookID); err != nil {
		h.writeErr(w, r, err, "failed to restore trashed book")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *trashHandlers) deleteTrashedBook(w http.ResponseWriter, r *http.Request) {
	shelfData, ok := h.resolveShelf(w, r)
	if !ok {
		return
	}

	bookID, ok := resolveBookID(w, r)
	if !ok {
		return
	}

	if !h.lookupTrashedBook(w, r, shelfData, bookID) {
		return
	}

	if err := shelfData.DeleteTrashedBook(bookID); err != nil {
		h.writeErr(w, r, err, "failed to permanently delete trashed book")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
