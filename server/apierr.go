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

const RequestIDHeader = "X-Request-Id"

type apiError struct {
	status  int
	message string

	code string

	retryAfter string
}

const codeIgnoredFolderName = "IGNORED_FOLDER_NAME"

const codeMalformedMetadata = "MALFORMED_METADATA"

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
	{shelf.ErrFolderNotFound, apiError{
		code:    "FOLDER_NOT_FOUND",
		status:  http.StatusNotFound,
		message: "folder not found",
	}},
	{shelf.ErrInvalidFolderIcon, apiError{
		code:    "INVALID_FOLDER_ICON",
		status:  http.StatusBadRequest,
		message: "folder icon is not one this server offers; see icons in GET .../folder-meta",
	}},
	{shelf.ErrFolderMetaConflict, apiError{
		code:    "FOLDER_META_CONFLICT",
		status:  http.StatusConflict,
		message: "folder.json changed since it was read; read it again and retry",
	}},
	{shelf.ErrFolderMetaNotWritable, apiError{
		code:    "FOLDER_META_NOT_WRITABLE",
		status:  http.StatusConflict,
		message: "folder.json cannot be read safely by this server, so it was left unchanged; repair or remove the file",
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

	{fsutil.ErrSourceUnauthorized, apiError{
		code:    "SOURCE_UNAUTHORIZED",
		status:  http.StatusServiceUnavailable,
		message: "the shelf source refused the credential",
	}},

	{shelf.ErrNotAShelf, apiError{
		code:    SourceCheckNotAShelf,
		status:  http.StatusServiceUnavailable,
		message: "the shelf source is not a shelf: it has no books/ directory",
	}},

	{shelf.ErrBookNotDownloaded, apiError{
		code:    "BOOK_NOT_DOWNLOADED",
		status:  http.StatusServiceUnavailable,
		message: "the shelf source is unavailable and this book has not been downloaded",
	}},
	{fsutil.ErrSourceUnavailable, apiError{
		code:    "SOURCE_UNAVAILABLE",
		status:  http.StatusServiceUnavailable,
		message: "the shelf source is unavailable",
	}},
	{shelf.ErrDownloadUnsupported, apiError{
		code:    "DOWNLOAD_UNSUPPORTED",
		status:  http.StatusConflict,
		message: "this shelf is already local; only a virtual shelf keeps offline copies",
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

const (
	codeInternal = "INTERNAL"
	codeUnknown  = "UNKNOWN"
)

type apiErrorBody struct {
	Error apiErrorDetail `json:"error"`
}

type apiErrorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`

	Incident string `json:"incident,omitempty"`
}

func apiErrorFor(err error) (apiError, bool) {

	if ignored, ok := errors.AsType[*shelf.IgnoredFolderNameError](err); ok {
		return apiError{
			code:    codeIgnoredFolderName,
			status:  http.StatusBadRequest,
			message: ignoredFolderMessage(ignored),
		}, true
	}

	if malformed, ok := errors.AsType[*shelf.MalformedMetadataError](err); ok {
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

func ignoredFolderMessage(err *shelf.IgnoredFolderNameError) string {
	const consequence = ", so a folder named this way would not stay visible"
	if err.Reason == "" {
		return fmt.Sprintf("invalid folder name: this shelf skips %q while scanning%s", err.Folder, consequence)
	}
	return fmt.Sprintf("invalid folder name: this shelf skips %q while scanning (%s)%s", err.Folder, err.Reason, consequence)
}

func malformedMetadataMessage(err *shelf.MalformedMetadataError) string {
	return fmt.Sprintf("%s is not valid JSON: %v; repair the file and try again", err.File, err.Err)
}

func (c *apiCore) writeErr(w http.ResponseWriter, r *http.Request, err error, fallback string) {
	c.writeErrStatus(w, r, err, fallback, http.StatusInternalServerError)
}

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

func (c *apiCore) writeErrBody(w http.ResponseWriter, status int, code, message, incident string) {
	c.writeJSON(w, status, apiErrorBody{Error: apiErrorDetail{Code: code, Message: message, Incident: incident}})
}
