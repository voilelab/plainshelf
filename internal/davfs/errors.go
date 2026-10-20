package davfs

import (
	"context"
	"errors"
	"io/fs"
	"net/http"
	"net/url"
	"strconv"

	"github.com/voilelab/plainshelf/internal/fsutil"
)

var retryableStatus = map[int]bool{
	http.StatusTooManyRequests:     true,
	http.StatusInternalServerError: true,
	http.StatusBadGateway:          true,
	http.StatusServiceUnavailable:  true,
	http.StatusGatewayTimeout:      true,
}

type Error struct {
	Op      string
	Path    string
	Status  int
	Message string

	transport bool
	cause     error
}

func newTransportError(op, name string, err error) *Error {
	if uerr, ok := errors.AsType[*url.Error](err); ok {
		err = uerr.Err
	}
	return &Error{Op: op, Path: name, Message: err.Error(), transport: true, cause: err}
}

func (e *Error) Error() string {
	msg := "webdav: " + e.Op + " " + strconv.Quote(e.Path) + ": " + e.Message
	if e.Status == http.StatusUnauthorized {
		msg += " (credential refused)"
	}
	return msg
}

func (e *Error) Unwrap() []error {
	var errs []error
	switch e.Status {
	case http.StatusNotFound, http.StatusGone:
		errs = []error{fs.ErrNotExist}
	case http.StatusUnauthorized:
		errs = []error{fsutil.ErrSourceUnauthorized, fsutil.ErrSourceUnavailable}
	default:
		errs = []error{fsutil.ErrSourceUnavailable}
	}
	if e.cause != nil {
		errs = append(errs, e.cause)
	}
	return errs
}

func isRetryable(err error) bool {
	var e *Error
	if !errors.As(err, &e) {
		return false
	}
	if e.transport {
		return !errors.Is(e.cause, context.Canceled) && !errors.Is(e.cause, context.DeadlineExceeded)
	}
	return retryableStatus[e.Status]
}
