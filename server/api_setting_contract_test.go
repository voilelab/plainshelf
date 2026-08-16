package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAPISettingEPUBImportStrategyContract(t *testing.T) {
	env := newAPITestEnv(t)
	url := "/api/setting/epub_import_strategy"

	// The built-in default applies when nothing is configured.
	rec := env.do(httptest.NewRequest(http.MethodGet, url, nil))
	assertStatus(t, rec, http.StatusOK)
	assertJSONContentType(t, rec)
	got := decodeJSON[map[string]any](t, rec)
	val, _ := got["value"].(map[string]any)
	if preset, _ := val["preset"].(string); preset != "markdown" {
		t.Fatalf("default preset = %q, want markdown", preset)
	}
	if include, _ := val["include_description"].(bool); !include {
		t.Fatal("default include_description = false, want true")
	}

	// Setting it changes what an import with no strategy field uses.
	rec = env.do(httptest.NewRequest(http.MethodPost, url,
		strings.NewReader(`{"preset":"plain","include_description":false}`)))
	assertStatus(t, rec, http.StatusNoContent)

	rec = env.do(httptest.NewRequest(http.MethodGet, url, nil))
	assertStatus(t, rec, http.StatusOK)
	got = decodeJSON[map[string]any](t, rec)
	val, _ = got["value"].(map[string]any)
	if preset, _ := val["preset"].(string); preset != "plain" {
		t.Fatalf("preset after set = %q, want plain", preset)
	}

	imported := importFileBook(t, env, "uses-default.epub", "application/epub+zip", string(buildTestEPUB(t)))
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
		rec = env.do(httptest.NewRequest(http.MethodPost, url, strings.NewReader(body)))
		assertStatus(t, rec, http.StatusBadRequest)
	}

	// Deleting reverts to the built-in default.
	rec = env.do(httptest.NewRequest(http.MethodDelete, url, nil))
	assertStatus(t, rec, http.StatusNoContent)

	rec = env.do(httptest.NewRequest(http.MethodGet, url, nil))
	assertStatus(t, rec, http.StatusOK)
	got = decodeJSON[map[string]any](t, rec)
	val, _ = got["value"].(map[string]any)
	if preset, _ := val["preset"].(string); preset != "markdown" {
		t.Fatalf("preset after delete = %q, want markdown", preset)
	}
}

func TestAPISettingCoverToJPGContract(t *testing.T) {
	env := newAPITestEnv(t)
	url := "/api/setting/cover_to_jpg"

	// Default value reflects AppConf (false in test env).
	rec := env.do(httptest.NewRequest(http.MethodGet, url, nil))
	assertStatus(t, rec, http.StatusOK)
	assertJSONContentType(t, rec)
	got := decodeJSON[map[string]any](t, rec)
	if val, _ := got["value"].(bool); val != false {
		t.Fatalf("default cover_to_jpg = %v, want false", got["value"])
	}

	// Set to true.
	rec = env.do(httptest.NewRequest(http.MethodPost, url, strings.NewReader("true")))
	assertStatus(t, rec, http.StatusNoContent)

	rec = env.do(httptest.NewRequest(http.MethodGet, url, nil))
	assertStatus(t, rec, http.StatusOK)
	got = decodeJSON[map[string]any](t, rec)
	if val, _ := got["value"].(bool); val != true {
		t.Fatalf("cover_to_jpg after set = %v, want true", got["value"])
	}

	// Set to false.
	rec = env.do(httptest.NewRequest(http.MethodPost, url, strings.NewReader("false")))
	assertStatus(t, rec, http.StatusNoContent)

	rec = env.do(httptest.NewRequest(http.MethodGet, url, nil))
	assertStatus(t, rec, http.StatusOK)
	got = decodeJSON[map[string]any](t, rec)
	if val, _ := got["value"].(bool); val != false {
		t.Fatalf("cover_to_jpg after set false = %v, want false", got["value"])
	}

	// Invalid value returns 400.
	rec = env.do(httptest.NewRequest(http.MethodPost, url, strings.NewReader("maybe")))
	assertStatus(t, rec, http.StatusBadRequest)

	// Set to true then delete resets to AppConf default (false).
	rec = env.do(httptest.NewRequest(http.MethodPost, url, strings.NewReader("true")))
	assertStatus(t, rec, http.StatusNoContent)

	rec = env.do(httptest.NewRequest(http.MethodDelete, url, nil))
	assertStatus(t, rec, http.StatusNoContent)

	rec = env.do(httptest.NewRequest(http.MethodGet, url, nil))
	assertStatus(t, rec, http.StatusOK)
	got = decodeJSON[map[string]any](t, rec)
	if val, _ := got["value"].(bool); val != false {
		t.Fatalf("cover_to_jpg after delete = %v, want false (AppConf default)", got["value"])
	}
}

