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

func TestAPIGetLogContentTailContract(t *testing.T) {
	logDir := t.TempDir()
	env := apitest.NewUnstarted(t, apitest.WithAppLogDir(logDir, "app"), apitest.WithLogRetention(0))

	apitest.WriteLogFile(t, filepath.Join(logDir, "app-2024-01-02.log"), "aaaa\nbbbb\ncccc\n")

	target := apitest.LogEntryByFilename(t, env, "app-2024-01-02.log")

	rec := env.Get(apitest.LogURL(target.ID, "content") + "?tail_bytes=12")
	apitest.AssertStatus(t, rec, http.StatusOK)
	apitest.AssertContentType(t, rec, apitest.PlainTextContentType)
	if got := rec.Body.String(); got != "bbbb\ncccc\n" {
		t.Fatalf("tail = %q, want whole trailing lines", got)
	}

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

func TestAPILogsRequireTokenWithoutProtectReadContract(t *testing.T) {
	logDir := t.TempDir()
	security := apitest.LocalTokenSecurity()
	security.ProtectRead = false
	env := apitest.NewUnstarted(t, apitest.WithAppLogDir(logDir, "app"), apitest.WithSecurity(security), apitest.WithLogRetention(0))

	apitest.WriteLogFile(t, filepath.Join(logDir, "app-2024-01-02.log"), "line 1\n")

	rec := env.DoRaw(httptest.NewRequest(http.MethodGet, apitest.LogURL(), nil))
	apitest.AssertStatus(t, rec, http.StatusUnauthorized)

	rec = env.DoRaw(httptest.NewRequest(http.MethodGet, apitest.SettingPath, nil))
	apitest.AssertStatus(t, rec, http.StatusOK)

	target := apitest.LogEntryByFilename(t, env, "app-2024-01-02.log")

	rec = env.DoRaw(httptest.NewRequest(http.MethodGet, apitest.LogURL(target.ID, "content"), nil))
	apitest.AssertStatus(t, rec, http.StatusUnauthorized)

	rec = env.Get(apitest.LogURL(target.ID, "content"))
	apitest.AssertStatus(t, rec, http.StatusOK)
	if got := rec.Body.String(); got != "line 1\n" {
		t.Fatalf("content = %q, want seeded log content", got)
	}
}

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
