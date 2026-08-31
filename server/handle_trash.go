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

	if err := shelfData.MoveBookToTrash(bookID); err != nil {
		h.writeErr(w, err, "failed to trash book")
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
		h.writeErr(w, err, "failed to list trashed books")
		return
	}

	resp := make([]TrashedBook, 0, len(books))
	for _, b := range books {
		resp = append(resp, TrashedBook{
			ID:             b.ID,
			Title:          b.Title,
			Authors:        slices.Clone(b.Authors),
			OriginalPath:   b.OriginalPath,
			OriginalFolder: slices.Clone(b.OriginalFolder),
			DeletedAt:      b.DeletedAt,
		})
	}

	h.writeJSON(w, http.StatusOK, resp)
}

// POST /api/shelves/{shelf_id}/trash/empty
func (h *trashHandlers) emptyTrash(w http.ResponseWriter, r *http.Request) {
	shelfData, ok := h.resolveShelf(w, r)
	if !ok {
		return
	}
	if h.rejectReadOnlyShelf(w, shelfData) {
		return
	}

	h.submitTaskChain(w,
		task.NewEmptyTrashChain(shelfData.ID, shelfData.Shelf, h.Logger),
		"failed to schedule empty trash task")
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

	if err := shelfData.RestoreTrashedBook(bookID); err != nil {
		h.writeErr(w, err, "failed to restore trashed book")
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

	if err := shelfData.DeleteTrashedBook(bookID); err != nil {
		h.writeErr(w, err, "failed to permanently delete trashed book")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
