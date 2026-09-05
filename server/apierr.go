package server

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/voilelab/plainshelf/internal/fsutil"
	"github.com/voilelab/plainshelf/internal/logutil"
	"github.com/voilelab/plainshelf/internal/taskutil"
	"github.com/voilelab/plainshelf/shelf"
)

// RequestIDHeader carries each request's ID back to the client. It is the
// header form of the error envelope's incident field, so a user can quote a
// number for a request that did not fail at all.
const RequestIDHeader = "X-Request-Id"

type apiError struct {
	status  int
	message string

	// code is this error's stable public identifier, quoted verbatim in bug
	// reports. It is part of the API: renaming one is a breaking change, and
	// TestAPIErrorCodeListIsPinned holds the whole list so a rename cannot pass
	// unseen.
	code string

	// retryAfter, when set, is sent as the Retry-After header in seconds.
	retryAfter string
}

// codeIgnoredFolderName is spelled once because two paths answer with it: the
// table entry, and the errors.As branch in apiErrorFor that replaces only the
// message. They are the same refusal, so a client must not have to switch on
// two codes for it.
const codeIgnoredFolderName = "IGNORED_FOLDER_NAME"

// codeMalformedMetadata is spelled once for the same reason: the table entry
// and the errors.As branch below are one refusal, and only the message differs.
const codeMalformedMetadata = "MALFORMED_METADATA"

// apiErrorTable is consulted in order, so a more specific sentinel must come
// before any general one it could also match.
var apiErrorTable = []struct {
	sentinel error
	response apiError
}{
	{shelf.ErrInvalidIdentifierKey, apiError{
		code:    "INVALID_IDENTIFIER_KEY",
		status:  http.StatusBadRequest,
		message: "identifier key cannot be empty",
	}},
	{shelf.ErrInvalidStar, apiError{
		code:    "INVALID_STAR",
		status:  http.StatusBadRequest,
		message: "star must be between 0 and 5",
	}},
	{shelf.ErrInvalidLanguageTag, apiError{
		code:    "INVALID_LANGUAGE_TAG",
		status:  http.StatusBadRequest,
		message: "language must be a BCP 47 tag",
	}},
	{shelf.ErrInvalidBookFormat, apiError{
		code:    "INVALID_BOOK_FORMAT",
		status:  http.StatusBadRequest,
		message: `format must be "txt" or "md"`,
	}},
	{shelf.ErrIgnoredFolderName, apiError{
		code:    codeIgnoredFolderName,
		status:  http.StatusBadRequest,
		message: "invalid folder name: this name is skipped by the shelf scanner, so a folder named this way would not stay visible",
	}},
	{shelf.ErrInvalidFolder, apiError{
		code:    "INVALID_FOLDER",
		status:  http.StatusBadRequest,
		message: "invalid folder name",
	}},
	{shelf.ErrShelfInitializing, apiError{
		code:       "SHELF_INITIALIZING",
		status:     http.StatusServiceUnavailable,
		message:    "shelf is initializing, please retry shortly",
		retryAfter: "3",
	}},
	{shelf.ErrShelfLockTimeout, apiError{
		code:       "SHELF_LOCK_TIMEOUT",
		status:     http.StatusServiceUnavailable,
		message:    "shelf is busy, please retry shortly",
		retryAfter: "5",
	}},
	{shelf.ErrBookNotFound, apiError{
		code:    "BOOK_NOT_FOUND",
		status:  http.StatusNotFound,
		message: "book not found",
	}},
	{shelf.ErrBookIDConflict, apiError{
		code:    "BOOK_ID_CONFLICT",
		status:  http.StatusConflict,
		message: "the target shelf already holds a book with this ID; the move would overwrite it",
	}},
	{shelf.ErrTrashedBookNotFound, apiError{
		code:    "TRASHED_BOOK_NOT_FOUND",
		status:  http.StatusNotFound,
		message: "trashed book not found",
	}},
	{shelf.ErrSourceNotFound, apiError{
		code:    "SOURCE_NOT_FOUND",
		status:  http.StatusNotFound,
		message: "source not found",
	}},
	{shelf.ErrInvalidAssetName, apiError{
		code:    "INVALID_ASSET_NAME",
		status:  http.StatusBadRequest,
		message: "invalid asset name",
	}},
	{shelf.ErrAssetNotFound, apiError{
		code:    "ASSET_NOT_FOUND",
		status:  http.StatusNotFound,
		message: "asset not found",
	}},
	{fsutil.ErrReadOnly, apiError{
		code:    "SHELF_READ_ONLY",
		status:  http.StatusConflict,
		message: "shelf is opened read-only; this PlainShelf instance cannot modify it",
	}},
	{shelf.ErrUnsupportedBookSchemaVersion, apiError{
		code:    "UNSUPPORTED_BOOK_SCHEMA_VERSION",
		status:  http.StatusConflict,
		message: "book uses a newer on-disk format than this PlainShelf build supports; upgrade PlainShelf to modify it",
	}},
	{shelf.ErrUnsupportedSourceSchemaVersion, apiError{
		code:    "UNSUPPORTED_SOURCE_SCHEMA_VERSION",
		status:  http.StatusConflict,
		message: "source uses a newer on-disk format than this PlainShelf build supports; upgrade PlainShelf to modify it",
	}},
	{shelf.ErrUnsupportedTrashSchemaVersion, apiError{
		code:    "UNSUPPORTED_TRASH_SCHEMA_VERSION",
		status:  http.StatusConflict,
		message: "trashed book uses a newer on-disk format than this PlainShelf build supports; upgrade PlainShelf to modify it",
	}},
	{shelf.ErrMalformedMetadata, apiError{
		code:    codeMalformedMetadata,
		status:  http.StatusConflict,
		message: "a metadata file on the shelf is not valid JSON; repair the file and try again",
	}},
	{taskutil.ErrWorkerBusy, apiError{
		code:       "WORKER_BUSY",
		status:     http.StatusServiceUnavailable,
		message:    "background worker is busy",
		retryAfter: "5",
	}},
}

