package contract_test

import (
	"net/http"
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

	// A form with no file part at all, and a file whose extension this build does
	// not import, are both client errors.
	rec := postBookImport(t, env, formUpload{
		fields:    [][2]string{{"title", "Missing File"}},
		fileField: "file",
	})
	assertStatus(t, rec, http.StatusBadRequest)

	rec = postBookImport(t, env, bookUpload("book.cbz", "text/plain", "not a supported upload"))
	assertStatus(t, rec, http.StatusBadRequest)
}

func TestAPIImportMarkdownBookContract(t *testing.T) {
	env := newAPITestEnv(t)

	// Browsers vary in what Content-Type they send for a .md upload; the
	// extension is the primary signal, so all of these must succeed.
	const markdown = "# Notes\n\nhello markdown"
	var markdownBook server.Book
	for _, tc := range []struct{ name, filename, contentType string }{
		{"markdown content type", "notes.md", "text/markdown; charset=utf-8"},
		{"text/plain content type", "plain-notes.md", plainTextContentType},
		{"no content type", "no-content-type.md", ""},
		{"legacy x-markdown content type", "legacy-notes.md", "text/x-markdown; charset=utf-8"},
	} {
		imported := importFileBook(t, env, tc.filename, tc.contentType, markdown)
		if imported.Meta == nil || imported.Meta.Format != "md" {
			t.Fatalf("%s: unexpected imported book meta: %#v", tc.name, imported.Meta)
		}
		if markdownBook.Meta == nil {
			markdownBook = imported
		}
	}

	// A plain .txt import must still be recognized as "txt" format.
	txtBook := importTextBook(t, env, "Plain Text", "", "plain.txt", "hello world")
	if txtBook.Meta == nil || txtBook.Meta.Format != "txt" {
		t.Fatalf("unexpected imported book meta: %#v", txtBook.Meta)
	}

	// The source carries the same format as the book it was created for.
	for _, imported := range []struct {
		book server.Book
		want string
	}{
		{markdownBook, "md"},
		{txtBook, "txt"},
	} {
		sourceMeta := getJSON[map[string]any](t, env,
			sourceURL(imported.book.Meta.ID, imported.book.Meta.CurrentSource))
		if sourceMeta["format"] != imported.want || sourceMeta["schema_version"] != float64(1) {
			t.Fatalf("source meta = %#v, want schema 1 format %s", sourceMeta, imported.want)
		}
	}

	// Reading back the content must still be exactly what was uploaded, with no
	// markdown rendering applied (rendering is out of scope for this PR).
	contentRec := env.get(bookURL(markdownBook.Meta.ID, "content"))
	assertStatus(t, contentRec, http.StatusOK)
	assertContentType(t, contentRec, plainTextContentType)
	if got := contentRec.Body.String(); got != markdown {
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

	rec := env.get(bookURL(imported.Meta.ID, "content"))
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
	rec = env.get(sourceURL(imported.Meta.ID, imported.Meta.CurrentSource))
	assertStatus(t, rec, http.StatusOK)
	assertJSONContentType(t, rec)
	assertSourceFormat(t, decodeJSON[map[string]any](t, rec), "md")

	// The cover survives the round trip.
	rec = env.get(bookURL(imported.Meta.ID, "cover"))
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

	rec := env.get(bookURL(plain.Meta.ID, "content"))
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
	plainSourceMeta := getJSON[map[string]any](t, env, sourceURL(plain.Meta.ID, plain.Meta.CurrentSource))
	assertSourceFormat(t, plainSourceMeta, "txt")

	// An unknown preset is rejected rather than silently falling back, and a file
	// that is not a readable archive is rejected rather than stored as a broken
	// book.
	importEPUBExpectingStatus(t, env, "bad.epub", archive, `{"preset":"custom"}`, http.StatusBadRequest)
	importEPUBExpectingStatus(t, env, "broken.epub", "this is not a zip", "", http.StatusBadRequest)
}

// assertSourceFormat pins the two fields an imported source's meta must report:
// the format the reader parses it with, and the absence of a split config, since
// chapter structure now comes from the text itself.
func assertSourceFormat(t *testing.T, sourceMeta map[string]any, wantFormat string) {
	t.Helper()

	if format, _ := sourceMeta["format"].(string); format != wantFormat {
		t.Fatalf("source format = %q, want %s", format, wantFormat)
	}
	if version, _ := sourceMeta["schema_version"].(float64); version != 1 {
		t.Fatalf("source schema_version = %v, want 1", sourceMeta["schema_version"])
	}
	if split, _ := sourceMeta["split_config"].(map[string]any); split["type"] != "" {
		t.Fatalf("source split config = %#v, want none", split)
	}
}
