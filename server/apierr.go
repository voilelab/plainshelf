package server

import (
	"errors"
	"net/http"

	"github.com/voilelab/plainshelf/internal/taskutil"
	"github.com/voilelab/plainshelf/shelf"
)

// This file holds the single mapping from a domain error to its HTTP response.
//
// It replaces the previous arrangement, where one helper knew four shelf
// sentinels and every other sentinel was matched by a hand-written errors.Is
// chain inside whichever handler happened to need it. Those chains disagreed:
// a shelf that was still initializing produced 503 on the book routes and 500
// on the source routes, purely because one chain had been copied without the
// shelf check. Keeping the mapping in one ordered table makes that class of
// drift impossible, and it removes the ordering hazard that came with checking
// a specific sentinel before a general one in each caller.

// apiError is the HTTP response a known domain error maps to.
type apiError struct {
	status  int
	message string

	// retryAfter, when set, is sent as the Retry-After header in seconds.
	retryAfter string
}

// apiErrorTable is consulted in order, so a more specific sentinel must come
// before any general one it could also match.
var apiErrorTable = []struct {
	sentinel error
	response apiError
}{
	{shelf.ErrInvalidIdentifierKey, apiError{
		status:  http.StatusBadRequest,
		message: "identifier key cannot be empty",
	}},
	{shelf.ErrInvalidStar, apiError{
		status:  http.StatusBadRequest,
		message: "star must be between 0 and 5",
	}},
	{shelf.ErrInvalidLanguageTag, apiError{
		status:  http.StatusBadRequest,
		message: "language must be a BCP 47 tag",
	}},
	{shelf.ErrShelfInitializing, apiError{
		status:     http.StatusServiceUnavailable,
		message:    "shelf is initializing, please retry shortly",
		retryAfter: "3",
	}},
	{shelf.ErrShelfLockTimeout, apiError{
		status:     http.StatusServiceUnavailable,
		message:    "shelf is busy, please retry shortly",
		retryAfter: "5",
	}},
	{shelf.ErrBookNotFound, apiError{
		status:  http.StatusNotFound,
		message: "book not found",
	}},
	{shelf.ErrTrashedBookNotFound, apiError{
		status:  http.StatusNotFound,
		message: "trashed book not found",
	}},
	{shelf.ErrSourceNotFound, apiError{
		status:  http.StatusNotFound,
		message: "source not found",
	}},
	{shelf.ErrUnsupportedBookSchemaVersion, apiError{
		status:  http.StatusConflict,
		message: "book uses a newer on-disk format than this PlainShelf build supports; upgrade PlainShelf to modify it",
	}},
	{taskutil.ErrWorkerBusy, apiError{
		status:     http.StatusServiceUnavailable,
		message:    "background worker is busy",
		retryAfter: "5",
	}},
}

// apiErrorFor reports the response a known domain error maps to.
//
// taskutil.ErrTaskChainRunning is deliberately absent: it answers with a JSON
// body carrying the running chain's ID, which this table cannot express, so
// the task handlers match it themselves before calling writeErr.
func apiErrorFor(err error) (apiError, bool) {
	for _, entry := range apiErrorTable {
		if errors.Is(err, entry.sentinel) {
			return entry.response, true
		}
	}

	return apiError{}, false
}

// writeErr writes the response a known domain error maps to. An error the table
// does not know is logged with fallback as the message and answered with 500,
// so an unexpected failure never leaks its detail to the client.
func (app *App) writeErr(w http.ResponseWriter, err error, fallback string) {
	if resp, ok := apiErrorFor(err); ok {
		if resp.retryAfter != "" {
			w.Header().Set("Retry-After", resp.retryAfter)
		}
		http.Error(w, resp.message, resp.status)
		return
	}

	app.Error(fallback, "error", err)
	http.Error(w, fallback, http.StatusInternalServerError)
}