// The two codes no sentinel owns; both say only "the table does not name this
// failure".
//
//   - codeInternal accompanies a 5xx, whose cause was logged and withheld.
//   - codeUnknown accompanies writeErrStatus's caller-chosen non-5xx statuses,
//     where the request is at fault but the table cannot say how. Calling those
//     INTERNAL would send a reporter after a server bug that is not there.
const (
	codeInternal = "INTERNAL"
	codeUnknown  = "UNKNOWN"
)

// apiErrorBody is the JSON envelope writeErr answers with. It is nested under a
// single "error" key so a client can tell an error body from a successful one
// by shape alone, and so later fields land inside the envelope rather than
// colliding with a payload's own field names.
type apiErrorBody struct {
	Error apiErrorDetail `json:"error"`
}

type apiErrorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`

	// Incident is the request's ID, not a second number minted for the failure,
	// so a user reporting a refusal they understand has something to quote and
	// quotes the same string X-Request-Id carries.
	//
	// omitempty for the responses no middleware saw: the desktop client reaches
	// some handlers directly, and those have no request to take an ID from.
	Incident string `json:"incident,omitempty"`
}

// taskutil.ErrTaskChainRunning is deliberately absent from the table: it
// answers with a JSON body carrying the running chain's ID, so the task
// handlers match it themselves.
func apiErrorFor(err error) (apiError, bool) {
	// Which directories a shelf skips is that shelf's own setting, so naming the
	// built-in ones here would send a user whose shelf.json lists its own after
	// a rule that does not apply. The shelf carries the reason out instead.
	var ignored *shelf.IgnoredFolderNameError
	if errors.As(err, &ignored) {
		return apiError{
			code:    codeIgnoredFolderName,
			status:  http.StatusBadRequest,
			message: ignoredFolderMessage(ignored),
		}, true
	}

	// Which file is broken, and where, is the whole content of this refusal: a
	// user who hand-edited a book.json cannot act on "some file is invalid", so
	// the path and the decoder's own message travel out with the error.
	var malformed *shelf.MalformedMetadataError
	if errors.As(err, &malformed) {
		return apiError{
			code:    codeMalformedMetadata,
			status:  http.StatusConflict,
			message: malformedMetadataMessage(malformed),
		}, true
	}

	for _, entry := range apiErrorTable {
		if errors.Is(err, entry.sentinel) {
			return entry.response, true
		}
	}

	return apiError{}, false
}

// ignoredFolderMessage explains a refused folder name in the shelf's own terms.
func ignoredFolderMessage(err *shelf.IgnoredFolderNameError) string {
	const consequence = ", so a folder named this way would not stay visible"
	if err.Reason == "" {
		return fmt.Sprintf("invalid folder name: this shelf skips %q while scanning%s", err.Folder, consequence)
	}
	return fmt.Sprintf("invalid folder name: this shelf skips %q while scanning (%s)%s", err.Folder, err.Reason, consequence)
}

// malformedMetadataMessage names the file that could not be read and quotes the
// decoder on why, which is where the offending member name comes from.
func malformedMetadataMessage(err *shelf.MalformedMetadataError) string {
	return fmt.Sprintf("%s is not valid JSON: %v; repair the file and try again", err.File, err.Err)
}

// writeErr answers a known error from the table, and anything else with 500 and
// fallback. The body is the JSON envelope either way; err.Error() reaches the
// log, not the client.
func (c *apiCore) writeErr(w http.ResponseWriter, r *http.Request, err error, fallback string) {
	c.writeErrStatus(w, r, err, fallback, http.StatusInternalServerError)
}

// writeErrStatus is writeErr with a caller-chosen status for unknown errors.
// The folder routes answer a family of outcomes the table cannot yet name with
// a single status, and 500 would be wrong for those.
func (c *apiCore) writeErrStatus(w http.ResponseWriter, r *http.Request, err error, fallback string, fallbackStatus int) {
	incident := logutil.RequestIDFrom(r.Context())

	if resp, ok := apiErrorFor(err); ok {
		if resp.retryAfter != "" {
			w.Header().Set("Retry-After", resp.retryAfter)
		}
		c.writeErrBody(w, resp.status, resp.code, resp.message, incident)
		return
	}

	code := codeUnknown
	if fallbackStatus >= http.StatusInternalServerError {
		code = codeInternal
	}

	// The body deliberately withholds the cause, so this line has to answer
	// every question it cannot: which request, what it asked for, which shelf,
	// and the whole error chain.
	c.Error(fallback,
		"request_id", incident,
		"code", code,
		"method", r.Method,
		"path", r.URL.Path,
		"shelf_id", r.PathValue("shelf_id"),
		"error", err,
	)

	c.writeErrBody(w, fallbackStatus, code, fallback, incident)
}

// writeErrBody sends the error envelope. It shares writeJSON so error and
// success bodies cannot drift apart on content type or trailing newline.
func (c *apiCore) writeErrBody(w http.ResponseWriter, status int, code, message, incident string) {
	c.writeJSON(w, status, apiErrorBody{Error: apiErrorDetail{Code: code, Message: message, Incident: incident}})
}
