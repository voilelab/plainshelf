package platform_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/voilelab/plainshelf/server/contract/apitest"

	"github.com/voilelab/plainshelf/internal/logutil"
	"github.com/voilelab/plainshelf/server"
)

func TestAPIGetLogsContract(t *testing.T) {
	appLogFile := filepath.Join(t.TempDir(), "app.log")
	shelfLogDir := t.TempDir()

	// The app is left unstarted, so the only file the shelf logger has is the
	// seeded one. The app logger writes to its own file as requests are served.
	env := apitest.NewUnstarted(t,
		apitest.WithAppLogFile(appLogFile),
		apitest.WithShelfLogDir(shelfLogDir, "shelf"),
		apitest.WithLogRetention(0),
	)

	apitest.WriteLogFile(t, appLogFile, "app log")
	appLogTime := time.Date(2024, 1, 3, 12, 0, 0, 0, time.UTC)
	if err := os.Chtimes(appLogFile, appLogTime, appLogTime); err != nil {
		t.Fatalf("Chtimes app log: %v", err)
	}
	apitest.WriteLogFile(t, filepath.Join(shelfLogDir, "shelf-2024-01-02.log"), "shelf log")

	// A file that does not look like a log is not listed.
	apitest.WriteLogFile(t, filepath.Join(shelfLogDir, "ignore.txt"), "nope")

	rec := env.Get(apitest.LogURL())
	apitest.AssertStatus(t, rec, http.StatusOK)
	apitest.AssertJSONContentType(t, rec)

	logs := apitest.DecodeJSON[[]server.LogFileEntry](t, rec)
	if len(logs) != 2 {
		t.Fatalf("log count = %d, want 2", len(logs))
	}

	if logs[0].ID == "" || logs[0].Source != "logger" || logs[0].Filename != "app.log" || logs[0].Date == "" {
		t.Fatalf("first log = %#v, want app logger entry", logs[0])
	}
	if logs[1].ID == "" || logs[1].Source != "shelves[0].shelfconf.logger" || logs[1].Filename != "shelf-2024-01-02.log" || logs[1].Date != "2024-01-02" {
		t.Fatalf("second log = %#v, want shelf logger entry", logs[1])
	}

	// The size is what tells a caller reading a bounded tail that it is holding
	// only the end of the file. The app's own log grows as requests are served,
	// so it is checked against the file rather than against what was seeded.
	info, err := os.Stat(appLogFile)
	if err != nil {
		t.Fatalf("Stat app log: %v", err)
	}
	if logs[0].Size != info.Size() {
		t.Fatalf("app log size = %d, want %d", logs[0].Size, info.Size())
	}
	if logs[1].Size != int64(len("shelf log")) {
		t.Fatalf("shelf log size = %d, want %d", logs[1].Size, len("shelf log"))
	}
}

func TestAPIGetLogContentContract(t *testing.T) {
	logDir := t.TempDir()
	// Retention is off so the past-dated fixture survives the rotation the
	// app's own logging performs into today's file.
	env := apitest.NewUnstarted(t, apitest.WithAppLogDir(logDir, "app"), apitest.WithLogRetention(0))

	apitest.WriteLogFile(t, filepath.Join(logDir, "app-2024-01-02.log"), "line 1\nline 2\n")

	target := apitest.LogEntryByFilename(t, env, "app-2024-01-02.log")

	rec := env.Get(apitest.LogURL(target.ID, "content"))
	apitest.AssertStatus(t, rec, http.StatusOK)
	apitest.AssertContentType(t, rec, apitest.PlainTextContentType)
	if got := rec.Body.String(); got != "line 1\nline 2\n" {
		t.Fatalf("content = %q, want seeded log content", got)
	}

	rec = env.Get(apitest.LogURL("missing", "content"))
	apitest.AssertStatus(t, rec, http.StatusNotFound)
}

// TestAPIGetLogContentTailContract pins the bound on the content route: a log
// file grows without limit, so the default response is its end rather than all
// of it, and the cut lands on a line boundary.
func TestAPIGetLogContentTailContract(t *testing.T) {
	logDir := t.TempDir()
	env := apitest.NewUnstarted(t, apitest.WithAppLogDir(logDir, "app"), apitest.WithLogRetention(0))

	apitest.WriteLogFile(t, filepath.Join(logDir, "app-2024-01-02.log"), "aaaa\nbbbb\ncccc\n")

	target := apitest.LogEntryByFilename(t, env, "app-2024-01-02.log")

	// A tail that falls inside a line is moved forward to the next one.
	rec := env.Get(apitest.LogURL(target.ID, "content") + "?tail_bytes=12")
	apitest.AssertStatus(t, rec, http.StatusOK)
	apitest.AssertContentType(t, rec, apitest.PlainTextContentType)
	if got := rec.Body.String(); got != "bbbb\ncccc\n" {
		t.Fatalf("tail = %q, want whole trailing lines", got)
	}

	// An explicit zero is how a caller asks for the whole file.
	rec = env.Get(apitest.LogURL(target.ID, "content") + "?tail_bytes=0")
	apitest.AssertStatus(t, rec, http.StatusOK)
	if got := rec.Body.String(); got != "aaaa\nbbbb\ncccc\n" {
		t.Fatalf("untailed content = %q, want the whole file", got)
	}

	for _, raw := range []string{"-1", "abc", "1.5"} {
		rec = env.Get(apitest.LogURL(target.ID, "content") + "?tail_bytes=" + raw)
		apitest.AssertStatus(t, rec, http.StatusBadRequest)
	}
}

