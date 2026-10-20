package server

import (
	"encoding/json/v2"
	"io"
	"io/fs"
	"net/http"
	"sync"

	"github.com/voilelab/plainshelf/internal/fsutil"
	"github.com/voilelab/plainshelf/internal/jsonopt"
	"github.com/voilelab/plainshelf/internal/logutil"
	"github.com/voilelab/plainshelf/shelf"
)

type apiCore struct {
	*logutil.Logger

	shelves  *shelf.ShelfManager
	security *Security
	settings *settings

	showNSFWByShelf sync.Map
}

func (c *apiCore) requestLogger(r *http.Request) *logutil.Logger {
	return c.With("request_id", logutil.RequestIDFrom(r.Context()))
}

func (c *apiCore) resolveShelf(w http.ResponseWriter, r *http.Request) (*shelf.ShelfData, bool) {
	shelfID, err := readShelfID(r)
	if err != nil {
		http.Error(w, "invalid shelf_id", http.StatusBadRequest)
		return nil, false
	}

	shelfData, ok := c.shelves.GetShelf(shelfID)
	if !ok {
		http.Error(w, "shelf not found", http.StatusNotFound)
		return nil, false
	}

	return shelfData, true
}

func (c *apiCore) rejectReadOnlyShelf(w http.ResponseWriter, r *http.Request, shelfData *shelf.ShelfData) bool {
	if !shelfData.ReadOnly() {
		return false
	}

	c.writeErr(w, r, fsutil.ErrReadOnly, "shelf is read-only")
	return true
}

func (c *apiCore) lookupBook(w http.ResponseWriter, r *http.Request, shelfData *shelf.ShelfData, bookID string) (*shelf.Book, bool) {
	listing, ok := c.lookupBookListing(w, r, shelfData, bookID)
	if !ok {
		return nil, false
	}

	return listing.Book, true
}

func (c *apiCore) lookupBookListing(w http.ResponseWriter, r *http.Request, shelfData *shelf.ShelfData, bookID string) (shelf.BookListing, bool) {
	listing, err := shelfData.GetBookListing(bookID)
	if err != nil {
		c.writeErr(w, r, err, "failed to get book")
		return shelf.BookListing{}, false
	}

	if !c.visibility(shelfData).allowsListing(listing) {
		c.writeErr(w, r, shelf.ErrBookNotFound, "failed to get book")
		return shelf.BookListing{}, false
	}

	return listing, true
}

func (c *apiCore) lookupSource(w http.ResponseWriter, r *http.Request, book *shelf.Book, sourceID string) (*shelf.Source, bool) {
	source, err := book.GetSource(sourceID)
	if err != nil {
		c.writeErr(w, r, err, "failed to get book source")
		return nil, false
	}

	return source, true
}

func (c *apiCore) loadBook(w http.ResponseWriter, r *http.Request) (*shelf.ShelfData, *shelf.Book, bool) {
	shelfData, ok := c.resolveShelf(w, r)
	if !ok {
		return nil, nil, false
	}

	bookID, ok := resolveBookID(w, r)
	if !ok {
		return nil, nil, false
	}

	book, ok := c.lookupBook(w, r, shelfData, bookID)
	if !ok {
		return nil, nil, false
	}

	return shelfData, book, true
}

func (c *apiCore) loadBookListing(w http.ResponseWriter, r *http.Request) (*shelf.ShelfData, shelf.BookListing, bool) {
	shelfData, ok := c.resolveShelf(w, r)
	if !ok {
		return nil, shelf.BookListing{}, false
	}

	bookID, ok := resolveBookID(w, r)
	if !ok {
		return nil, shelf.BookListing{}, false
	}

	listing, ok := c.lookupBookListing(w, r, shelfData, bookID)
	if !ok {
		return nil, shelf.BookListing{}, false
	}

	return shelfData, listing, true
}

func (c *apiCore) loadBookSource(w http.ResponseWriter, r *http.Request) (*shelf.ShelfData, *shelf.Book, *shelf.Source, bool) {
	shelfData, ok := c.resolveShelf(w, r)
	if !ok {
		return nil, nil, nil, false
	}

	bookID, ok := resolveBookID(w, r)
	if !ok {
		return nil, nil, nil, false
	}

	sourceID, ok := resolveSourceID(w, r)
	if !ok {
		return nil, nil, nil, false
	}

	book, ok := c.lookupBook(w, r, shelfData, bookID)
	if !ok {
		return nil, nil, nil, false
	}

	source, ok := c.lookupSource(w, r, book, sourceID)
	if !ok {
		return nil, nil, nil, false
	}

	return shelfData, book, source, true
}

func (c *apiCore) streamTextFile(w http.ResponseWriter, file fs.File, failureMsg string, logArgs ...any) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")

	if fi, statErr := file.Stat(); statErr == nil && fi.Size() == 0 {
		w.WriteHeader(http.StatusOK)
		return
	}

	if _, err := io.Copy(w, file); err != nil {
		c.Error(failureMsg, append([]any{"error", err}, logArgs...)...)
		http.Error(w, failureMsg, http.StatusInternalServerError)
	}
}

func (c *apiCore) writeJSON(w http.ResponseWriter, status int, v any) {
	bs, err := json.Marshal(v, jsonopt.API())
	if err != nil {
		c.Error("failed to encode response", "error", err)
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
		return
	}

	bs = append(bs, '\n')

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if _, err := w.Write(bs); err != nil {
		c.Error("failed to write response", "error", err)
	}
}
