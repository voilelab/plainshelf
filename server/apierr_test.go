package server

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/voilelab/plainshelf/internal/taskutil"
	"github.com/voilelab/plainshelf/internal/util"
	"github.com/voilelab/plainshelf/shelf"
)

// These pin the status, message, and Retry-After of every error the API maps,
// because those values were previously spread over one shared helper and
// several per-handler chains that had already drifted apart.
func TestAPIErrorForKnownSentinels(t *testing.T) {
	tests := []struct {
		name           string
		err            error
		wantStatus     int
		wantMessage    string
		wantRetryAfter string
	}{
		{
			name:        "invalid identifier key",
			err:         shelf.ErrInvalidIdentifierKey,
			wantStatus:  http.StatusBadRequest,
			wantMessage: "identifier key cannot be empty",
		},
		{
			name:        "invalid language tag",
			err:         shelf.ErrInvalidLanguageTag,
			wantStatus:  http.StatusBadRequest,
			wantMessage: "language must be a BCP 47 tag",
		},
		{
			name:           "shelf initializing",
			err:            shelf.ErrShelfInitializing,
			wantStatus:     http.StatusServiceUnavailable,
			wantMessage:    "shelf is initializing, please retry shortly",
			wantRetryAfter: "3",
		},
		{
			name:           "shelf lock timeout",
			err:            shelf.ErrShelfLockTimeout,
			wantStatus:     http.StatusServiceUnavailable,
			wantMessage:    "shelf is busy, please retry shortly",
			wantRetryAfter: "5",
		},
		{
			name:        "book not found",
			err:         shelf.ErrBookNotFound,
			wantStatus:  http.StatusNotFound,
			wantMessage: "book not found",
		},
		{
			name:        "trashed book not found",
			err:         shelf.ErrTrashedBookNotFound,
			wantStatus:  http.StatusNotFound,
			wantMessage: "trashed book not found",
		},
		{
			name:        "source not found",
			err:         shelf.ErrSourceNotFound,
			wantStatus:  http.StatusNotFound,
			wantMessage: "source not found",
		},
		{
			name:        "unsupported book schema version",
			err:         shelf.ErrUnsupportedBookSchemaVersion,
			wantStatus:  http.StatusConflict,
			wantMessage: "book uses a newer on-disk format than this PlainShelf build supports; upgrade PlainShelf to modify it",
		},
		{
			name:           "worker busy",
			err:            taskutil.ErrWorkerBusy,
			wantStatus:     http.StatusServiceUnavailable,
			wantMessage:    "background worker is busy",
			wantRetryAfter: "5",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Wrapped, because handlers never see a bare sentinel: the shelf
			// package returns it through util.Errorf.
			wrapped := util.Errorf("%w", tt.err)

			resp, ok := apiErrorFor(wrapped)
			if !ok {
				t.Fatalf("apiErrorFor(%v) not found in table", tt.err)
			}
			if resp.status != tt.wantStatus {
				t.Errorf("status = %d, want %d", resp.status, tt.wantStatus)
			}
			if resp.message != tt.wantMessage {
				t.Errorf("message = %q, want %q", resp.message, tt.wantMessage)
			}
			if resp.retryAfter != tt.wantRetryAfter {
				t.Errorf("retryAfter = %q, want %q", resp.retryAfter, tt.wantRetryAfter)
			}
		})
	}
}

func TestWriteErrSendsRetryAfterHeader(t *testing.T) {
	env := newAPITestEnv(t)

	rec := httptest.NewRecorder()
	env.app.writeErr(rec, util.Errorf("%w", shelf.ErrShelfLockTimeout), "failed to restore trashed book")

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
	if got := rec.Header().Get("Retry-After"); got != "5" {
		t.Fatalf("Retry-After = %q, want %q", got, "5")
	}
	if got := strings.TrimSpace(rec.Body.String()); got != "shelf is busy, please retry shortly" {
		t.Fatalf("body = %q, want the shelf-busy message", got)
	}
}

// An error the table does not know must not reach the client verbatim: the
// caller's fallback is sent instead, so internal detail stays in the log.
func TestWriteErrHidesUnknownErrorBehindFallback(t *testing.T) {
	env := newAPITestEnv(t)

	rec := httptest.NewRecorder()
	env.app.writeErr(rec, errors.New("disk offline at /srv/secret-mount"), "failed to restore trashed book")

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	body := rec.Body.String()
	if strings.TrimSpace(body) != "failed to restore trashed book" {
		t.Fatalf("body = %q, want the fallback message", body)
	}
	if strings.Contains(body, "secret-mount") {
		t.Fatalf("body = %q, must not leak the underlying error", body)
	}
}

// taskutil.ErrTaskChainRunning answers with a JSON body carrying the running
// chain's ID, which the table cannot express, so it must stay out of it.
func TestAPIErrorForExcludesTaskChainRunning(t *testing.T) {
	if _, ok := apiErrorFor(util.Errorf("%w", taskutil.ErrTaskChainRunning)); ok {
		t.Fatal("ErrTaskChainRunning must not be in the table; the task handlers answer it with a JSON body")
	}
}
