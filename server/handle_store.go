package server

import (
	"net/http"

	"github.com/voilelab/plainshelf/server/store"
)

// GET /api/shelves/{shelf_id}/marks/{book_id}
func (app *App) HandleAPIGetMarks(w http.ResponseWriter, r *http.Request) {
	// Unlike the update route this one intentionally does not require the shelf
	// to exist: an unknown shelf reads back as an empty bookmark.
	shelfID, err := readShelfID(r)
	if err != nil {
		http.Error(w, "invalid shelf_id", http.StatusBadRequest)
		return
	}

	bookID, ok := resolveBookID(w, r)
	if !ok {
		return
	}

	mark, err := app.storeDB.GetBookmark(shelfID, bookID)
	if err != nil {
		app.Error("failed to get marks", "error", err)
		http.Error(w, "failed to get marks", http.StatusInternalServerError)
		return
	}

	app.writeJSON(w, http.StatusOK, mark)
}

// POST /api/shelves/{shelf_id}/marks/{book_id}
func (app *App) HandleAPIUpdateMarks(w http.ResponseWriter, r *http.Request) {
	bookID, ok := resolveBookID(w, r)
	if !ok {
		return
	}

	var mark store.Bookmark
	if !decodeStrictJSON(w, r, &mark) {
		return
	}

	// The shelf is resolved after the body is decoded so a malformed body is
	// still reported as 400 for a shelf that does not exist.
	shelfData, ok := app.resolveShelf(w, r)
	if !ok {
		return
	}

	if err := app.storeDB.SetBookmark(shelfData.ID, bookID, mark); err != nil {
		app.Error("failed to update marks", "error", err)
		http.Error(w, "failed to update marks", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
