package server

import (
	"net/http"
	"slices"

	"github.com/voilelab/plainshelf/internal/util"
	"github.com/voilelab/plainshelf/server/task"
	"github.com/voilelab/plainshelf/shelf"
)

// trashHandlers serves the trash routes. Emptying the trash is background
// work, so the group carries the submitter rather than plain apiCore.
type trashHandlers struct {
	*taskSubmitter
}

// TrashListingPartialHeader marks a trash listing the server did not answer in
// full, because show_nsfw withheld at least one book from it.
//
// Emptying the trash is deliberately not filtered — it is one command over the
// whole trash — so a client that quoted the listing's length as what the sweep
// will erase would understate it. This header is how the client knows to say
// "everything in the trash" instead of a number it cannot stand behind.
//
// It says only that something is missing, never what or how much. That is a
// narrow disclosure the filter otherwise avoids, and it is the accepted price
// of not asking the user to confirm one deletion and performing three.
const TrashListingPartialHeader = "X-PlainShelf-Trash-Partial"

type TrashedBook struct {
	ID             string           `json:"id"`
	Title          string           `json:"title"`
	Authors        []string         `json:"authors,omitempty"`
	OriginalPath   string           `json:"original_path,omitempty"`
	OriginalFolder shelf.FolderPath `json:"original_folder,omitempty"`
	DeletedAt      util.JSONTime    `json:"deleted_at,omitzero"`
}

// Serves both DELETE /api/shelves/{shelf_id}/books/{book_id} and
// POST /api/shelves/{shelf_id}/books/{book_id}/trash. Deleting a book is
// trashing it -- neither route erases anything, so they are one handler.
func (h *trashHandlers) trashBook(w http.ResponseWriter, r *http.Request) {
	shelfData, ok := h.resolveShelf(w, r)
	if !ok {
		return
	}

	bookID, ok := resolveBookID(w, r)
	if !ok {
		return
	}

	// Trashing goes through the same lookup as reading, so a book this request
	// cannot see cannot be deleted either: a 204 here would confirm the book
	// exists just as loudly as a 200 on the GET would.
	if _, ok := h.lookupBookListing(w, r, shelfData, bookID); !ok {
		return
	}

	if err := shelfData.MoveBookToTrash(bookID); err != nil {
		h.writeErr(w, r, err, "failed to trash book")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// GET /api/shelves/{shelf_id}/trash/books
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

// POST /api/shelves/{shelf_id}/trash/empty
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

// lookupTrashedBook is the gate every route naming one trashed book passes
// through, the counterpart to apiCore.lookupBookListing for the trash.
//
// A marked book this request may not see is answered as one that is not there,
// with the envelope an unknown ID gets: restoring or erasing it would otherwise
// confirm it exists, which is the fact the trash listing has just withheld.
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

// POST /api/shelves/{shelf_id}/trash/books/{book_id}/restore
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

// DELETE /api/shelves/{shelf_id}/trash/books/{book_id}
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
