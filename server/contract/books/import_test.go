package books_test

import (
	"net/http"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/voilelab/plainshelf/server/contract/apitest"

	"github.com/voilelab/plainshelf/internal/testutil"
	"github.com/voilelab/plainshelf/server"
	"golang.org/x/text/encoding/japanese"
)

func TestAPIImportBookContract(t *testing.T) {
	env := apitest.New(t)

	created := apitest.ImportTextBook(t, env, "Imported Book", " /inbox/txt/ ", "upload.txt", "hello world")
	if created.Meta == nil || created.Meta.ID == "" || created.Meta.Title != "Imported Book" {
		t.Fatalf("unexpected imported book meta: %#v", created.Meta)
	}
	if strings.Join(created.Folder, "/") != "inbox/txt" {
		t.Fatalf("folder = %#v, want inbox/txt", created.Folder)
	}
	if created.Meta.CurrentSource == "" {
		t.Fatal("import response missing current_source")
	}

	// A form with no file part at all, and a file whose extension this build does
	// not import, are both client errors.
	rec := apitest.PostBookImport(t, env, apitest.FormUpload{
		Fields:    [][2]string{{"title", "Missing File"}},
		FileField: "file",
	})
	apitest.AssertStatus(t, rec, http.StatusBadRequest)

	rec = apitest.PostBookImport(t, env, apitest.BookUpload("book.cbz", "text/plain", "not a supported upload"))
	apitest.AssertStatus(t, rec, http.StatusBadRequest)
}

// TestAPIImportUnsupportedEncodingContract pins that a .txt whose bytes decode to
// an encoding the server has no decoder for is a client error (400), not a server
// error (500), and that the detected encoding name reaches the client so the user
// can tell why their file was refused.
func TestAPIImportUnsupportedEncodingContract(t *testing.T) {
	env := apitest.New(t)

	shiftJIS, err := japanese.ShiftJIS.NewEncoder().String(
		"これは日本語のテキストです。文字化けのテスト。")
	if err != nil {
		t.Fatalf("failed to encode Shift-JIS fixture: %v", err)
	}

	rec := apitest.PostBookImport(t, env, apitest.BookUpload("novel.txt", apitest.PlainTextContentType, shiftJIS))
	apitest.AssertStatus(t, rec, http.StatusBadRequest)
	if body := rec.Body.String(); !strings.Contains(body, "SHIFT_JIS") {
		t.Fatalf("400 body = %q, want it to name the detected encoding SHIFT_JIS", body)
	}
}

// TestAPIImportStripsBOMContract pins the UTF-8-SIG behavior end to end: a leading
// BOM must not survive into the stored source content, and it must not be counted
// in char_count. apitest.ImportTextBook prepends a BOM to every upload, so this exercises
// the common path.
func TestAPIImportStripsBOMContract(t *testing.T) {
	env := apitest.New(t)

	// apitest.ImportTextBook uploads "\ufeff" + body + "\n世界"; after BOM stripping the
	// stored content is exactly that without the leading BOM.
	book := apitest.ImportTextBook(t, env, "BOM Book", "", "bom.txt", "hello world")
	const wantContent = "hello world\n世界"

	rec := env.Get(apitest.BookURL(book.Meta.ID, "content"))
	apitest.AssertStatus(t, rec, http.StatusOK)
	content := rec.Body.String()
	if strings.HasPrefix(content, "\ufeff") {
		t.Fatalf("stored content still starts with a BOM: %q", content)
	}
	if content != wantContent {
		t.Fatalf("content = %q, want %q", content, wantContent)
	}

	// char_count is derived from the stored content, so it must not count the BOM.
	books := apitest.GetJSON[[]server.Book](t, env, apitest.BooksURL()+"?include=char_count")
	var charCount int
	var found bool
	for _, b := range books {
		if b.Meta != nil && b.Meta.ID == book.Meta.ID {
			charCount = b.CharCount
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("imported book %s missing from listing", book.Meta.ID)
	}
	if want := utf8.RuneCountInString(wantContent); charCount != want {
		t.Fatalf("char_count = %d, want %d (BOM must not be counted)", charCount, want)
	}
}

func TestAPIImportMarkdownBookContract(t *testing.T) {
	env := apitest.New(t)

	// Browsers vary in what Content-Type they send for a .md upload; the
	// extension is the primary signal, so all of these must succeed.
	const markdown = "# Notes\n\nhello markdown"
	var markdownBook server.Book
	for _, tc := range []struct{ name, filename, contentType string }{
		{"markdown content type", "notes.md", "text/markdown; charset=utf-8"},
		{"text/plain content type", "plain-notes.md", apitest.PlainTextContentType},
		{"no content type", "no-content-type.md", ""},
		{"legacy x-markdown content type", "legacy-notes.md", "text/x-markdown; charset=utf-8"},
	} {
		imported := apitest.ImportFileBook(t, env, tc.filename, tc.contentType, markdown)
		if imported.Meta == nil || imported.Meta.Format != "md" {
			t.Fatalf("%s: unexpected imported book meta: %#v", tc.name, imported.Meta)
		}
		if markdownBook.Meta == nil {
			markdownBook = imported
		}
	}

	// A plain .txt import must still be recognized as "txt" format.
	txtBook := apitest.ImportTextBook(t, env, "Plain Text", "", "plain.txt", "hello world")
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
		sourceMeta := apitest.GetJSON[map[string]any](t, env,
			apitest.SourceURL(imported.book.Meta.ID, imported.book.Meta.CurrentSource))
		if sourceMeta["format"] != imported.want || sourceMeta["schema_version"] != float64(1) {
			t.Fatalf("source meta = %#v, want schema 1 format %s", sourceMeta, imported.want)
		}
	}

	// Reading back the content must still be exactly what was uploaded, with no
	// markdown rendering applied (rendering is out of scope for this PR).
	contentRec := env.Get(apitest.BookURL(markdownBook.Meta.ID, "content"))
	apitest.AssertStatus(t, contentRec, http.StatusOK)
	apitest.AssertContentType(t, contentRec, apitest.PlainTextContentType)
	if got := contentRec.Body.String(); got != markdown {
		t.Fatalf("content = %q, want raw markdown source", got)
	}
}

