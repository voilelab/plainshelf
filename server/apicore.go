package server

import (
	"encoding/json"
	"io"
	"io/fs"
	"net/http"

	"github.com/voilelab/plainshelf/internal/logutil"
	"github.com/voilelab/plainshelf/shelf"
)

// apiCore is what every handler group needs and nothing more: somewhere to log,
// the shelves to look things up in, and a single way to write a response.
//
// security is here only so cacheVisibility can derive a stored file's
// Cache-Control visibility from the token gate rather than from the config, so
// the two cannot drift apart. No handler consults it for anything else - the
// gate itself runs in Security.Middleware, before routing.
type apiCore struct {
	*logutil.Logger

	shelves  *shelf.ShelfManager
	security *Security
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

func (c *apiCore) getBook(w http.ResponseWriter, shelfData *shelf.ShelfData, bookID string) (*shelf.Book, bool) {
	book, err := shelfData.GetBook(bookID)
	if err != nil {
		c.writeErr(w, err, "failed to get book")
		return nil, false
	}

	return book, true
}

func (c *apiCore) getSource(w http.ResponseWriter, book *shelf.Book, sourceID string) (*shelf.Source, bool) {
	source, err := book.GetSource(sourceID)
	if err != nil {
		c.writeErr(w, err, "failed to get book source")
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

	book, ok := c.getBook(w, shelfData, bookID)
	if !ok {
		return nil, nil, false
	}

	return shelfData, book, true
}

func (c *apiCore) loadBookSource(w http.ResponseWriter, r *http.Request) (*shelf.Book, *shelf.Source, bool) {
	shelfData, ok := c.resolveShelf(w, r)
	if !ok {
		return nil, nil, false
	}

	bookID, ok := resolveBookID(w, r)
	if !ok {
		return nil, nil, false
	}

	sourceID, ok := resolveSourceID(w, r)
	if !ok {
		return nil, nil, false
	}

	book, ok := c.getBook(w, shelfData, bookID)
	if !ok {
		return nil, nil, false
	}

	source, ok := c.getSource(w, book, sourceID)
	if !ok {
		return nil, nil, false
	}

	return book, source, true
}

// streamTextFile writes an open file as the plain-text response body.
//
// A zero-length file needs the explicit 200: a copy that writes nothing never
// commits the response.
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

// writeJSON encodes v as the response body with the given status.
//
// The value is marshalled before any header is written so an encoding failure
// can still be reported as 500; once the body is on the wire a write failure
// can only be logged.
func (c *apiCore) writeJSON(w http.ResponseWriter, status int, v any) {
	bs, err := json.Marshal(v)
	if err != nil {
		c.Error("failed to encode response", "error", err)
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
		return
	}

	// json.Encoder.Encode terminates each value with a newline.
	bs = append(bs, '\n')

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if _, err := w.Write(bs); err != nil {
		c.Error("failed to write response", "error", err)
	}
}
