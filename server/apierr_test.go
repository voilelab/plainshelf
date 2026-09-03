package server

import (
	"encoding/json"
	"errors"
	"maps"
	"net/http"
	"net/http/httptest"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/voilelab/plainshelf/internal/fsutil"
	"github.com/voilelab/plainshelf/internal/taskutil"
	"github.com/voilelab/plainshelf/internal/util"
	"github.com/voilelab/plainshelf/shelf"
)

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
			name:        "invalid folder",
			err:         shelf.ErrInvalidFolder,
			wantStatus:  http.StatusBadRequest,
			wantMessage: "invalid folder name",
		},
		{
			name:        "ignored folder name",
			err:         shelf.ErrIgnoredFolderName,
			wantStatus:  http.StatusBadRequest,
			wantMessage: "invalid folder name: this name is skipped by the shelf scanner, so a folder named this way would not stay visible",
		},
		{
			// The shelf decides which names it skips, so the rejection carries
			// the name and the reason out and the API says both.
			name:        "ignored folder name with a reason",
			err:         &shelf.IgnoredFolderNameError{Folder: "@eaDir", Reason: "Synology index and thumbnail sidecar"},
			wantStatus:  http.StatusBadRequest,
			wantMessage: `invalid folder name: this shelf skips "@eaDir" while scanning (Synology index and thumbnail sidecar), so a folder named this way would not stay visible`,
		},
		{
			name:        "ignored folder name a shelf did not explain",
			err:         &shelf.IgnoredFolderNameError{Folder: "Thumbs"},
			wantStatus:  http.StatusBadRequest,
			wantMessage: `invalid folder name: this shelf skips "Thumbs" while scanning, so a folder named this way would not stay visible`,
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
			name:        "read-only shelf",
			err:         fsutil.ErrReadOnly,
			wantStatus:  http.StatusConflict,
			wantMessage: "shelf is opened read-only; this PlainShelf instance cannot modify it",
		},
		{
			name:        "unsupported book schema version",
			err:         shelf.ErrUnsupportedBookSchemaVersion,
			wantStatus:  http.StatusConflict,
			wantMessage: "book uses a newer on-disk format than this PlainShelf build supports; upgrade PlainShelf to modify it",
		},
		{
			name:        "unsupported trash schema version",
			err:         shelf.ErrUnsupportedTrashSchemaVersion,
			wantStatus:  http.StatusConflict,
			wantMessage: "trashed book uses a newer on-disk format than this PlainShelf build supports; upgrade PlainShelf to modify it",
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
			// Handlers never see a bare sentinel.
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
	app := newTestApp(t)

	rec := httptest.NewRecorder()
	app.handlers.core.writeErr(rec, httptest.NewRequest(http.MethodPost, "/api/shelves/s/trash", nil),
		util.Errorf("%w", shelf.ErrShelfLockTimeout), "failed to restore trashed book")

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
	if got := rec.Header().Get("Retry-After"); got != "5" {
		t.Fatalf("Retry-After = %q, want %q", got, "5")
	}
	got := decodeErrorEnvelope(t, rec)
	if got.Code != "SHELF_LOCK_TIMEOUT" {
		t.Fatalf("code = %q, want %q", got.Code, "SHELF_LOCK_TIMEOUT")
	}
	if got.Message != "shelf is busy, please retry shortly" {
		t.Fatalf("message = %q, want the shelf-busy message", got.Message)
	}
}

// An error the table does not know must not reach the client verbatim: the
// caller's fallback is sent instead, so internal detail stays in the log.
func TestWriteErrHidesUnknownErrorBehindFallback(t *testing.T) {
	app := newTestApp(t)

	rec := httptest.NewRecorder()
	app.handlers.core.writeErr(rec, httptest.NewRequest(http.MethodPost, "/api/shelves/s/trash", nil),
		errors.New("disk offline at /srv/secret-mount"), "failed to restore trashed book")

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	got := decodeErrorEnvelope(t, rec)
	if got.Code != "INTERNAL" {
		t.Fatalf("code = %q, want %q", got.Code, "INTERNAL")
	}
	if got.Message != "failed to restore trashed book" {
		t.Fatalf("message = %q, want the fallback message", got.Message)
	}
	if body := rec.Body.String(); strings.Contains(body, "secret-mount") {
		t.Fatalf("body = %q, must not leak the underlying error", body)
	}
}