// TestAPIGetLogContentDefaultsToABoundedTail keeps the unparameterized route
// bounded: an existing caller that asks for nothing must not be handed a file
// of unlimited size.
func TestAPIGetLogContentDefaultsToABoundedTail(t *testing.T) {
	logDir := t.TempDir()
	env := apitest.NewUnstarted(t, apitest.WithAppLogDir(logDir, "app"), apitest.WithLogRetention(0))

	line := strings.Repeat("x", 1023) + "\n"
	lines := int(logutil.DefaultTailBytes)/len(line) + 8
	apitest.WriteLogFile(t, filepath.Join(logDir, "app-2024-01-02.log"), strings.Repeat(line, lines))

	target := apitest.LogEntryByFilename(t, env, "app-2024-01-02.log")

	rec := env.Get(apitest.LogURL(target.ID, "content"))
	apitest.AssertStatus(t, rec, http.StatusOK)

	body := rec.Body.String()
	if int64(len(body)) > logutil.DefaultTailBytes {
		t.Fatalf("default response = %d bytes, want at most %d", len(body), logutil.DefaultTailBytes)
	}
	// Aligning to a line boundary drops at most one whole line off the front.
	if int64(len(body)) < logutil.DefaultTailBytes-int64(len(line)) {
		t.Fatalf("default response = %d bytes, want close to the %d-byte window", len(body), logutil.DefaultTailBytes)
	}
	if !strings.HasPrefix(body, line) || !strings.HasSuffix(body, line) {
		t.Fatal("default response does not start and end on a line boundary")
	}
	if int64(target.Size) <= logutil.DefaultTailBytes {
		t.Fatalf("fixture size = %d, want larger than the %d-byte window", target.Size, logutil.DefaultTailBytes)
	}
}

// The log API stays behind the token whatever protect_read says: a log records
// every request path, access time and remote address, which is the shelf's
// structure and its usage rather than the books a reader came for.
func TestAPILogsRequireTokenWithoutProtectReadContract(t *testing.T) {
	logDir := t.TempDir()
	security := apitest.LocalTokenSecurity()
	security.ProtectRead = false
	env := apitest.NewUnstarted(t, apitest.WithAppLogDir(logDir, "app"), apitest.WithSecurity(security), apitest.WithLogRetention(0))

	apitest.WriteLogFile(t, filepath.Join(logDir, "app-2024-01-02.log"), "line 1\n")

	rec := env.DoRaw(httptest.NewRequest(http.MethodGet, apitest.LogURL(), nil))
	apitest.AssertStatus(t, rec, http.StatusUnauthorized)

	// An ordinary read is still open, so this is the log routes being excepted
	// rather than protect_read having been ignored. A setting is read instead of
	// the book list because this env is deliberately unstarted, and a shelf that
	// has not finished its initial scan answers 503.
	rec = env.DoRaw(httptest.NewRequest(http.MethodGet, apitest.SettingPath, nil))
	apitest.AssertStatus(t, rec, http.StatusOK)

	// With the token the page works unchanged.
	target := apitest.LogEntryByFilename(t, env, "app-2024-01-02.log")

	rec = env.DoRaw(httptest.NewRequest(http.MethodGet, apitest.LogURL(target.ID, "content"), nil))
	apitest.AssertStatus(t, rec, http.StatusUnauthorized)

	rec = env.Get(apitest.LogURL(target.ID, "content"))
	apitest.AssertStatus(t, rec, http.StatusOK)
	if got := rec.Body.String(); got != "line 1\n" {
		t.Fatalf("content = %q, want seeded log content", got)
	}
}

// Security mode "none" is an explicit "do not authenticate", so the exception
// above does not reintroduce a gate the operator turned off.
func TestAPILogsOpenUnderSecurityModeNoneContract(t *testing.T) {
	logDir := t.TempDir()
	env := apitest.NewUnstarted(t,
		apitest.WithAppLogDir(logDir, "app"),
		apitest.WithSecurity(&server.SecurityConf{Mode: server.SecurityModeNone}),
		apitest.WithLogRetention(0))

	apitest.WriteLogFile(t, filepath.Join(logDir, "app-2024-01-02.log"), "line 1\n")

	rec := env.DoRaw(httptest.NewRequest(http.MethodGet, apitest.LogURL(), nil))
	apitest.AssertStatus(t, rec, http.StatusOK)
}