func TestAPIImportEPUBBookContract(t *testing.T) {
	env := apitest.New(t)
	archive := string(testutil.BuildTestEPUB(t))

	imported := apitest.ImportFileBook(t, env, "three-body.epub", "application/epub+zip", archive)
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

	rec := env.Get(apitest.BookURL(imported.Meta.ID, "content"))
	apitest.AssertStatus(t, rec, http.StatusOK)
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
	rec = env.Get(apitest.SourceURL(imported.Meta.ID, imported.Meta.CurrentSource))
	apitest.AssertStatus(t, rec, http.StatusOK)
	apitest.AssertJSONContentType(t, rec)
	assertSourceFormat(t, apitest.DecodeJSON[map[string]any](t, rec), "md")

	// The cover survives the round trip.
	rec = env.Get(apitest.BookURL(imported.Meta.ID, "cover"))
	apitest.AssertStatus(t, rec, http.StatusOK)
	if rec.Body.Len() == 0 {
		t.Fatal("cover endpoint returned an empty body")
	}
}

func TestAPIImportEPUBBookStrategyContract(t *testing.T) {
	env := apitest.New(t)
	archive := string(testutil.BuildTestEPUB(t))

	// A per-import strategy overrides the configured default.
	plain := apitest.ImportEPUBWithStrategy(t, env, "plain.epub", archive,
		`{"preset":"plain","include_description":false}`)
	if plain.Meta.Format != "txt" {
		t.Fatalf("format = %q, want txt for the plain preset", plain.Meta.Format)
	}

	rec := env.Get(apitest.BookURL(plain.Meta.ID, "content"))
	apitest.AssertStatus(t, rec, http.StatusOK)
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
	plainSourceMeta := apitest.GetJSON[map[string]any](t, env, apitest.SourceURL(plain.Meta.ID, plain.Meta.CurrentSource))
	assertSourceFormat(t, plainSourceMeta, "txt")

	// An unknown preset is rejected rather than silently falling back, and a file
	// that is not a readable archive is rejected rather than stored as a broken
	// book.
	apitest.ImportEPUBExpectingStatus(t, env, "bad.epub", archive, `{"preset":"custom"}`, http.StatusBadRequest)
	apitest.ImportEPUBExpectingStatus(t, env, "broken.epub", "this is not a zip", "", http.StatusBadRequest)
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
	if _, ok := sourceMeta["split_config"]; ok {
		t.Fatalf("imported source wrote split_config = %#v, want it absent", sourceMeta["split_config"])
	}
}
