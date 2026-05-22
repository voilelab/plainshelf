package server

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/voilelab/plainshelf/server/store"
)

// GET /api/marks/{book_id}
func (app *App) HandleAPIGetMarks(w http.ResponseWriter, r *http.Request) {
	bookID, err := readBookID(r)
	if err != nil {
		http.Error(w, "invalid book_id", http.StatusBadRequest)
		return
	}

	mark, err := app.storeDB.GetBookmark(bookID)
	if err != nil {
		app.Error("failed to get marks", "error", err)
		http.Error(w, "failed to get marks", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	err = json.NewEncoder(w).Encode(mark)
	if err != nil {
		app.Error("failed to encode response", "error", err)
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
		return
	}
}

// POST /api/marks/{book_id}
func (app *App) HandleAPIUpdateMarks(w http.ResponseWriter, r *http.Request) {
	bookID, err := readBookID(r)
	if err != nil {
		http.Error(w, "invalid book_id", http.StatusBadRequest)
		return
	}

	var mark store.Bookmark
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&mark); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	err = app.storeDB.SetBookmark(bookID, mark)
	if err != nil {
		app.Error("failed to update marks", "error", err)
		http.Error(w, "failed to update marks", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// GET /api/read_history
func (app *App) HandleAPIGetReadHistory(w http.ResponseWriter, r *http.Request) {
	history, err := app.storeDB.GetReadHistory()
	if err != nil {
		app.Error("failed to get read history", "error", err)
		http.Error(w, "failed to get read history", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	err = json.NewEncoder(w).Encode(history)
	if err != nil {
		app.Error("failed to encode response", "error", err)
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
		return
	}
}

// POST /api/read_history?book_id={book_id}
func (app *App) HandleAPIUpdateReadHistory(w http.ResponseWriter, r *http.Request) {
	bookID := r.URL.Query().Get("book_id")
	if bookID == "" {
		http.Error(w, "missing book_id", http.StatusBadRequest)
		return
	}

	err := app.storeDB.AddToReadHistory(bookID)
	if err != nil {
		app.Error("failed to update read history", "error", err)
		http.Error(w, "failed to update read history", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// DELETE /api/read_history
func (app *App) HandleAPIClearReadHistory(w http.ResponseWriter, r *http.Request) {
	err := app.storeDB.SetReadHistory([]string{})
	if err != nil {
		app.Error("failed to clear read history", "error", err)
		http.Error(w, "failed to clear read history", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
