package server

import (
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"net/http"

	"github.com/voilelab/plainshelf/shelf"
)

// This file holds the small HTTP plumbing helpers shared by every handler.
// They exist so the shelf/book lookup preamble and the JSON response tail are
// written once instead of being copied into each route, which is how the
// error mapping drifted apart between routes in the first place.
//
// The lookup helpers write the error response themselves and report whether
// the caller may continue, so a handler reads as a sequence of guard clauses:
//
//	shelfData, ok := app.resolveShelf(w, r)
//	if !ok {
//		return
//	}

// resolveShelf reads shelf_id from the request and looks the shelf up.
// It writes 400 for an unusable shelf_id and 404 for an unknown shelf.
func (app *App) resolveShelf(w http.ResponseWriter, r *http.Request) (*shelf.ShelfData, bool) {
	shelfID, err := readShelfID(r)
	if err != nil {
		http.Error(w, "invalid shelf_id", http.StatusBadRequest)
		return nil, false
	}

	shelfData, ok := app.shelfManager.GetShelf(shelfID)
	if !ok {
		http.Error(w, "shelf not found", http.StatusNotFound)
		return nil, false
	}

	return shelfData, true
}

// resolveBookID reads book_id from the request, writing 400 when it is missing
// or malformed. It is separate from getBook because several handlers must
// decode the request body between reading the ID and touching the shelf.
func resolveBookID(w http.ResponseWriter, r *http.Request) (string, bool) {
	bookID, err := readBookID(r)
	if err != nil {
		http.Error(w, "invalid book_id", http.StatusBadRequest)
		return "", false
	}

	return bookID, true
}

// resolveSourceID reads source_id from the request, writing 400 when it is
// missing or malformed.
func resolveSourceID(w http.ResponseWriter, r *http.Request) (string, bool) {
	sourceID, err := readSourceID(r)
	if err != nil {
		http.Error(w, "invalid source_id", http.StatusBadRequest)
		return "", false
	}

	return sourceID, true
}

// getBook loads a book, mapping the domain errors through writeErr.
func (app *App) getBook(w http.ResponseWriter, shelfData *shelf.ShelfData, bookID string) (*shelf.Book, bool) {
	book, err := shelfData.GetBook(bookID)
	if err != nil {
		app.writeErr(w, err, "failed to get book")
		return nil, false
	}

	return book, true
}

// getSource loads a source of a book, mapping the domain errors through
// writeErr.
func (app *App) getSource(w http.ResponseWriter, book *shelf.Book, sourceID string) (*shelf.Source, bool) {
	source, err := book.GetSource(sourceID)
	if err != nil {
		app.writeErr(w, err, "failed to get book source")
		return nil, false
	}

	return source, true
}

// loadBookSource resolves the shelf, book, and source for the source routes.
// source_id is validated before the book is loaded, matching the order the
// hand-written handlers used.
func (app *App) loadBookSource(w http.ResponseWriter, r *http.Request) (*shelf.Book, *shelf.Source, bool) {
	shelfData, ok := app.resolveShelf(w, r)
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

	book, ok := app.getBook(w, shelfData, bookID)
	if !ok {
		return nil, nil, false
	}

	source, ok := app.getSource(w, book, sourceID)
	if !ok {
		return nil, nil, false
	}

	return book, source, true
}

// loadBook resolves the shelf, reads book_id, and loads the book. Handlers that
// need to do work between those steps should call the three helpers directly.
func (app *App) loadBook(w http.ResponseWriter, r *http.Request) (*shelf.ShelfData, *shelf.Book, bool) {
	shelfData, ok := app.resolveShelf(w, r)
	if !ok {
		return nil, nil, false
	}

	bookID, ok := resolveBookID(w, r)
	if !ok {
		return nil, nil, false
	}

	book, ok := app.getBook(w, shelfData, bookID)
	if !ok {
		return nil, nil, false
	}

	return shelfData, book, true
}

// decodeStrictJSON decodes exactly one JSON value from the request body into v,
// rejecting unknown fields and any trailing content. It writes 400 and reports
// false when the body is not acceptable.
func decodeStrictJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(v); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return false
	}

	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return false
	}

	return true
}

// streamTextFile writes an open file as the plain-text response body.
//
// A zero-length file gets an explicit 200 rather than being handed to io.Copy:
// a copy that writes nothing never commits the response, so the status has to
// be written here instead. logArgs are appended to the failure log entry for
// callers that can name the file they were serving.
func (app *App) streamTextFile(w http.ResponseWriter, file fs.File, failureMsg string, logArgs ...any) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")

	if fi, statErr := file.Stat(); statErr == nil && fi.Size() == 0 {
		w.WriteHeader(http.StatusOK)
		return
	}

	if _, err := io.Copy(w, file); err != nil {
		app.Error(failureMsg, append([]any{"error", err}, logArgs...)...)
		http.Error(w, failureMsg, http.StatusInternalServerError)
	}
}

// writeJSON encodes v as the response body with the given status.
//
// The value is marshalled before any header is written so an encoding failure
// can still be reported as 500. Once the body is on the wire a write failure
// can only be logged: the status line is already gone, so the http.Error call
// that the hand-written copies of this code performed never took effect.
func (app *App) writeJSON(w http.ResponseWriter, status int, v any) {
	bs, err := json.Marshal(v)
	if err != nil {
		app.Error("failed to encode response", "error", err)
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
		return
	}

	// json.Encoder.Encode terminates each value with a newline; keep the bytes
	// on the wire identical to what the previous per-handler encoders produced.
	bs = append(bs, '\n')

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if _, err := w.Write(bs); err != nil {
		app.Error("failed to write response", "error", err)
	}
}
