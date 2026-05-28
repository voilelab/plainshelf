package server

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/voilelab/plainshelf/internal/util"
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

// POST /api/books/{book_id}/trash
func (app *App) HandleAPITrashBook(w http.ResponseWriter, r *http.Request) {
	bookID, err := readBookID(r)
	if err != nil {
		http.Error(w, "invalid book_id", http.StatusBadRequest)
		return
	}

	if err := app.shelf.MoveBookToTrash(bookID); err != nil {
		if errors.Is(err, shelf.ErrBookNotFound) {
			http.Error(w, "book not found", http.StatusNotFound)
			return
		}
		app.Error("failed to trash book", "error", err)
		http.Error(w, "failed to trash book", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// GET /api/trash/books
func (app *App) HandleAPIGetTrashedBooks(w http.ResponseWriter, r *http.Request) {
	books, err := app.shelf.ListTrashedBooks()
	if err != nil {
		app.Error("failed to list trashed books", "error", err)
		http.Error(w, "failed to list trashed books", http.StatusInternalServerError)
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

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		app.Error("failed to encode response", "error", err)
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
		return
	}
}

// POST /api/trash/books/{book_id}/restore
func (app *App) HandleAPIRestoreTrashedBook(w http.ResponseWriter, r *http.Request) {
	bookID, err := readBookID(r)
	if err != nil {
		http.Error(w, "invalid book_id", http.StatusBadRequest)
		return
	}

	if err := app.shelf.RestoreTrashedBook(bookID); err != nil {
		if errors.Is(err, shelf.ErrTrashedBookNotFound) {
			http.Error(w, "trashed book not found", http.StatusNotFound)
			return
		}
		app.Error("failed to restore trashed book", "error", err)
		http.Error(w, "failed to restore trashed book", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// DELETE /api/trash/books/{book_id}
func (app *App) HandleAPIDeleteTrashedBook(w http.ResponseWriter, r *http.Request) {
	bookID, err := readBookID(r)
	if err != nil {
		http.Error(w, "invalid book_id", http.StatusBadRequest)
		return
	}

	if err := app.shelf.DeleteTrashedBook(bookID); err != nil {
		if errors.Is(err, shelf.ErrTrashedBookNotFound) {
			http.Error(w, "trashed book not found", http.StatusNotFound)
			return
		}
		app.Error("failed to permanently delete trashed book", "error", err)
		http.Error(w, "failed to permanently delete trashed book", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