// decodeErrorEnvelope reads the JSON envelope writeErr answers with.
func decodeErrorEnvelope(t *testing.T, rec *httptest.ResponseRecorder) apiErrorDetail {
	t.Helper()

	var body apiErrorBody
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode error body %q: %v", rec.Body.String(), err)
	}
	return body.Error
}

// taskutil.ErrTaskChainRunning answers with a JSON body carrying the running
// chain's ID, which the table cannot express, so it must stay out of it.
func TestAPIErrorForExcludesTaskChainRunning(t *testing.T) {
	if _, ok := apiErrorFor(util.Errorf("%w", taskutil.ErrTaskChainRunning)); ok {
		t.Fatal("ErrTaskChainRunning must not be in the table; the task handlers answer it with a JSON body")
	}
}

// ErrIgnoredFolderName also matches ErrInvalidFolder, so the table's ordering
// rule is what makes the specific message reachable at all.
func TestAPIErrorForPrefersIgnoredFolderNameOverInvalidFolder(t *testing.T) {
	if !errors.Is(shelf.ErrIgnoredFolderName, shelf.ErrInvalidFolder) {
		t.Fatal("ErrIgnoredFolderName must stay an ErrInvalidFolder for callers that only classify folder errors")
	}

	resp, ok := apiErrorFor(util.Errorf("%w", shelf.ErrIgnoredFolderName))
	if !ok {
		t.Fatal("apiErrorFor(ErrIgnoredFolderName) not found in table")
	}
	if resp.message == "invalid folder name" {
		t.Fatalf("message = %q, want the ignored-name explanation; the entry must precede ErrInvalidFolder", resp.message)
	}
}

// Error codes are a public interface: a user quotes one in a bug report and a
// client switches on it, so a rename is a breaking change for both. This pins
// the whole list, which is why it is spelled out here rather than derived from
// the table - a test that read the table would agree with any edit to it.
//
// Changing this list is allowed. Changing it *silently* is not: an edit here is
// the moment to decide whether callers can follow.
func TestAPIErrorCodeListIsPinned(t *testing.T) {
	want := []string{
		"INVALID_IDENTIFIER_KEY",
		"INVALID_STAR",
		"INVALID_LANGUAGE_TAG",
		"INVALID_BOOK_FORMAT",
		"IGNORED_FOLDER_NAME",
		"INVALID_FOLDER",
		"SHELF_INITIALIZING",
		"SHELF_LOCK_TIMEOUT",
		"BOOK_NOT_FOUND",
		"BOOK_ID_CONFLICT",
		"TRASHED_BOOK_NOT_FOUND",
		"SOURCE_NOT_FOUND",
		"INVALID_ASSET_NAME",
		"ASSET_NOT_FOUND",
		"SHELF_READ_ONLY",
		"UNSUPPORTED_BOOK_SCHEMA_VERSION",
		"UNSUPPORTED_SOURCE_SCHEMA_VERSION",
		"UNSUPPORTED_TRASH_SCHEMA_VERSION",
		"WORKER_BUSY",
	}

	got := make([]string, 0, len(apiErrorTable))
	for _, entry := range apiErrorTable {
		got = append(got, entry.response.code)
	}

	if !slices.Equal(got, want) {
		t.Fatalf("codes =\n\t%q\nwant\n\t%q", got, want)
	}

	// The two codes no sentinel owns are part of the same published list.
	for _, code := range []string{codeInternal, codeUnknown} {
		if slices.Contains(want, code) {
			t.Errorf("code %q is both a table entry and a fallback", code)
		}
	}
}

