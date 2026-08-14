package server

import (
	"net/http"

	"github.com/voilelab/plainshelf/internal/util"
	"github.com/voilelab/plainshelf/server/task"
	"github.com/voilelab/plainshelf/shelf"
)

type TrashedBook struct {
	ID            string        `json:"id"`
	Title         string        `json:"title"`
	Authors       []string      `json:"authors,omitempty"`
	OriginalPath  string        `json:"original_path,omitempty"`
	OriginalLayer shelf.Layers  `json:"original_layer,omitempty"`
	DeletedAt     util.JSONTime `json:"deleted_at,omitzero"`
}

// Serves both DELETE /api/shelves/{shelf_id}/books/{book_id} and
// POST /api/shelves/{shelf_id}/books/{book_id}/trash. Deleting a book is
// trashing it -- neither route erases anything, so they are one handler.
func (app *App) HandleAPITrashBook(w http.ResponseWriter, r *http.Request) {
	shelfData, ok := app.resolveShelf(w, r)
	if !ok {
		return
	}

	bookID, ok := resolveBookID(w, r)
	if !ok {
		return
	}

	if err := shelfData.MoveBookToTrash(bookID); err != nil {
		app.writeErr(w, err, "failed to trash book")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// GET /api/shelves/{shelf_id}/trash/books
func (app *App) HandleAPIGetTrashedBooks(w http.ResponseWriter, r *http.Request) {
	shelfData, ok := app.resolveShelf(w, r)
	if !ok {
		return
	}

	books, err := shelfData.ListTrashedBooks()
	if err != nil {
		app.writeErr(w, err, "failed to list trashed books")
		return
	}

	resp := make([]TrashedBook, 0, len(books))
	for _, b := range books {
		resp = append(resp, TrashedBook{
			ID:            b.ID,
			Title:         b.Title,
			Authors:       append([]string(nil), b.Authors...),
			OriginalPath:  b.OriginalPath,
			OriginalLayer: append(shelf.Layers(nil), b.OriginalLayer...),
			DeletedAt:     b.DeletedAt,
		})
	}

	app.writeJSON(w, http.StatusOK, resp)
}

// POST /api/shelves/{shelf_id}/trash/empty
func (app *App) HandleAPIEmptyTrash(w http.ResponseWriter, r *http.Request) {
	shelfData, ok := app.resolveShelf(w, r)
	if !ok {
		return
	}

	app.submitTaskChain(w,
		task.NewEmptyTrashChain(shelfData.ID, shelfData.Shelf, &app.Logger),
		"failed to schedule empty trash task")
}

// POST /api/shelves/{shelf_id}/trash/books/{book_id}/restore
func (app *App) HandleAPIRestoreTrashedBook(w http.ResponseWriter, r *http.Request) {
	shelfData, ok := app.resolveShelf(w, r)
	if !ok {
		return
	}

	bookID, ok := resolveBookID(w, r)
	if !ok {
		return
	}

	if err := shelfData.RestoreTrashedBook(bookID); err != nil {
		app.writeErr(w, err, "failed to restore trashed book")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// DELETE /api/shelves/{shelf_id}/trash/books/{book_id}
func (app *App) HandleAPIDeleteTrashedBook(w http.ResponseWriter, r *http.Request) {
	shelfData, ok := app.resolveShelf(w, r)
	if !ok {
		return
	}

	bookID, ok := resolveBookID(w, r)
	if !ok {
		return
	}

	if err := shelfData.DeleteTrashedBook(bookID); err != nil {
		app.writeErr(w, err, "failed to permanently delete trashed book")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
