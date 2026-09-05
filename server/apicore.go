package server

import (
	"encoding/json/v2"
	"io"
	"io/fs"
	"net/http"

	"github.com/voilelab/plainshelf/internal/fsutil"
	"github.com/voilelab/plainshelf/internal/jsonopt"
	"github.com/voilelab/plainshelf/internal/logutil"
	"github.com/voilelab/plainshelf/shelf"
)

// apiCore is what every handler group needs and nothing more: somewhere to log,
// the shelves to look things up in, and a single way to write a response.
//
// security is here only so cacheVisibility can derive Cache-Control visibility
// from the token gate rather than from the config; the gate itself runs in
// Security.Middleware, before routing.
//
// settings is here for the one setting that decides what a request may see at
// all rather than how a handler behaves: show_nsfw. It sits on the shared core
// because the book lookups below apply it - see bookVisibility.
type apiCore struct {
	*logutil.Logger

	shelves  *shelf.ShelfManager
	security *Security
	settings *settings
}

// requestLogger stamps the request's ID on every line, for work that outlives
// the response: a background chain keeps logging long after the 202 that gave
// the user their number.
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

// rejectReadOnlyShelf answers the refusal a read-only shelf owes, ahead of the
// work. Two kinds of endpoint need it, for the same reason: the answer has to
// come before something the refusal would invalidate.
//
//   - The endpoints that queue a background chain would otherwise answer 202 and
//     let the caller discover the refusal in a task report, or not at all.
//   - The folder move and rename, which are synchronous, but ask the user
//     whether to unhide a marked subtree first (see refuseUnconfirmedReveal). A
//     question put to the user must not run ahead of a refusal, or they approve
//     a disclosure for a change that was never going to happen.
//
// Every other synchronous handler needs none of this: the shelf refuses it with
// fsutil.ErrReadOnly and writeErr turns that into the same 409 this writes.
func (c *apiCore) rejectReadOnlyShelf(w http.ResponseWriter, r *http.Request, shelfData *shelf.ShelfData) bool {
	if !shelfData.ReadOnly() {
		return false
	}

	c.writeErr(w, r, fsutil.ErrReadOnly, "shelf is read-only")
	return true
}

// lookupBook goes through the listing because half the visibility answer is the
// book's folder, which the book does not carry. The shelf does the same work
// either way, so this costs nothing beyond the folder it discards.
func (c *apiCore) lookupBook(w http.ResponseWriter, r *http.Request, shelfData *shelf.ShelfData, bookID string) (*shelf.Book, bool) {
	listing, ok := c.lookupBookListing(w, r, shelfData, bookID)
	if !ok {
		return nil, false
	}

	return listing.Book, true
}

// lookupBookListing is the single gate every route naming a book passes
// through, via loadBook, loadBookListing and loadBookSource.
//
// A book the request may not see is answered as one that is not there: 403 would
// confirm it exists, which is the fact being withheld, so this writes the
// response an unknown ID gets, byte for byte apart from the incident ID. The
// lookup still happens first, so a caller timing the two could in principle tell
// them apart; closing that would mean not consulting the shelf at all.
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

// loadBookListing is loadBook for a handler that also needs the book's folder.
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

// writeJSON marshals before writing any header so an encoding failure can still
// be reported as 500 - which is also why it is json.Marshal rather than
// json.MarshalWrite, since writing straight to w would commit a 200 before the
// encoder had a chance to fail.
//
// Every array-valued field reaches the client as [] rather than null, because
// jsonopt.API() takes json/v2's default for a nil slice or map. That is a
// contract - see the never-null assertions in server/contract.
func (c *apiCore) writeJSON(w http.ResponseWriter, status int, v any) {
	bs, err := json.Marshal(v, jsonopt.API())
	if err != nil {
		c.Error("failed to encode response", "error", err)
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
		return
	}

	// Kept from the json.Encoder.Encode this replaced, which terminated each
	// value with a newline. json.MarshalWrite does not, and changing the bytes
	// of every response body is not what this conversion is for.
	bs = append(bs, '\n')

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if _, err := w.Write(bs); err != nil {
		c.Error("failed to write response", "error", err)
	}
}