// A code identifies one error, so two entries sharing one would send a reporter
// to the wrong row of the table, and an empty one identifies nothing.
func TestAPIErrorCodesAreUniqueAndWellFormed(t *testing.T) {
	wellFormed := regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`)

	seen := map[string]error{}
	for _, entry := range apiErrorTable {
		code := entry.response.code
		if !wellFormed.MatchString(code) {
			t.Errorf("code %q for %v is not SCREAMING_SNAKE", code, entry.sentinel)
		}
		if other, dup := seen[code]; dup {
			t.Errorf("code %q is shared by %v and %v", code, other, entry.sentinel)
		}
		seen[code] = entry.sentinel
	}
}

// The ignored-folder rejection is matched by errors.As ahead of the table, so
// it has its own code assignment that the table pin above cannot reach.
func TestIgnoredFolderNameErrorKeepsTheTablesCode(t *testing.T) {
	tableEntry, ok := apiErrorFor(util.Errorf("%w", shelf.ErrIgnoredFolderName))
	if !ok {
		t.Fatal("apiErrorFor(ErrIgnoredFolderName) not found in table")
	}

	specific, ok := apiErrorFor(&shelf.IgnoredFolderNameError{Folder: "@eaDir"})
	if !ok {
		t.Fatal("apiErrorFor(*IgnoredFolderNameError) not matched")
	}

	if specific.code != tableEntry.code {
		t.Fatalf("code = %q, want the table's %q: the richer message is the same "+
			"refusal, so a client cannot be made to switch on two codes for it",
			specific.code, tableEntry.code)
	}
}

// writeErrStatus lets a handler pick a non-5xx status for an error the table
// cannot name. Those are the caller's fault, not the server's, so they must not
// be reported as INTERNAL - a reporter would go looking for a server bug.
func TestWriteErrStatusSeparatesUnknownFromInternal(t *testing.T) {
	app := newTestApp(t)

	tests := []struct {
		name   string
		status int
		want   string
	}{
		{"caller error", http.StatusBadRequest, codeUnknown},
		{"caller conflict", http.StatusConflict, codeUnknown},
		{"server error", http.StatusInternalServerError, codeInternal},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			app.handlers.core.writeErrStatus(rec, httptest.NewRequest(http.MethodGet, "/api/meta", nil),
				errors.New("unnamed"), "fallback", tt.status)

			if rec.Code != tt.status {
				t.Fatalf("status = %d, want %d", rec.Code, tt.status)
			}
			if got := decodeErrorEnvelope(t, rec).Code; got != tt.want {
				t.Fatalf("code = %q, want %q", got, tt.want)
			}
		})
	}
}

// The envelope is what every client parses, so its content type and nesting are
// pinned here rather than left to writeJSON's success-path tests.
func TestWriteErrBodyShape(t *testing.T) {
	app := newTestApp(t)

	rec := httptest.NewRecorder()
	app.handlers.core.writeErr(rec, httptest.NewRequest(http.MethodGet, "/api/meta", nil),
		util.Errorf("%w", shelf.ErrBookNotFound), "unused")

	if got := rec.Header().Get("Content-Type"); got != "application/json; charset=utf-8" {
		t.Fatalf("Content-Type = %q, want the JSON content type", got)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatalf("decode %q: %v", rec.Body.String(), err)
	}
	if len(raw) != 1 {
		t.Fatalf("top-level keys = %v, want only \"error\"", slices.Sorted(maps.Keys(raw)))
	}

	// A request that never passed through the middleware has no ID to quote,
	// and the envelope omits the field rather than sending an empty string a
	// client would have to special-case. The desktop client reaches handlers
	// that way.
	var detail map[string]any
	if err := json.Unmarshal(raw["error"], &detail); err != nil {
		t.Fatalf("decode error object: %v", err)
	}
	if _, present := detail["incident"]; present {
		t.Errorf("incident is present for a request that carries no ID: %v", detail)
	}
}
