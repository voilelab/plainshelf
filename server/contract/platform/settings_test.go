package platform_test

import (
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/voilelab/plainshelf/server/contract/apitest"

	"github.com/voilelab/plainshelf/internal/logutil"
	"github.com/voilelab/plainshelf/internal/testutil"
)

func settingValue[T any](t *testing.T, env *apitest.Env, key string) T {
	t.Helper()

	rec := env.Get(apitest.SettingURL(key))
	apitest.AssertStatus(t, rec, http.StatusOK)
	apitest.AssertJSONContentType(t, rec)

	value, _ := apitest.DecodeJSON[map[string]any](t, rec)["value"].(T)
	return value
}

func setSetting(t *testing.T, env *apitest.Env, key, body string, wantStatus int) {
	t.Helper()

	rec := env.Post(apitest.SettingURL(key), strings.NewReader(body))
	apitest.AssertStatus(t, rec, wantStatus)
}

func deleteSetting(t *testing.T, env *apitest.Env, key string) {
	t.Helper()

	rec := env.Delete(apitest.SettingURL(key))
	apitest.AssertStatus(t, rec, http.StatusNoContent)
}

func TestAPISettingEPUBImportStrategyContract(t *testing.T) {
	env := apitest.New(t)
	const key = "epub_import_strategy"

	value := settingValue[map[string]any](t, env, key)
	if preset, _ := value["preset"].(string); preset != "markdown" {
		t.Fatalf("default preset = %q, want markdown", preset)
	}
	if include, _ := value["include_description"].(bool); !include {
		t.Fatal("default include_description = false, want true")
	}

	setSetting(t, env, key, `{"preset":"plain","include_description":false}`, http.StatusNoContent)

	value = settingValue[map[string]any](t, env, key)
	if preset, _ := value["preset"].(string); preset != "plain" {
		t.Fatalf("preset after set = %q, want plain", preset)
	}

	imported := apitest.ImportFileBook(t, env, "uses-default.epub", "application/epub+zip", string(testutil.BuildTestEPUB(t)))
	if imported.Meta.Format != "txt" {
		t.Fatalf("format = %q, want txt from the configured default", imported.Meta.Format)
	}

	for _, body := range []string{
		`{"preset":"custom"}`,
		`{"include_description":true}`,
		`{"preset":"plain","template":"x"}`,
		`not json`,
	} {
		setSetting(t, env, key, body, http.StatusBadRequest)
	}

	deleteSetting(t, env, key)

	value = settingValue[map[string]any](t, env, key)
	if preset, _ := value["preset"].(string); preset != "markdown" {
		t.Fatalf("preset after delete = %q, want markdown", preset)
	}
}

func TestAPISettingCoverToJPGContract(t *testing.T) {
	env := apitest.New(t)
	const key = "cover_to_jpg"

	if got := settingValue[bool](t, env, key); got {
		t.Fatalf("default cover_to_jpg = %v, want false", got)
	}

	for _, want := range []bool{true, false} {
		setSetting(t, env, key, strconv.FormatBool(want), http.StatusNoContent)
		if got := settingValue[bool](t, env, key); got != want {
			t.Fatalf("cover_to_jpg after set = %v, want %v", got, want)
		}
	}

	setSetting(t, env, key, "maybe", http.StatusBadRequest)

	setSetting(t, env, key, "true", http.StatusNoContent)
	deleteSetting(t, env, key)
	if got := settingValue[bool](t, env, key); got {
		t.Fatalf("cover_to_jpg after delete = %v, want false (apitest.AppConf default)", got)
	}
}

func TestAPISettingShowNSFWContract(t *testing.T) {
	env := apitest.New(t)
	const key = "show_nsfw"

	if got := settingValue[bool](t, env, key); got {
		t.Fatalf("default show_nsfw = %v, want false", got)
	}

	for _, want := range []bool{true, false} {
		setSetting(t, env, key, strconv.FormatBool(want), http.StatusNoContent)
		if got := settingValue[bool](t, env, key); got != want {
			t.Fatalf("show_nsfw after set = %v, want %v", got, want)
		}
	}

	setSetting(t, env, key, `"yes"`, http.StatusBadRequest)

	setSetting(t, env, key, "true", http.StatusNoContent)
	deleteSetting(t, env, key)
	if got := settingValue[bool](t, env, key); got {
		t.Fatalf("show_nsfw after delete = %v, want the hidden default", got)
	}
}

func TestAPISettingLogRetentionDaysContract(t *testing.T) {
	logDir := t.TempDir()
	env := apitest.New(t, apitest.WithAppLogDir(logDir, "app"))
	const key = "log_retention_days"

	if got := settingValue[float64](t, env, key); int(got) != logutil.DefaultRetentionDays {
		t.Fatalf("default log_retention_days = %v, want %d", got, logutil.DefaultRetentionDays)
	}

	for _, want := range []int{0, 7, apitest.MaxLogRetentionDays} {
		setSetting(t, env, key, strconv.Itoa(want), http.StatusNoContent)
		if got := settingValue[float64](t, env, key); int(got) != want {
			t.Fatalf("log_retention_days after set = %v, want %d", got, want)
		}
	}

	for _, body := range []string{"-1", strconv.Itoa(apitest.MaxLogRetentionDays + 1), "1.5", `"30"`, "not json"} {
		setSetting(t, env, key, body, http.StatusBadRequest)
	}

	if got := settingValue[float64](t, env, key); int(got) != apitest.MaxLogRetentionDays {
		t.Fatalf("log_retention_days after rejected writes = %v, want %d", got, apitest.MaxLogRetentionDays)
	}

	deleteSetting(t, env, key)
	if got := settingValue[float64](t, env, key); int(got) != logutil.DefaultRetentionDays {
		t.Fatalf("log_retention_days after delete = %v, want %d", got, logutil.DefaultRetentionDays)
	}
}

func TestAPILogRetentionAppliesLive(t *testing.T) {
	logDir := t.TempDir()
	env := apitest.New(t, apitest.WithAppLogDir(logDir, "app"))

	setSetting(t, env, "log_retention_days", "0", http.StatusNoContent)

	writer := logutil.NewDailyFileWriter(env.App.Conf().Logger.LogFile)
	t.Cleanup(func() { _ = writer.Close() })

	apitest.WriteLogFile(t, filepath.Join(logDir, "app-2000-01-01.log"), "ancient")
	if _, err := writer.Write([]byte("{}\n")); err != nil {
		t.Fatalf("Write: %v", err)
	}

	if _, err := os.Stat(filepath.Join(logDir, "app-2000-01-01.log")); err != nil {
		t.Fatalf("Stat ancient log after a rotation with retention off: %v", err)
	}
}
