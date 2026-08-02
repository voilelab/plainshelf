package server

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/voilelab/plainshelf/server/store"
)

// GET /api/shelves/{shelf_id}/marks/{book_id}
func (app *App) HandleAPIGetMarks(w http.ResponseWriter, r *http.Request) {
	shelfID, err := readShelfID(r)
	if err != nil {
		http.Error(w, "invalid shelf_id", http.StatusBadRequest)
		return
	}

	bookID, err := readBookID(r)
	if err != nil {
		http.Error(w, "invalid book_id", http.StatusBadRequest)
		return
	}

	mark, err := app.storeDB.GetBookmark(shelfID, bookID)
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

// POST /api/shelves/{shelf_id}/marks/{book_id}
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

	shelfID, err := readShelfID(r)
	if err != nil {
		http.Error(w, "invalid shelf_id", http.StatusBadRequest)
		return
	}
	if _, ok := app.shelfManager.GetShelf(shelfID); !ok {
		http.Error(w, "shelf not found", http.StatusNotFound)
		return
	}
	err = app.storeDB.SetBookmark(shelfID, bookID, mark)
	if err != nil {
		app.Error("failed to update marks", "error", err)
		http.Error(w, "failed to update marks", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
