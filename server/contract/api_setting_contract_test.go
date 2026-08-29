package contract_test

import (
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/voilelab/plainshelf/internal/logutil"
	"github.com/voilelab/plainshelf/internal/testutil"
)

func settingURL(key string) string {
	return "/api/setting/" + key
}

// settingValue reads a setting and returns the "value" the endpoint wraps it in.
// The response is decoded generically so the wrapper's field name is pinned too.
func settingValue[T any](t *testing.T, env *apiTestEnv, key string) T {
	t.Helper()

	rec := env.get(settingURL(key))
	assertStatus(t, rec, http.StatusOK)
	assertJSONContentType(t, rec)

	value, _ := decodeJSON[map[string]any](t, rec)["value"].(T)
	return value
}

// setSetting stores a setting value, asserting the status the endpoint must
// answer with — 204 for an accepted value, 400 for a rejected one.
func setSetting(t *testing.T, env *apiTestEnv, key, body string, wantStatus int) {
	t.Helper()

	rec := env.post(settingURL(key), strings.NewReader(body))
	assertStatus(t, rec, wantStatus)
}

// deleteSetting clears a stored setting, which reverts it to the built-in or
// configured default.
func deleteSetting(t *testing.T, env *apiTestEnv, key string) {
	t.Helper()

	rec := env.delete(settingURL(key))
	assertStatus(t, rec, http.StatusNoContent)
}

func TestAPISettingEPUBImportStrategyContract(t *testing.T) {
	env := newAPITestEnv(t)
	const key = "epub_import_strategy"

	// The built-in default applies when nothing is configured.
	value := settingValue[map[string]any](t, env, key)
	if preset, _ := value["preset"].(string); preset != "markdown" {
		t.Fatalf("default preset = %q, want markdown", preset)
	}
	if include, _ := value["include_description"].(bool); !include {
		t.Fatal("default include_description = false, want true")
	}

	// Setting it changes what an import with no strategy field uses.
	setSetting(t, env, key, `{"preset":"plain","include_description":false}`, http.StatusNoContent)

	value = settingValue[map[string]any](t, env, key)
	if preset, _ := value["preset"].(string); preset != "plain" {
		t.Fatalf("preset after set = %q, want plain", preset)
	}

	imported := importFileBook(t, env, "uses-default.epub", "application/epub+zip", string(testutil.BuildTestEPUB(t)))
	if imported.Meta.Format != "txt" {
		t.Fatalf("format = %q, want txt from the configured default", imported.Meta.Format)
	}

	// Invalid payloads are rejected.
	for _, body := range []string{
		`{"preset":"custom"}`,
		`{"include_description":true}`,
		`{"preset":"plain","template":"x"}`,
		`not json`,
	} {
		setSetting(t, env, key, body, http.StatusBadRequest)
	}

	// Deleting reverts to the built-in default.
	deleteSetting(t, env, key)

	value = settingValue[map[string]any](t, env, key)
	if preset, _ := value["preset"].(string); preset != "markdown" {
		t.Fatalf("preset after delete = %q, want markdown", preset)
	}
}

func TestAPISettingCoverToJPGContract(t *testing.T) {
	env := newAPITestEnv(t)
	const key = "cover_to_jpg"

	// The default value reflects AppConf, which is false in the test env.
	if got := settingValue[bool](t, env, key); got {
		t.Fatalf("default cover_to_jpg = %v, want false", got)
	}

	for _, want := range []bool{true, false} {
		setSetting(t, env, key, strconv.FormatBool(want), http.StatusNoContent)
		if got := settingValue[bool](t, env, key); got != want {
			t.Fatalf("cover_to_jpg after set = %v, want %v", got, want)
		}
	}

	// A value that is not a boolean is rejected.
	setSetting(t, env, key, "maybe", http.StatusBadRequest)

	// Deleting a stored value resets it to the AppConf default (false).
	setSetting(t, env, key, "true", http.StatusNoContent)
	deleteSetting(t, env, key)
	if got := settingValue[bool](t, env, key); got {
		t.Fatalf("cover_to_jpg after delete = %v, want false (AppConf default)", got)
	}
}

// TestAPISettingLogRetentionDaysContract pins the control that makes log
// deletion optional. It matters most on the desktop build, which has no config
// file to edit: without this route a user has no way to turn deletion off.
func TestAPISettingLogRetentionDaysContract(t *testing.T) {
	logDir := t.TempDir()
	env := newAPITestEnv(t, withAppLogDir(logDir, "app"))
	const key = "log_retention_days"

	// Nothing stored and nothing configured: the built-in window applies.
	if got := settingValue[float64](t, env, key); int(got) != logutil.DefaultRetentionDays {
		t.Fatalf("default log_retention_days = %v, want %d", got, logutil.DefaultRetentionDays)
	}

	// Zero is the meaningful end of the range: it is how deletion is turned off.
	for _, want := range []int{0, 7, maxLogRetentionDays} {
		setSetting(t, env, key, strconv.Itoa(want), http.StatusNoContent)
		if got := settingValue[float64](t, env, key); int(got) != want {
			t.Fatalf("log_retention_days after set = %v, want %d", got, want)
		}
	}

	for _, body := range []string{"-1", strconv.Itoa(maxLogRetentionDays + 1), "1.5", `"30"`, "not json"} {
		setSetting(t, env, key, body, http.StatusBadRequest)
	}

	// The rejected values left the last accepted one in place.
	if got := settingValue[float64](t, env, key); int(got) != maxLogRetentionDays {
		t.Fatalf("log_retention_days after rejected writes = %v, want %d", got, maxLogRetentionDays)
	}

	deleteSetting(t, env, key)
	if got := settingValue[float64](t, env, key); int(got) != logutil.DefaultRetentionDays {
		t.Fatalf("log_retention_days after delete = %v, want %d", got, logutil.DefaultRetentionDays)
	}
}

// TestAPISettingLogRetentionAppliesWithoutRestart covers the reason this is a
// setting rather than configuration: a user who turns deletion off must not
// have to restart the server before the running writers stop deleting.
func TestAPISettingLogRetentionAppliesWithoutRestart(t *testing.T) {
	logDir := t.TempDir()
	env := newAPITestEnv(t, withAppLogDir(logDir, "app"))

	setSetting(t, env, "log_retention_days", "0", http.StatusNoContent)

	// The app logger is the writer the running server actually holds.
	writer := logutil.NewDailyFileWriter(env.app.Conf().Logger.LogFile)
	t.Cleanup(func() { _ = writer.Close() })

	writeLogFile(t, filepath.Join(logDir, "app-2000-01-01.log"), "ancient")
	if _, err := writer.Write([]byte("{}\n")); err != nil {
		t.Fatalf("Write: %v", err)
	}

	if _, err := os.Stat(filepath.Join(logDir, "app-2000-01-01.log")); err != nil {
		t.Fatalf("Stat ancient log after a rotation with retention off: %v", err)
	}
}
