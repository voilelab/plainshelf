package server

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/voilelab/plainshelf/server/bookmark"
)

// GET /api/marks/{book_id}
func (app *App) HandleAPIGetMarks(w http.ResponseWriter, r *http.Request) {
	bookID, err := readBookID(r)
	if err != nil {
		http.Error(w, "invalid book_id", http.StatusBadRequest)
		return
	}

	mark, err := app.markLib.Get(bookID)
	if err != nil {
		http.Error(w, "failed to get marks", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	err = json.NewEncoder(w).Encode(mark)
	if err != nil {
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

	var mark bookmark.Mark
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

	err = app.markLib.Set(bookID, mark)
	if err != nil {
		http.Error(w, "failed to update marks", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