func TestAPISettingDefaultSplitConfigContract(t *testing.T) {
	env := newAPITestEnv(t)
	url := "/api/setting/default_split_config"

	// Default value is no splitting.
	rec := env.do(httptest.NewRequest(http.MethodGet, url, nil))
	assertStatus(t, rec, http.StatusOK)
	assertJSONContentType(t, rec)
	got := decodeJSON[map[string]any](t, rec)
	val, _ := got["value"].(map[string]any)
	if tp, _ := val["type"].(string); tp != "" {
		t.Fatalf("default split config type = %q, want empty", tp)
	}

	// Set to regex.
	rec = env.do(httptest.NewRequest(http.MethodPost, url, strings.NewReader(`{"type":"regex","regex":"^Chapter\\s+\\d+"}`)))
	assertStatus(t, rec, http.StatusNoContent)

	rec = env.do(httptest.NewRequest(http.MethodGet, url, nil))
	assertStatus(t, rec, http.StatusOK)
	got = decodeJSON[map[string]any](t, rec)
	val, _ = got["value"].(map[string]any)
	if tp, _ := val["type"].(string); tp != "regex" {
		t.Fatalf("split config type after set regex = %q, want regex", tp)
	}
	if re, _ := val["regex"].(string); re != `^Chapter\s+\d+` {
		t.Fatalf("split config regex = %q, want ^Chapter\\s+\\d+", re)
	}

	// Set to line_count.
	rec = env.do(httptest.NewRequest(http.MethodPost, url, strings.NewReader(`{"type":"line_count","line_count":50}`)))
	assertStatus(t, rec, http.StatusNoContent)

	rec = env.do(httptest.NewRequest(http.MethodGet, url, nil))
	assertStatus(t, rec, http.StatusOK)
	got = decodeJSON[map[string]any](t, rec)
	val, _ = got["value"].(map[string]any)
	if tp, _ := val["type"].(string); tp != "line_count" {
		t.Fatalf("split config type after set line_count = %q", tp)
	}
	if lc, _ := val["line_count"].(float64); lc != 50 {
		t.Fatalf("split config line_count = %v, want 50", lc)
	}

	// Setting type to empty string (none) is accepted.
	rec = env.do(httptest.NewRequest(http.MethodPost, url, strings.NewReader(`{"type":""}`)))
	assertStatus(t, rec, http.StatusNoContent)

	rec = env.do(httptest.NewRequest(http.MethodGet, url, nil))
	assertStatus(t, rec, http.StatusOK)
	got = decodeJSON[map[string]any](t, rec)
	val, _ = got["value"].(map[string]any)
	if tp, _ := val["type"].(string); tp != "" {
		t.Fatalf("split config type after set empty = %q, want empty", tp)
	}

	// Boundary type is rejected.
	rec = env.do(httptest.NewRequest(http.MethodPost, url, strings.NewReader(`{"type":"boundary","boundaries":[1,100]}`)))
	assertStatus(t, rec, http.StatusBadRequest)

	// Invalid regex is rejected.
	rec = env.do(httptest.NewRequest(http.MethodPost, url, strings.NewReader(`{"type":"regex","regex":"[invalid"}`)))
	assertStatus(t, rec, http.StatusBadRequest)

	// Non-positive line_count is rejected.
	rec = env.do(httptest.NewRequest(http.MethodPost, url, strings.NewReader(`{"type":"line_count","line_count":0}`)))
	assertStatus(t, rec, http.StatusBadRequest)

	// Delete resets to default.
	rec = env.do(httptest.NewRequest(http.MethodDelete, url, nil))
	assertStatus(t, rec, http.StatusNoContent)

	rec = env.do(httptest.NewRequest(http.MethodGet, url, nil))
	assertStatus(t, rec, http.StatusOK)
	got = decodeJSON[map[string]any](t, rec)
	val, _ = got["value"].(map[string]any)
	if tp, _ := val["type"].(string); tp != "" {
		t.Fatalf("split config type after delete = %q, want empty", tp)
	}
}
