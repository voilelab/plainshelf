package server

import (
	"errors"
	"net/http"

	"github.com/voilelab/plainshelf/internal/taskutil"
	"github.com/voilelab/plainshelf/shelf"
)

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
	{shelf.ErrInvalidBookFormat, apiError{
		status:  http.StatusBadRequest,
		message: `format must be "txt" or "md"`,
	}},
	{shelf.ErrIgnoredLayerName, apiError{
		status:  http.StatusBadRequest,
		message: "invalid layer name: hidden and system directory names (a leading dot, @eaDir, #recycle, $RECYCLE.BIN, lost+found) are skipped by the shelf scanner, so a layer named this way would not stay visible",
	}},
	{shelf.ErrInvalidLayer, apiError{
		status:  http.StatusBadRequest,
		message: "invalid layer name",
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
	{shelf.ErrInvalidAssetName, apiError{
		status:  http.StatusBadRequest,
		message: "invalid asset name",
	}},
	{shelf.ErrAssetNotFound, apiError{
		status:  http.StatusNotFound,
		message: "asset not found",
	}},
	{shelf.ErrUnsupportedBookSchemaVersion, apiError{
		status:  http.StatusConflict,
		message: "book uses a newer on-disk format than this PlainShelf build supports; upgrade PlainShelf to modify it",
	}},
	{shelf.ErrUnsupportedSourceSchemaVersion, apiError{
		status:  http.StatusConflict,
		message: "source uses a newer on-disk format than this PlainShelf build supports; upgrade PlainShelf to modify it",
	}},
	{shelf.ErrUnsupportedTrashSchemaVersion, apiError{
		status:  http.StatusConflict,
		message: "trashed book uses a newer on-disk format than this PlainShelf build supports; upgrade PlainShelf to modify it",
	}},
	{taskutil.ErrWorkerBusy, apiError{
		status:     http.StatusServiceUnavailable,
		message:    "background worker is busy",
		retryAfter: "5",
	}},
}

// taskutil.ErrTaskChainRunning is deliberately absent from the table: it
// answers with a JSON body carrying the running chain's ID, so the task
// handlers match it themselves.
func apiErrorFor(err error) (apiError, bool) {
	for _, entry := range apiErrorTable {
		if errors.Is(err, entry.sentinel) {
			return entry.response, true
		}
	}

	return apiError{}, false
}

// writeErr answers a known error from the table, and anything else with 500
// and fallback, so an unexpected failure never leaks its detail to the client.
func (c *apiCore) writeErr(w http.ResponseWriter, err error, fallback string) {
	c.writeErrStatus(w, err, fallback, http.StatusInternalServerError)
}

// writeErrStatus is writeErr with a caller-chosen status for unknown errors.
// The layer routes answer a family of outcomes the table cannot yet name with
// a single status, and 500 would be wrong for those.
func (c *apiCore) writeErrStatus(w http.ResponseWriter, err error, fallback string, fallbackStatus int) {
	if resp, ok := apiErrorFor(err); ok {
		if resp.retryAfter != "" {
			w.Header().Set("Retry-After", resp.retryAfter)
		}
		http.Error(w, resp.message, resp.status)
		return
	}

	c.Error(fallback, "error", err)
	http.Error(w, fallback, fallbackStatus)
}
