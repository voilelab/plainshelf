package contract_test

import (
	"archive/zip"
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// testEPUBTitle is the dc:title inside the archive buildTestEPUB produces.
const testEPUBTitle = "星塵集"

// onePixelPNG is a real 1x1 PNG so the cover survives the JPEG conversion the
// import performs when cover_to_jpg is enabled.
func onePixelPNG() []byte {
	return []byte{
		0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A,
		0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52,
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53,
		0xDE, 0x00, 0x00, 0x00, 0x0C, 0x49, 0x44, 0x41,
		0x54, 0x08, 0xD7, 0x63, 0xF8, 0xCF, 0xC0, 0x00,
		0x00, 0x03, 0x01, 0x01, 0x00, 0x18, 0xDD, 0x8D,
		0xB0, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4E,
		0x44, 0xAE, 0x42, 0x60, 0x82,
	}
}

func zipEPUBEntries(t *testing.T, entries []struct{ name, body string }) []byte {
	t.Helper()

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, e := range entries {
		w, err := zw.Create(e.name)
		if err != nil {
			t.Fatalf("create zip entry %q: %v", e.name, err)
		}
		if _, err := w.Write([]byte(e.body)); err != nil {
			t.Fatalf("write zip entry %q: %v", e.name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}

	return buf.Bytes()
}

// buildTestEPUB returns a small but complete EPUB 3 archive: two chapters, a
// navigation document, a cover, and enough metadata to exercise every field the
// importer copies onto the book.
func buildTestEPUB(t *testing.T) []byte {
	t.Helper()

	entries := []struct{ name, body string }{
		{"mimetype", "application/epub+zip"},
		{"META-INF/container.xml", `<?xml version="1.0"?>
<container xmlns="urn:oasis:names:tc:opendocument:xmlns:container" version="1.0">
  <rootfiles><rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/></rootfiles>
</container>`},
		{"OEBPS/content.opf", `<?xml version="1.0"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="pub-id">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:title>` + testEPUBTitle + `</dc:title>
    <dc:creator>林望舒</dc:creator>
    <dc:language>zh-Hant</dc:language>
    <dc:description>一部關於旅途的短篇小說。</dc:description>
    <dc:date>2024-03-01</dc:date>
    <dc:identifier id="pub-id">urn:isbn:9781234567897</dc:identifier>
  </metadata>
  <manifest>
    <item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/>
    <item id="cover-img" href="cover.png" media-type="image/png" properties="cover-image"/>
    <item id="c1" href="ch1.xhtml" media-type="application/xhtml+xml"/>
    <item id="c2" href="ch2.xhtml" media-type="application/xhtml+xml"/>
  </manifest>
  <spine><itemref idref="nav"/><itemref idref="c1"/><itemref idref="c2"/></spine>
</package>`},
		{"OEBPS/nav.xhtml", `<html xmlns:epub="http://www.idpf.org/2007/ops"><body>
<nav epub:type="toc"><ol>
  <li><a href="ch1.xhtml">啟程</a></li>
  <li><a href="ch2.xhtml">歸途</a></li>
</ol></nav></body></html>`},
		{"OEBPS/cover.png", string(onePixelPNG())},
		{"OEBPS/ch1.xhtml", `<html><body><h1>第一章</h1><p>他走出了車站。</p></body></html>`},
		{"OEBPS/ch2.xhtml", `<html><body><h1>第二章</h1><p>回程的路上。</p></body></html>`},
	}

	return zipEPUBEntries(t, entries)
}

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
