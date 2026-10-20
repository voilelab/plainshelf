package pcloud

import (
	"context"
	"errors"
	"io/fs"
	"net/url"
	"strconv"

	"github.com/voilelab/plainshelf/internal/fsutil"
)

// Result codes this package acts on.
const (
	// resultRecursiveListingUnsupported is returned by API variants that refuse
	// a recursive listing; it is permanent and is never retried.
	resultRecursiveListingUnsupported = 1101
	resultParentNotFound              = 2002
	resultDirectoryNotFound           = 2005
	resultFileNotFound                = 2009
)

var retryableStatus = map[int]bool{429: true, 500: true, 502: true, 503: true, 504: true, 509: true}

// Error is a failed pCloud call: an HTTP status, a non-zero result code, or a
// transport failure (neither set).
//
// It matches fs.ErrNotExist for "no such file or folder" and
// fsutil.ErrSourceUnavailable for everything else, which is how a shelf tells
// a missing file from being offline or refused.
type Error struct {
	Method  string
	Result  int
	Status  int
	Message string

	transport bool
	cause     error
}

func newTransportError(method string, err error) *Error {
	// A url.Error quotes the URL, and a download link is a credential.
	if uerr, ok := errors.AsType[*url.Error](err); ok {
		err = uerr.Err
	}
	return &Error{Method: method, Message: err.Error(), transport: true, cause: err}
}

func (e *Error) Error() string {
	msg := "pcloud: " + e.Method + " failed: " + e.Message
	if e.Result != 0 {
		msg += " (" + strconv.Itoa(e.Result) + ")"
	}
	return msg
}

func (e *Error) Unwrap() []error {
	errs := []error{fsutil.ErrSourceUnavailable}
	if e.notFound() {
		errs = []error{fs.ErrNotExist}
	}
	if e.cause != nil {
		errs = append(errs, e.cause)
	}
	return errs
}

func (e *Error) notFound() bool {
	switch e.Result {
	case resultParentNotFound, resultDirectoryNotFound, resultFileNotFound:
		return true
	}
	return false
}

// isRetryable follows rclone's pCloud backend: 4xxx is rate limiting and 5xxx
// an internal error; 1101 is permanent although it is 1xxx.
func isRetryable(err error) bool {
	var e *Error
	if !errors.As(err, &e) {
		return false
	}
	if e.transport {
		// A cancelled caller is not a transient failure.
		return !errors.Is(e.cause, context.Canceled)
	}
	if e.Result == resultRecursiveListingUnsupported {
		return false
	}
	if group := e.Result / 1000; group == 4 || group == 5 {
		return true
	}
	return retryableStatus[e.Status]
}
