package server

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/voilelab/plainshelf/internal/util"
	"github.com/voilelab/plainshelf/shelf"
)

// GET /api/shelves/{shelf_id}/books/{book_id}/sources
func (app *App) HandleAPIGetBookSources(w http.ResponseWriter, r *http.Request) {
	shelfID, err := readShelfID(r)
	if err != nil {
		http.Error(w, "invalid shelf_id", http.StatusBadRequest)
		return
	}

	shelfData, ok := app.shelfManager.GetShelf(shelfID)
	if !ok {
		http.Error(w, "shelf not found", http.StatusNotFound)
		return
	}

	bookID, err := readBookID(r)
	if err != nil {
		http.Error(w, "invalid book_id", http.StatusBadRequest)
		return
	}

	book, err := shelfData.GetBook(bookID)
	if err != nil {
		if handleShelfErr(w, err) {
			return
		}
		app.Error("failed to get book", "error", err)
		http.Error(w, "failed to get book", http.StatusInternalServerError)
		return
	}

	sources, err := book.ListSource()
	if err != nil {
		app.Error("failed to list book sources", "error", err)
		http.Error(w, "failed to list book sources", http.StatusInternalServerError)
		return
	}

	sourceMetas := make([]*shelf.SourceMeta, len(sources))
	for i, s := range sources {
		sourceMetas[i] = s.GetMeta()
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	err = json.NewEncoder(w).Encode(sourceMetas)
	if err != nil {
		app.Error("failed to encode response", "error", err)
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
		return
	}
}

// GET /api/shelves/{shelf_id}/books/{book_id}/sources/{source_id}
func (app *App) HandleAPIGetBookSource(w http.ResponseWriter, r *http.Request) {
	shelfID, err := readShelfID(r)
	if err != nil {
		http.Error(w, "invalid shelf_id", http.StatusBadRequest)
		return
	}

	shelfData, ok := app.shelfManager.GetShelf(shelfID)
	if !ok {
		http.Error(w, "shelf not found", http.StatusNotFound)
		return
	}

	bookID, err := readBookID(r)
	if err != nil {
		http.Error(w, "invalid book_id", http.StatusBadRequest)
		return
	}

	sourceID, err := readSourceID(r)
	if err != nil {
		http.Error(w, "invalid source_id", http.StatusBadRequest)
		return
	}

	book, err := shelfData.GetBook(bookID)
	if err != nil {
		if handleShelfErr(w, err) {
			return
		}
		app.Error("failed to get book", "error", err)
		http.Error(w, "failed to get book", http.StatusInternalServerError)
		return
	}

	source, err := book.GetSource(sourceID)
	if err != nil {
		if errors.Is(err, shelf.ErrSourceNotFound) {
			http.Error(w, "source not found", http.StatusNotFound)
			return
		}
		app.Error("failed to get book source", "error", err)
		http.Error(w, "failed to get book source", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	err = json.NewEncoder(w).Encode(source.GetMeta())
	if err != nil {
		app.Error("failed to encode response", "error", err)
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
		return
	}
}

// POST /api/shelves/{shelf_id}/books/{book_id}/sources
func (app *App) HandleAPICreateBookSource(w http.ResponseWriter, r *http.Request) {
	shelfID, err := readShelfID(r)
	if err != nil {
		http.Error(w, "invalid shelf_id", http.StatusBadRequest)
		return
	}

	shelfData, ok := app.shelfManager.GetShelf(shelfID)
	if !ok {
		http.Error(w, "shelf not found", http.StatusNotFound)
		return
	}

	bookID, err := readBookID(r)
	if err != nil {
		http.Error(w, "invalid book_id", http.StatusBadRequest)
		return
	}

	book, err := shelfData.GetBook(bookID)
	if err != nil {
		if handleShelfErr(w, err) {
			return
		}
		app.Error("failed to get book", "error", err)
		http.Error(w, "failed to get book", http.StatusInternalServerError)
		return
	}

	sourceMeta, err := book.NewSource(nil)
	if err != nil {
		app.Error("failed to create book source", "error", err)
		http.Error(w, "failed to create book source", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	err = json.NewEncoder(w).Encode(sourceMeta.GetMeta())
	if err != nil {
		app.Error("failed to encode response", "error", err)
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
		return
	}
}

// DELETE /api/shelves/{shelf_id}/books/{book_id}/sources/{source_id}
func (app *App) HandleAPIDeleteBookSource(w http.ResponseWriter, r *http.Request) {
	shelfID, err := readShelfID(r)
	if err != nil {
		http.Error(w, "invalid shelf_id", http.StatusBadRequest)
		return
	}

	shelfData, ok := app.shelfManager.GetShelf(shelfID)
	if !ok {
		http.Error(w, "shelf not found", http.StatusNotFound)
		return
	}

	bookID, err := readBookID(r)
	if err != nil {
		http.Error(w, "invalid book_id", http.StatusBadRequest)
		return
	}

	sourceID, err := readSourceID(r)
	if err != nil {
		http.Error(w, "invalid source_id", http.StatusBadRequest)
		return
	}

	book, err := shelfData.GetBook(bookID)
	if err != nil {
		if handleShelfErr(w, err) {
			return
		}
		app.Error("failed to get book", "error", err)
		http.Error(w, "failed to get book", http.StatusInternalServerError)
		return
	}

	err = book.DeleteSource(sourceID)
	if err != nil {
		if errors.Is(err, shelf.ErrSourceNotFound) {
			http.Error(w, "source not found", http.StatusNotFound)
			return
		}
		app.Error("failed to delete book source", "error", err)
		http.Error(w, "failed to delete book source", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// PUT /api/shelves/{shelf_id}/books/{book_id}/sources/{source_id}/current
func (app *App) HandleAPISetCurrentBookSource(w http.ResponseWriter, r *http.Request) {
	shelfID, err := readShelfID(r)
	if err != nil {
		http.Error(w, "invalid shelf_id", http.StatusBadRequest)
		return
	}

	shelfData, ok := app.shelfManager.GetShelf(shelfID)
	if !ok {
		http.Error(w, "shelf not found", http.StatusNotFound)
		return
	}

	bookID, err := readBookID(r)
	if err != nil {
		http.Error(w, "invalid book_id", http.StatusBadRequest)
		return
	}

	sourceID, err := readSourceID(r)
	if err != nil {
		http.Error(w, "invalid source_id", http.StatusBadRequest)
		return
	}

	book, err := shelfData.GetBook(bookID)
	if err != nil {
		if handleShelfErr(w, err) {
			return
		}
		app.Error("failed to get book", "error", err)
		http.Error(w, "failed to get book", http.StatusInternalServerError)
		return
	}

	_, err = book.GetSource(sourceID)
	if err != nil {
		if errors.Is(err, shelf.ErrSourceNotFound) {
			http.Error(w, "source not found", http.StatusNotFound)
			return
		}
		app.Error("failed to get book source", "error", err)
		http.Error(w, "failed to get book source", http.StatusInternalServerError)
		return
	}

	err = book.SetCurrentSource(sourceID)
	if err != nil {
		app.Error("failed to set current book source", "error", err)
		http.Error(w, "failed to set current book source", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// GET /api/shelves/{shelf_id}/books/{book_id}/sources/{source_id}/content
func (app *App) HandleAPIGetBookSourceContent(w http.ResponseWriter, r *http.Request) {
	shelfID, err := readShelfID(r)
	if err != nil {
		http.Error(w, "invalid shelf_id", http.StatusBadRequest)
		return
	}

	shelfData, ok := app.shelfManager.GetShelf(shelfID)
	if !ok {
		http.Error(w, "shelf not found", http.StatusNotFound)
		return
	}

	bookID, err := readBookID(r)
	if err != nil {
		http.Error(w, "invalid book_id", http.StatusBadRequest)
		return
	}

	sourceID, err := readSourceID(r)
	if err != nil {
		http.Error(w, "invalid source_id", http.StatusBadRequest)
		return
	}

	book, err := shelfData.GetBook(bookID)
	if err != nil {
		if handleShelfErr(w, err) {
			return
		}
		app.Error("failed to get book", "error", err)
		http.Error(w, "failed to get book", http.StatusInternalServerError)
		return
	}

	source, err := book.GetSource(sourceID)
	if err != nil {
		if errors.Is(err, shelf.ErrSourceNotFound) {
			http.Error(w, "source not found", http.StatusNotFound)
			return
		}
		app.Error("failed to get book source", "error", err)
		http.Error(w, "failed to get book source", http.StatusInternalServerError)
		return
	}

	src, err := source.Open()
	if err != nil {
		app.Error("failed to open book source", "error", err)
		http.Error(w, "failed to open book source", http.StatusInternalServerError)
		return
	}
	defer src.Close()

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	if fi, statErr := src.Stat(); statErr == nil && fi.Size() == 0 {
		w.WriteHeader(http.StatusOK)
		return
	}
	_, err = io.Copy(w, src)
	if err != nil {
		app.Error("failed to write book source content", "error", err)
		http.Error(w, "failed to write book source content", http.StatusInternalServerError)
		return
	}
}

// POST /api/shelves/{shelf_id}/books/{book_id}/sources/{source_id}/refresh
func (app *App) HandleAPIRefreshBookSourceMeta(w http.ResponseWriter, r *http.Request) {
	shelfID, err := readShelfID(r)
	if err != nil {
		http.Error(w, "invalid shelf_id", http.StatusBadRequest)
		return
	}

	shelfData, ok := app.shelfManager.GetShelf(shelfID)
	if !ok {
		http.Error(w, "shelf not found", http.StatusNotFound)
		return
	}

	bookID, err := readBookID(r)
	if err != nil {
		http.Error(w, "invalid book_id", http.StatusBadRequest)
		return
	}

	sourceID, err := readSourceID(r)
	if err != nil {
		http.Error(w, "invalid source_id", http.StatusBadRequest)
		return
	}

	book, err := shelfData.GetBook(bookID)
	if err != nil {
		if handleShelfErr(w, err) {
			return
		}
		app.Error("failed to get book", "error", err)
		http.Error(w, "failed to get book", http.StatusInternalServerError)
		return
	}

	source, err := book.GetSource(sourceID)
	if err != nil {
		if errors.Is(err, shelf.ErrSourceNotFound) {
			http.Error(w, "source not found", http.StatusNotFound)
			return
		}
		app.Error("failed to get book source", "error", err)
		http.Error(w, "failed to get book source", http.StatusInternalServerError)
		return
	}

	err = source.RefreshContentMetadata()
	if err != nil {
		app.Error("failed to refresh source metadata", "error", err)
		http.Error(w, "failed to refresh source metadata", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	err = json.NewEncoder(w).Encode(source.GetMeta())
	if err != nil {
		app.Error("failed to encode response", "error", err)
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
		return
	}
}

// PATCH /api/shelves/{shelf_id}/books/{book_id}/sources/{source_id}/content
func (app *App) HandleAPIUpdateBookSourceContent(w http.ResponseWriter, r *http.Request) {
	shelfID, err := readShelfID(r)
	if err != nil {
		http.Error(w, "invalid shelf_id", http.StatusBadRequest)
		return
	}

	shelfData, ok := app.shelfManager.GetShelf(shelfID)
	if !ok {
		http.Error(w, "shelf not found", http.StatusNotFound)
		return
	}

	bookID, err := readBookID(r)
	if err != nil {
		http.Error(w, "invalid book_id", http.StatusBadRequest)
		return
	}

	sourceID, err := readSourceID(r)
	if err != nil {
		http.Error(w, "invalid source_id", http.StatusBadRequest)
		return
	}

	book, err := shelfData.GetBook(bookID)
	if err != nil {
		if handleShelfErr(w, err) {
			return
		}
		app.Error("failed to get book", "error", err)
		http.Error(w, "failed to get book", http.StatusInternalServerError)
		return
	}

	source, err := book.GetSource(sourceID)
	if err != nil {
		if errors.Is(err, shelf.ErrSourceNotFound) {
			http.Error(w, "source not found", http.StatusNotFound)
			return
		}
		app.Error("failed to get book source", "error", err)
		http.Error(w, "failed to get book source", http.StatusInternalServerError)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxImportBodySize)
	utf8Reader, _, err := util.ReEncodeToUTF8(r.Body)
	if err != nil {
		if isRequestBodyTooLarge(err) {
			http.Error(w, "request body too large (max 100 MB)", http.StatusRequestEntityTooLarge)
			return
		}
		app.Error("failed to re-encode request body to UTF-8", "error", err)
		http.Error(w, "failed to re-encode request body to UTF-8", http.StatusInternalServerError)
		return
	}

	err = source.UpdateContent(utf8Reader)
	if err != nil {
		app.Error("failed to update book source content", "error", err)
		http.Error(w, "failed to update book source content", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
