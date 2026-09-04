package contract_test

import (
	"bufio"
	"bytes"
	"encoding/json/v2"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// requestIDConfusables are the characters a request ID must never contain. The
// ID exists to be read aloud and copied by hand off a screen into a bug report,
// so 0 against O and 1 against I or L would each cost a lookup that fails for a
// reason the user cannot see.
const requestIDConfusables = "0O1IL"

// assertRequestIDShape pins what a user has to be able to transcribe: eight
// characters, no pair anyone confuses.
func assertRequestIDShape(t *testing.T, id string) {
	t.Helper()

	if len(id) != 8 {
		t.Fatalf("request ID = %q, want 8 characters", id)
	}
	for _, c := range id {
		if strings.ContainsRune(requestIDConfusables, c) {
			t.Fatalf("request ID = %q contains the confusable %q", id, c)
		}
		if !strings.ContainsRune("23456789ABCDEFGHJKMNPQRSTVWXYZ", c) {
			t.Fatalf("request ID = %q contains %q, which is outside Crockford base32 without 0 and 1", id, c)
		}
	}
}

// Every response carries the ID, not only the failures: a user reporting a
// screen that looks wrong without erroring has something to quote too, which is
// the whole reason the ID is per request rather than per error.
func TestAPIEveryResponseCarriesARequestIDContract(t *testing.T) {
	env := New(t)

	seen := map[string]struct{}{}
	for _, url := range []string{"/api/version", "/api/shelves", ShelfURL("books"), BookURL("no_such_book")} {
		rec := env.Get(url)

		id := rec.Header().Get("X-Request-Id")
		if id == "" {
			t.Fatalf("GET %s answered without an X-Request-Id header", url)
		}
		assertRequestIDShape(t, id)

		if _, repeated := seen[id]; repeated {
			t.Fatalf("GET %s reused the request ID %q", url, id)
		}
		seen[id] = struct{}{}
	}
}

// The number the user reads off the error and the number in the header are one
// number. A second identifier for the same request would mean the developer has
// to know which of the two the user copied.
func TestAPIErrorEnvelopeIncidentIsTheRequestIDContract(t *testing.T) {
	env := New(t)

	// A refusal the table names, which is the case a per-error incident ID
	// would have left without a number to quote.
	rec := env.Request(http.MethodDelete, BookURL("no_such_book"), nil)
	AssertErrorEnvelope(t, rec, http.StatusNotFound, "BOOK_NOT_FOUND", "book not found")

	got := DecodeJSON[ErrorEnvelope](t, rec)
	header := rec.Header().Get("X-Request-Id")
	if got.Error.Incident == "" {
		t.Fatalf("incident is empty; the user has nothing to report (body: %s)", rec.Body.String())
	}
	if got.Error.Incident != header {
		t.Fatalf("incident = %q, X-Request-Id = %q; they must be the same number", got.Error.Incident, header)
	}
	assertRequestIDShape(t, got.Error.Incident)
}

// The point of the whole chain: the ID the user was shown finds the log line,
// and that line answers what the response body deliberately withheld.
func TestAPIUnknownErrorLogsTheReportedIDWithItsRouteContract(t *testing.T) {
	appLogFile := filepath.Join(t.TempDir(), "app.log")
	libRoot := t.TempDir()

	env := New(t, WithLibRoot(libRoot), WithAppLogFile(appLogFile))
	book := ImportTextBook(t, env, "Incident Book", "", "incident.txt", "body")

	sources := GetJSON[[]struct {
		ID string `json:"id"`
	}](t, env, BookURL(book.Meta.ID, "sources"))
	if len(sources) != 1 {
		t.Fatalf("sources = %d, want 1", len(sources))
	}

	// Replacing the source file with a directory makes the atomic rename fail.
	// That is an error the table cannot name, which is the fallback path this
	// test is about.
	sourceFile := findSourceFile(t, libRoot)
	if err := os.Remove(sourceFile); err != nil {
		t.Fatalf("remove source file: %v", err)
	}
	if err := os.Mkdir(sourceFile, 0755); err != nil {
		t.Fatalf("replace source file with a directory: %v", err)
	}

	contentURL := SourceURL(book.Meta.ID, sources[0].ID, "content")
	rec := env.PatchContent(contentURL, PlainTextContentType, strings.NewReader("replacement"))
	AssertStatus(t, rec, http.StatusInternalServerError)

	incident := DecodeJSON[ErrorEnvelope](t, rec).Error.Incident
	if incident == "" {
		t.Fatalf("incident is empty, so the 500 cannot be traced (body: %s)", rec.Body.String())
	}

	entry := findLogEntryByRequestID(t, appLogFile, incident, "failed to update book source content")
	for field, want := range map[string]string{
		"code":     "INTERNAL",
		"method":   http.MethodPatch,
		"path":     contentURL,
		"shelf_id": DefaultShelfID,
	} {
		if got, _ := entry[field].(string); got != want {
			t.Errorf("log %s = %q, want %q (entry: %v)", field, got, want, entry)
		}
	}
	if got, _ := entry["error"].(string); got == "" {
		t.Errorf("log error is empty, so the cause the body withheld is lost (entry: %v)", entry)
	}
}

// findLogEntryByRequestID does what a developer holding a reported number does:
// search the log for it and read the line back.
func findLogEntryByRequestID(t *testing.T, logFile, requestID, msg string) map[string]any {
	t.Helper()

	file, err := os.Open(logFile)
	if err != nil {
		t.Fatalf("open app log: %v", err)
	}
	defer func() { _ = file.Close() }()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for scanner.Scan() {
		var entry map[string]any
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			continue
		}
		if entry["request_id"] == requestID && entry["msg"] == msg {
			return entry
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("read app log: %v", err)
	}

	t.Fatalf("no %q line carries request_id %q", msg, requestID)
	return nil
}

// The request log line is what a developer searches, so it has to carry the
// reported number - and it has to keep the fields it already had, because an
// operator's existing log tooling reads them by name.
func TestAPIRequestLogLineCarriesTheRequestIDContract(t *testing.T) {
	appLogFile := filepath.Join(t.TempDir(), "app.log")
	env := New(t, WithAppLogFile(appLogFile))

	rec := env.Get(ShelfURL("books"))
	AssertStatus(t, rec, http.StatusOK)

	requestID := rec.Header().Get("X-Request-Id")
	entry := findLogEntryByRequestID(t, appLogFile, requestID, "app handler")

	for field, want := range map[string]string{
		"method": http.MethodGet,
		"path":   ShelfURL("books"),
	} {
		if got, _ := entry[field].(string); got != want {
			t.Errorf("log %s = %q, want %q (entry: %v)", field, got, want, entry)
		}
	}
	if got, _ := entry["remote_addr"].(string); got == "" {
		t.Errorf("log remote_addr is empty; the field predates this line and must stay (entry: %v)", entry)
	}
}

// A background chain fails minutes after the response that gave the user their
// number, so it logs under that same number rather than one of its own. Nothing
// else could connect the two: the user never sees the chain ID.
func TestAPIBackgroundTaskFailureLogsTheSubmittingRequestIDContract(t *testing.T) {
	appLogFile := filepath.Join(t.TempDir(), "app.log")
	env := New(t, WithAppLogFile(appLogFile))

	body, err := json.Marshal(map[string]any{
		"operation": "trash",
		"book_ids":  []string{"no_such_book"},
	})
	if err != nil {
		t.Fatalf("marshal batch request: %v", err)
	}

	rec := env.Post(BookBatchURL(), bytes.NewReader(body))
	AssertStatus(t, rec, http.StatusAccepted)

	requestID := rec.Header().Get("X-Request-Id")
	if requestID == "" {
		t.Fatal("the 202 that queued the chain carries no X-Request-Id")
	}

	accepted := DecodeJSON[TaskChainSubmitResponse](t, rec)
	chain := WaitForTaskChain(t, env, accepted.TaskChainID)
	if chain.Status != "failed" && chain.Status != "partially_completed" {
		t.Fatalf("chain status = %q, want the batch to have failed its only item", chain.Status)
	}

	entry := findLogEntryByRequestID(t, appLogFile, requestID, "book batch item failed")
	if got, _ := entry["book_id"].(string); got != "no_such_book" {
		t.Errorf("log book_id = %q, want the failing book (entry: %v)", got, entry)
	}
}
