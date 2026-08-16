package contract_test

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"strings"
	"testing"

	"github.com/voilelab/plainshelf/internal/testutil"
	"github.com/voilelab/plainshelf/server"
)

func TestAPIImportBookContract(t *testing.T) {
	env := newAPITestEnv(t)

	created := importTextBook(t, env, "Imported Book", " /inbox/txt/ ", "upload.txt", "hello world")
	if created.Meta == nil || created.Meta.ID == "" || created.Meta.Title != "Imported Book" {
		t.Fatalf("unexpected imported book meta: %#v", created.Meta)
	}
	if strings.Join(created.Layer, "/") != "inbox/txt" {
		t.Fatalf("layer = %#v, want inbox/txt", created.Layer)
	}
	if created.Meta.CurrentSource == "" {
		t.Fatal("import response missing current_source")
	}

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	if err := writer.WriteField("title", "Missing File"); err != nil {
		t.Fatalf("WriteField: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close multipart writer: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/shelves/default_shelf/books/import", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := env.do(req)
	assertStatus(t, rec, http.StatusBadRequest)

	buf.Reset()
	writer = multipart.NewWriter(&buf)
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", `form-data; name="file"; filename="book.cbz"`)
	h.Set("Content-Type", "text/plain")
	part, err := writer.CreatePart(h)
	if err != nil {
		t.Fatalf("CreatePart: %v", err)
	}
	if _, err := part.Write([]byte("not a supported upload")); err != nil {
		t.Fatalf("write bad file: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close multipart writer: %v", err)
	}
	req = httptest.NewRequest(http.MethodPost, "/api/shelves/default_shelf/books/import", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec = env.do(req)
	assertStatus(t, rec, http.StatusBadRequest)
}

func TestAPIImportMarkdownBookContract(t *testing.T) {
	env := newAPITestEnv(t)

	// Browsers vary in what Content-Type they send for a .md upload; the
	// extension is the primary signal, so all of these must succeed.
	withMarkdownContentType := importFileBook(t, env, "notes.md", "text/markdown; charset=utf-8", "# Notes\n\nhello markdown")
	if withMarkdownContentType.Meta == nil || withMarkdownContentType.Meta.Format != "md" {
		t.Fatalf("unexpected imported book meta: %#v", withMarkdownContentType.Meta)
	}

	withTextPlainContentType := importFileBook(t, env, "plain-notes.md", "text/plain; charset=utf-8", "# Notes\n\nhello markdown")
	if withTextPlainContentType.Meta == nil || withTextPlainContentType.Meta.Format != "md" {
		t.Fatalf("unexpected imported book meta: %#v", withTextPlainContentType.Meta)
	}

	withNoContentType := importFileBook(t, env, "no-content-type.md", "", "# Notes\n\nhello markdown")
	if withNoContentType.Meta == nil || withNoContentType.Meta.Format != "md" {
		t.Fatalf("unexpected imported book meta: %#v", withNoContentType.Meta)
	}

	withXMarkdownContentType := importFileBook(t, env, "legacy-notes.md", "text/x-markdown; charset=utf-8", "# Notes\n\nhello markdown")
	if withXMarkdownContentType.Meta == nil || withXMarkdownContentType.Meta.Format != "md" {
		t.Fatalf("unexpected imported book meta: %#v", withXMarkdownContentType.Meta)
	}

	// A plain .txt import must still be recognized as "txt" format.
	txtBook := importTextBook(t, env, "Plain Text", "", "plain.txt", "hello world")
	if txtBook.Meta == nil || txtBook.Meta.Format != "txt" {
		t.Fatalf("unexpected imported book meta: %#v", txtBook.Meta)
	}

	for _, imported := range []struct {
		book server.Book
		want string
	}{
		{withMarkdownContentType, "md"},
		{txtBook, "txt"},
	} {
		sourceURL := "/api/shelves/default_shelf/books/" + imported.book.Meta.ID + "/sources/" + imported.book.Meta.CurrentSource
		sourceRec := env.do(httptest.NewRequest(http.MethodGet, sourceURL, nil))
		assertStatus(t, sourceRec, http.StatusOK)
		sourceMeta := decodeJSON[map[string]any](t, sourceRec)
		if sourceMeta["format"] != imported.want || sourceMeta["schema_version"] != float64(1) {
			t.Fatalf("source meta = %#v, want schema 1 format %s", sourceMeta, imported.want)
		}
	}

	// Reading back the content must still be exactly what was uploaded, with no
	// markdown rendering applied (rendering is out of scope for this PR).
	contentReq := httptest.NewRequest(http.MethodGet, "/api/shelves/default_shelf/books/"+withMarkdownContentType.Meta.ID+"/content", nil)
	contentRec := env.do(contentReq)
	assertStatus(t, contentRec, http.StatusOK)
	if got := contentRec.Header().Get("Content-Type"); got != "text/plain; charset=utf-8" {
		t.Fatalf("Content-Type = %q, want text/plain; charset=utf-8", got)
	}
	if got := contentRec.Body.String(); got != "# Notes\n\nhello markdown" {
		t.Fatalf("content = %q, want raw markdown source", got)
	}
}

func TestAPIImportEPUBBookContract(t *testing.T) {
	env := newAPITestEnv(t)
	archive := string(testutil.BuildTestEPUB(t))

	imported := importFileBook(t, env, "three-body.epub", "application/epub+zip", archive)
	if imported.Meta == nil {
		t.Fatal("import response missing meta")
	}

	// The book's own dc:title beats the filename.
	if imported.Meta.Title != testutil.TestEPUBTitle {
		t.Fatalf("title = %q, want %q", imported.Meta.Title, testutil.TestEPUBTitle)
	}
	// The default strategy is the Markdown preset, so the stored format is "md".
	if imported.Meta.Format != "md" {
		t.Fatalf("format = %q, want md", imported.Meta.Format)
	}
	if len(imported.Meta.Authors) != 1 || imported.Meta.Authors[0] != "林望舒" {
		t.Fatalf("authors = %#v, want [林望舒]", imported.Meta.Authors)
	}
	if imported.Meta.Language != "zh-Hant" {
		t.Fatalf("language = %q, want zh-Hant", imported.Meta.Language)
	}
	// dc:description lands in the metadata regardless of whether it is also
	// written into the text.
	if imported.Meta.Comments != "一部關於旅途的短篇小說。" {
		t.Fatalf("comments = %q, want the epub description", imported.Meta.Comments)
	}
	if got := imported.Meta.Identifiers["isbn"]; got != "urn:isbn:9781234567897" {
		t.Fatalf("identifiers[isbn] = %q", got)
	}
	if imported.Meta.Cover == "" {
		t.Fatal("imported book has no cover")
	}
	if imported.Meta.CurrentSource == "" {
		t.Fatal("imported book has no current source")
	}

	base := "/api/shelves/default_shelf/books/" + imported.Meta.ID

	rec := env.do(httptest.NewRequest(http.MethodGet, base+"/content", nil))
	assertStatus(t, rec, http.StatusOK)
	content := rec.Body.String()
	for _, want := range []string{
		"# " + testutil.TestEPUBTitle,
		"一部關於旅途的短篇小說。",
		"## 啟程",
		"他走出了車站。",
		"## 歸途",
		"回程的路上。",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("content is missing %q:\n%s", want, content)
		}
	}
	// The navigation document is not a chapter, and the headings consumed as
	// chapter titles must not be duplicated in the body.
	if strings.Contains(content, "第一章") || strings.Contains(content, "第二章") {
		t.Fatalf("content still contains in-document headings:\n%s", content)
	}

	// The source owns the Markdown format. H2 text is the chapter structure, so
	// EPUB import no longer persists a parallel regex or boundary configuration.
	rec = env.do(httptest.NewRequest(http.MethodGet, base+"/sources/"+imported.Meta.CurrentSource, nil))
	assertStatus(t, rec, http.StatusOK)
	assertJSONContentType(t, rec)
	sourceMeta := decodeJSON[map[string]any](t, rec)
	if format, _ := sourceMeta["format"].(string); format != "md" {
		t.Fatalf("source format = %q, want md", format)
	}
	if version, _ := sourceMeta["schema_version"].(float64); version != 1 {
		t.Fatalf("source schema_version = %v, want 1", sourceMeta["schema_version"])
	}
	if split, _ := sourceMeta["split_config"].(map[string]any); split["type"] != "" {
		t.Fatalf("source split config = %#v, want none", split)
	}

	// The cover survives the round trip.
	rec = env.do(httptest.NewRequest(http.MethodGet, base+"/cover", nil))
	assertStatus(t, rec, http.StatusOK)
	if rec.Body.Len() == 0 {
		t.Fatal("cover endpoint returned an empty body")
	}
}

func TestAPIImportEPUBBookStrategyContract(t *testing.T) {
	env := newAPITestEnv(t)
	archive := string(testutil.BuildTestEPUB(t))

	// A per-import strategy overrides the configured default.
	plain := importEPUBWithStrategy(t, env, "plain.epub", archive,
		`{"preset":"plain","include_description":false}`)
	if plain.Meta.Format != "txt" {
		t.Fatalf("format = %q, want txt for the plain preset", plain.Meta.Format)
	}

	rec := env.do(httptest.NewRequest(http.MethodGet,
		"/api/shelves/default_shelf/books/"+plain.Meta.ID+"/content", nil))
	assertStatus(t, rec, http.StatusOK)
	content := rec.Body.String()
	if strings.Contains(content, "#") {
		t.Fatalf("plain preset emitted markdown markers:\n%s", content)
	}
	if strings.Contains(content, "一部關於旅途的短篇小說。") {
		t.Fatalf("include_description=false still wrote the description into the text:\n%s", content)
	}
	if !strings.Contains(content, "啟程") {
		t.Fatalf("plain preset lost the chapter titles:\n%s", content)
	}

	// Plain EPUB output is an unstructured TXT source; it deliberately carries
	// no chapter navigation state.
	rec = env.do(httptest.NewRequest(http.MethodGet,
		"/api/shelves/default_shelf/books/"+plain.Meta.ID+"/sources/"+plain.Meta.CurrentSource, nil))
	assertStatus(t, rec, http.StatusOK)
	plainSourceMeta := decodeJSON[map[string]any](t, rec)
	if format, _ := plainSourceMeta["format"].(string); format != "txt" {
		t.Fatalf("source format = %q, want txt", format)
	}
	if split, _ := plainSourceMeta["split_config"].(map[string]any); split["type"] != "" {
		t.Fatalf("source split config = %#v, want none", split)
	}

	// An unknown preset is rejected rather than silently falling back.
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	if err := writer.WriteField("strategy", `{"preset":"custom"}`); err != nil {
		t.Fatalf("WriteField strategy: %v", err)
	}
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", `form-data; name="file"; filename="bad.epub"`)
	h.Set("Content-Type", "application/epub+zip")
	part, err := writer.CreatePart(h)
	if err != nil {
		t.Fatalf("CreatePart: %v", err)
	}
	if _, err := io.Copy(part, strings.NewReader(archive)); err != nil {
		t.Fatalf("write multipart file: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close multipart writer: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/shelves/default_shelf/books/import", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	assertStatus(t, env.do(req), http.StatusBadRequest)

	// A file that is not a readable archive is rejected, not stored as a broken
	// book.
	notAnArchive := importEPUBExpectingStatus(t, env, "broken.epub", "this is not a zip", "", http.StatusBadRequest)
	_ = notAnArchive
}
