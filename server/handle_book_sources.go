package server

import (
	"errors"
	"io"
	"net/http"

	"github.com/voilelab/plainshelf/internal/util"
	"github.com/voilelab/plainshelf/shelf"
)

// GET /api/shelves/{shelf_id}/books/{book_id}/sources
func (app *App) HandleAPIGetBookSources(w http.ResponseWriter, r *http.Request) {
	_, book, ok := app.loadBook(w, r)
	if !ok {
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

	app.writeJSON(w, http.StatusOK, sourceMetas)
}

// GET /api/shelves/{shelf_id}/books/{book_id}/sources/{source_id}
func (app *App) HandleAPIGetBookSource(w http.ResponseWriter, r *http.Request) {
	_, source, ok := app.loadBookSource(w, r)
	if !ok {
		return
	}

	app.writeJSON(w, http.StatusOK, source.GetMeta())
}

// POST /api/shelves/{shelf_id}/books/{book_id}/sources
func (app *App) HandleAPICreateBookSource(w http.ResponseWriter, r *http.Request) {
	_, book, ok := app.loadBook(w, r)
	if !ok {
		return
	}

	sourceMeta, err := book.NewSource(nil)
	if err != nil {
		app.Error("failed to create book source", "error", err)
		http.Error(w, "failed to create book source", http.StatusInternalServerError)
		return
	}

	app.writeJSON(w, http.StatusOK, sourceMeta.GetMeta())
}

// DELETE /api/shelves/{shelf_id}/books/{book_id}/sources/{source_id}
func (app *App) HandleAPIDeleteBookSource(w http.ResponseWriter, r *http.Request) {
	shelfData, ok := app.resolveShelf(w, r)
	if !ok {
		return
	}

	bookID, ok := resolveBookID(w, r)
	if !ok {
		return
	}

	sourceID, ok := resolveSourceID(w, r)
	if !ok {
		return
	}

	book, ok := app.getBook(w, shelfData, bookID)
	if !ok {
		return
	}

	// DeleteSource reports a missing source itself, so the source is not
	// loaded up front here.
	if err := book.DeleteSource(sourceID); err != nil {
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
	book, _, ok := app.loadBookSource(w, r)
	if !ok {
		return
	}

	// loadBookSource has already rejected an unknown source_id.
	sourceID, ok := resolveSourceID(w, r)
	if !ok {
		return
	}

	if err := book.SetCurrentSource(sourceID); err != nil {
		if handleShelfErr(w, err) {
			return
		}
		app.Error("failed to set current book source", "error", err)
		http.Error(w, "failed to set current book source", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// GET /api/shelves/{shelf_id}/books/{book_id}/sources/{source_id}/content
func (app *App) HandleAPIGetBookSourceContent(w http.ResponseWriter, r *http.Request) {
	_, source, ok := app.loadBookSource(w, r)
	if !ok {
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
	if _, err := io.Copy(w, src); err != nil {
		app.Error("failed to write book source content", "error", err)
		http.Error(w, "failed to write book source content", http.StatusInternalServerError)
		return
	}
}

// POST /api/shelves/{shelf_id}/books/{book_id}/sources/{source_id}/refresh
func (app *App) HandleAPIRefreshBookSourceMeta(w http.ResponseWriter, r *http.Request) {
	_, source, ok := app.loadBookSource(w, r)
	if !ok {
		return
	}

	if err := source.RefreshContentMetadata(); err != nil {
		app.Error("failed to refresh source metadata", "error", err)
		http.Error(w, "failed to refresh source metadata", http.StatusInternalServerError)
		return
	}

	app.writeJSON(w, http.StatusOK, source.GetMeta())
}

// PATCH /api/shelves/{shelf_id}/books/{book_id}/sources/{source_id}/content
func (app *App) HandleAPIUpdateBookSourceContent(w http.ResponseWriter, r *http.Request) {
	_, source, ok := app.loadBookSource(w, r)
	if !ok {
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

	if err := source.UpdateContent(utf8Reader); err != nil {
		app.Error("failed to update book source content", "error", err)
		http.Error(w, "failed to update book source content", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
