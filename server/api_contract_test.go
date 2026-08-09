package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/voilelab/plainshelf/internal/logutil"
	"github.com/voilelab/plainshelf/server/store"
	"github.com/voilelab/plainshelf/shelf"
)

type apiTestEnv struct {
	app     *App
	handler http.Handler
	// libRoot is the shelf's on-disk root, so tests can inspect or tamper with
	// the files behind the API.
	libRoot string
}

type wailsLikeRecorder struct {
	header      http.Header
	body        bytes.Buffer
	code        int
	wroteHeader bool
}

func newWailsLikeRecorder() *wailsLikeRecorder {
	return &wailsLikeRecorder{
		header: http.Header{},
		code:   http.StatusNotImplemented,
	}
}

func (rec *wailsLikeRecorder) Header() http.Header {
	return rec.header
}

func (rec *wailsLikeRecorder) Write(buf []byte) (int, error) {
	if !rec.wroteHeader {
		rec.WriteHeader(http.StatusOK)
	}
	return rec.body.Write(buf)
}

func (rec *wailsLikeRecorder) WriteHeader(code int) {
	if rec.wroteHeader {
		return
	}
	rec.wroteHeader = true
	rec.code = code
}

func newAPITestEnv(t *testing.T) *apiTestEnv {
	t.Helper()

	libRoot := t.TempDir()

	app, err := NewApp(&AppConf{
		Shelves: []*shelf.ShelfConfWithID{
			{
				ID: "default_shelf",
				ShelfConf: shelf.ShelfConf{
					LibRoot: libRoot,
				},
			},
		},
		StorePath:  t.TempDir(),
		CoverToJPG: false,
	})
	if err != nil {
		t.Fatalf("NewApp: %v", err)
	}
	t.Cleanup(func() {
		if err := app.Close(); err != nil {
			t.Fatalf("Close app: %v", err)
		}
	})

	// Start the background worker so endpoints backed by task chains behave as
	// they do in production.
	if err := app.Start(); err != nil {
		t.Fatalf("Start app: %v", err)
	}

	return &apiTestEnv{app: app, handler: app.Handler(), libRoot: libRoot}
}

// bookMetaPath locates the on-disk book.json for the given book ID by walking
// the shelf root, so tests do not have to reproduce the folder-naming scheme.
func (env *apiTestEnv) bookMetaPath(t *testing.T, bookID string) string {
	t.Helper()

	var found string
	err := filepath.WalkDir(env.libRoot, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || d.Name() != shelf.BookMetaFile || found != "" {
			return err
		}
		raw, readErr := os.ReadFile(p)
		if readErr != nil {
			return readErr
		}
		var meta shelf.BookMeta
		if json.Unmarshal(raw, &meta) == nil && meta.ID == bookID {
			found = p
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk lib root: %v", err)
	}
	if found == "" {
		t.Fatalf("could not find book.json for book %s under %s", bookID, env.libRoot)
	}
	return found
}

func (env *apiTestEnv) do(req *http.Request) *httptest.ResponseRecorder {
	if isMutatingMethod(req.Method) && req.Header.Get(env.app.SecurityTokenHeader()) == "" && req.Header.Get("Authorization") == "" {
		req.Header.Set(env.app.SecurityTokenHeader(), env.app.SecurityToken())
	}
	return env.doRaw(req)
}

func (env *apiTestEnv) doRaw(req *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	env.handler.ServeHTTP(rec, req)
	return rec
}

func assertStatus(t *testing.T, rec *httptest.ResponseRecorder, want int) {
	t.Helper()
	if rec.Code != want {
		t.Fatalf("status = %d, want %d, body: %s", rec.Code, want, rec.Body.String())
	}
}

func assertJSONContentType(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	if got := rec.Header().Get("Content-Type"); got != "application/json; charset=utf-8" {
		t.Fatalf("Content-Type = %q, want application/json; charset=utf-8", got)
	}
}

func assertWailsLikeStatus(t *testing.T, rec *wailsLikeRecorder, want int) {
	t.Helper()
	if rec.code != want {
		t.Fatalf("status = %d, want %d, body: %s", rec.code, want, rec.body.String())
	}
}

func decodeJSON[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var out T
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode JSON %q: %v", rec.Body.String(), err)
	}
	return out
}

func importTextBook(t *testing.T, env *apiTestEnv, title, layer, filename, body string) Book {
	t.Helper()

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	if title != "" {
		if err := writer.WriteField("title", title); err != nil {
			t.Fatalf("WriteField title: %v", err)
		}
	}
	if layer != "" {
		if err := writer.WriteField("layer", layer); err != nil {
			t.Fatalf("WriteField layer: %v", err)
		}
	}

	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="file"; filename="%s"`, filename))
	h.Set("Content-Type", "text/plain; charset=utf-8")
	part, err := writer.CreatePart(h)
	if err != nil {
		t.Fatalf("CreatePart: %v", err)
	}
	if _, err := io.Copy(part, strings.NewReader("\ufeff"+body+"\n世界")); err != nil {
		t.Fatalf("write multipart file: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close multipart writer: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/shelves/default_shelf/books/import", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := env.do(req)
	assertStatus(t, rec, http.StatusCreated)
	assertJSONContentType(t, rec)
	return decodeJSON[Book](t, rec)
}

func TestAPIGetBooksContract(t *testing.T) {
	env := newAPITestEnv(t)

	rec := env.do(httptest.NewRequest(http.MethodGet, "/api/shelves/default_shelf/books", nil))
	assertStatus(t, rec, http.StatusOK)
	assertJSONContentType(t, rec)
	if got := decodeJSON[[]Book](t, rec); len(got) != 0 {
		t.Fatalf("empty library returned %d books", len(got))
	}

	alpha := importTextBook(t, env, "Alpha Tale", "/fiction/adventure", "alpha.txt", "alpha body")
	_ = importTextBook(t, env, "Beta Notes", "/notes", "beta.txt", "beta body")

	patchBody := `{"authors":["Ada"],"tags":["contract","api"],"language":"en","comment":"needle comment"}`
	rec = env.do(httptest.NewRequest(http.MethodPatch, "/api/shelves/default_shelf/books/"+alpha.Meta.ID, strings.NewReader(patchBody)))
	assertStatus(t, rec, http.StatusOK)

	rec = env.do(httptest.NewRequest(http.MethodGet, "/api/shelves/default_shelf/books", nil))
	assertStatus(t, rec, http.StatusOK)
	books := decodeJSON[[]Book](t, rec)
	if len(books) != 2 {
		t.Fatalf("list returned %d books, want 2", len(books))
	}
	var got *Book
	for i := range books {
		if books[i].Meta != nil && books[i].Meta.ID == alpha.Meta.ID {
			got = &books[i]
			break
		}
	}
	if got == nil || got.Meta.Title != "Alpha Tale" {
		t.Fatalf("unexpected book meta: %#v", got)
	}
	if got.Meta.Comments != "needle comment" || got.Meta.Language != "en" {
		t.Fatalf("metadata fields not preserved in list response: %#v", got.Meta)
	}
	if len(got.Meta.Authors) != 1 || got.Meta.Authors[0] != "Ada" {
		t.Fatalf("authors = %#v, want Ada", got.Meta.Authors)
	}
	if len(got.Meta.Tags) != 2 || got.Meta.Tags[0] != "contract" || got.Meta.Tags[1] != "api" {
		t.Fatalf("tags = %#v, want contract/api", got.Meta.Tags)
	}
	if strings.Join(got.Layer, "/") != "fiction/adventure" {
		t.Fatalf("layer = %#v, want fiction/adventure", got.Layer)
	}
}

func TestAPIGetBooksCharCountContract(t *testing.T) {
	env := newAPITestEnv(t)
	_ = importTextBook(t, env, "Char Count Me", "", "charcount.txt", "alpha body")

	// Without include=char_count, the field must not appear in the response at all.
	rec := env.do(httptest.NewRequest(http.MethodGet, "/api/shelves/default_shelf/books", nil))
	assertStatus(t, rec, http.StatusOK)
	if strings.Contains(rec.Body.String(), "char_count") {
		t.Fatalf("response without include=char_count must not contain char_count field: %s", rec.Body.String())
	}

	// With include=char_count, every book carries a positive char_count.
	rec = env.do(httptest.NewRequest(http.MethodGet, "/api/shelves/default_shelf/books?include=char_count", nil))
	assertStatus(t, rec, http.StatusOK)
	books := decodeJSON[[]Book](t, rec)
	if len(books) != 1 {
		t.Fatalf("list returned %d books, want 1", len(books))
	}
	if books[0].CharCount <= 0 {
		t.Fatalf("char_count = %d, want > 0", books[0].CharCount)
	}
}

func TestAPIGetLogsContract(t *testing.T) {
	appLogDir := t.TempDir()
	appLogFile := filepath.Join(appLogDir, "app.log")
	shelfLogDir := t.TempDir()
	app, err := NewApp(&AppConf{
		Logger: logutil.LogConf{
			LogFile: logutil.LogFileConf{
				Type:     logutil.LogFileTypeName,
				Filename: appLogFile,
			},
		},
		Shelves: []*shelf.ShelfConfWithID{
			{
				ID: "default_shelf",
				ShelfConf: shelf.ShelfConf{
					Logger: logutil.LogConf{
						LogFile: logutil.LogFileConf{
							Type:   logutil.LogFileTypeNameRotate,
							Dir:    shelfLogDir,
							Prefix: "shelf",
						},
					},
					LibRoot: t.TempDir(),
				},
			},
		},
		StorePath:  t.TempDir(),
		CoverToJPG: false,
	})
	if err != nil {
		t.Fatalf("NewApp: %v", err)
	}
	t.Cleanup(func() {
		if err := app.Close(); err != nil {
			t.Fatalf("Close app: %v", err)
		}
	})

	handler := app.Handler()

	if err := os.WriteFile(appLogFile, []byte("app log"), 0o644); err != nil {
		t.Fatalf("WriteFile app log: %v", err)
	}
	appLogTime := time.Date(2024, 1, 3, 12, 0, 0, 0, time.UTC)
	if err := os.Chtimes(appLogFile, appLogTime, appLogTime); err != nil {
		t.Fatalf("Chtimes app log: %v", err)
	}
	if err := os.WriteFile(filepath.Join(shelfLogDir, "shelf-2024-01-02.log"), []byte("shelf log"), 0o644); err != nil {
		t.Fatalf("WriteFile shelf log: %v", err)
	}
	if err := os.WriteFile(filepath.Join(shelfLogDir, "ignore.txt"), []byte("nope"), 0o644); err != nil {
		t.Fatalf("WriteFile ignore file: %v", err)
	}

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/logs", nil))
	assertStatus(t, rec, http.StatusOK)
	assertJSONContentType(t, rec)

	logs := decodeJSON[[]LogFileEntry](t, rec)
	if len(logs) != 2 {
		t.Fatalf("log count = %d, want 2", len(logs))
	}

	if logs[0].ID == "" || logs[0].Source != "logger" || logs[0].Filename != "app.log" || logs[0].Date == "" {
		t.Fatalf("first log = %#v, want app logger entry", logs[0])
	}
	if logs[1].ID == "" || logs[1].Source != "shelves[0].shelfconf.logger" || logs[1].Filename != "shelf-2024-01-02.log" || logs[1].Date != "2024-01-02" {
		t.Fatalf("second log = %#v, want shelf logger entry", logs[1])
	}
}

func TestAPIGetLogContentContract(t *testing.T) {
	logDir := t.TempDir()
	app, err := NewApp(&AppConf{
		Logger: logutil.LogConf{
			LogFile: logutil.LogFileConf{
				Type:   logutil.LogFileTypeNameRotate,
				Dir:    logDir,
				Prefix: "app",
			},
		},
		Shelves: []*shelf.ShelfConfWithID{
			{
				ID: "default_shelf",
				ShelfConf: shelf.ShelfConf{
					LibRoot: t.TempDir(),
				},
			},
		},
		StorePath:  t.TempDir(),
		CoverToJPG: false,
	})
	if err != nil {
		t.Fatalf("NewApp: %v", err)
	}
	t.Cleanup(func() {
		if err := app.Close(); err != nil {
			t.Fatalf("Close app: %v", err)
		}
	})

	if err := os.WriteFile(filepath.Join(logDir, "app-2024-01-02.log"), []byte("line 1\nline 2\n"), 0o644); err != nil {
		t.Fatalf("WriteFile log: %v", err)
	}

	handler := app.Handler()
	listRec := httptest.NewRecorder()
	handler.ServeHTTP(listRec, httptest.NewRequest(http.MethodGet, "/api/logs", nil))
	assertStatus(t, listRec, http.StatusOK)

	logs := decodeJSON[[]LogFileEntry](t, listRec)
	var target *LogFileEntry
	for i := range logs {
		if logs[i].Filename == "app-2024-01-02.log" {
			target = &logs[i]
			break
		}
	}
	if target == nil {
		t.Fatalf("log list missing seeded file: %#v", logs)
	}

	contentRec := httptest.NewRecorder()
	handler.ServeHTTP(contentRec, httptest.NewRequest(http.MethodGet, "/api/logs/"+target.ID+"/content", nil))
	assertStatus(t, contentRec, http.StatusOK)
	if got := contentRec.Header().Get("Content-Type"); got != "text/plain; charset=utf-8" {
		t.Fatalf("Content-Type = %q, want text/plain; charset=utf-8", got)
	}
	if got := contentRec.Body.String(); got != "line 1\nline 2\n" {
		t.Fatalf("content = %q, want seeded log content", got)
	}

	missingRec := httptest.NewRecorder()
	handler.ServeHTTP(missingRec, httptest.NewRequest(http.MethodGet, "/api/logs/missing/content", nil))
	assertStatus(t, missingRec, http.StatusNotFound)
}

func TestAPIStreamContentReturns200ForEmptyFilesInWails(t *testing.T) {
	env := newAPITestEnv(t)
	created := importTextBook(t, env, "Empty Stream Book", "", "empty.txt", "content")
	bookID := created.Meta.ID
	sourceID := created.Meta.CurrentSource

	updateReq := httptest.NewRequest(http.MethodPatch, "/api/shelves/default_shelf/books/"+bookID+"/sources/"+sourceID+"/content", strings.NewReader(""))
	updateReq.Header.Set("Content-Type", "text/plain; charset=utf-8")
	updateRec := env.do(updateReq)
	assertStatus(t, updateRec, http.StatusNoContent)

	bookContentRec := newWailsLikeRecorder()
	env.handler.ServeHTTP(bookContentRec, httptest.NewRequest(http.MethodGet, "/api/shelves/default_shelf/books/"+bookID+"/content", nil))
	assertWailsLikeStatus(t, bookContentRec, http.StatusOK)
	if got := bookContentRec.header.Get("Content-Type"); got != "text/plain; charset=utf-8" {
		t.Fatalf("book Content-Type = %q, want text/plain; charset=utf-8", got)
	}
	if got := bookContentRec.body.String(); got != "" {
		t.Fatalf("book content = %q, want empty", got)
	}

	sourceContentRec := newWailsLikeRecorder()
	env.handler.ServeHTTP(sourceContentRec, httptest.NewRequest(http.MethodGet, "/api/shelves/default_shelf/books/"+bookID+"/sources/"+sourceID+"/content", nil))
	assertWailsLikeStatus(t, sourceContentRec, http.StatusOK)
	if got := sourceContentRec.header.Get("Content-Type"); got != "text/plain; charset=utf-8" {
		t.Fatalf("source Content-Type = %q, want text/plain; charset=utf-8", got)
	}
	if got := sourceContentRec.body.String(); got != "" {
		t.Fatalf("source content = %q, want empty", got)
	}

	logDir := t.TempDir()
	logApp, err := NewApp(&AppConf{
		Shelves: []*shelf.ShelfConfWithID{
			{
				ID: "default_shelf",
				ShelfConf: shelf.ShelfConf{
					Logger: logutil.LogConf{
						LogFile: logutil.LogFileConf{
							Type:   logutil.LogFileTypeNameRotate,
							Dir:    logDir,
							Prefix: "shelf",
						},
					},
					LibRoot: t.TempDir(),
				},
			},
		},
		StorePath:  t.TempDir(),
		CoverToJPG: false,
	})
	if err != nil {
		t.Fatalf("NewApp: %v", err)
	}
	t.Cleanup(func() {
		if err := logApp.Close(); err != nil {
			t.Fatalf("Close app: %v", err)
		}
	})

	if err := os.WriteFile(filepath.Join(logDir, "shelf-2024-01-02.log"), nil, 0o644); err != nil {
		t.Fatalf("WriteFile empty log: %v", err)
	}

	listRec := httptest.NewRecorder()
	logHandler := logApp.Handler()
	logHandler.ServeHTTP(listRec, httptest.NewRequest(http.MethodGet, "/api/logs", nil))
	assertStatus(t, listRec, http.StatusOK)

	logs := decodeJSON[[]LogFileEntry](t, listRec)
	var emptyLogID string
	for i := range logs {
		if logs[i].Filename == "shelf-2024-01-02.log" {
			emptyLogID = logs[i].ID
			break
		}
	}
	if emptyLogID == "" {
		t.Fatalf("expected empty shelf log in list, got %#v", logs)
	}

	logContentRec := newWailsLikeRecorder()
	logHandler.ServeHTTP(logContentRec, httptest.NewRequest(http.MethodGet, "/api/logs/"+emptyLogID+"/content", nil))
	assertWailsLikeStatus(t, logContentRec, http.StatusOK)
	if got := logContentRec.header.Get("Content-Type"); got != "text/plain; charset=utf-8" {
		t.Fatalf("log Content-Type = %q, want text/plain; charset=utf-8", got)
	}
	if got := logContentRec.body.String(); got != "" {
		t.Fatalf("log content = %q, want empty", got)
	}
}

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

// importFileBook uploads a book file with an explicit filename and content type,
// asserting the import succeeds. It complements importTextBook (which always
// uses "text/plain; charset=utf-8") by letting callers exercise other
// browser-supplied content types (e.g. for Markdown uploads).
func importFileBook(t *testing.T, env *apiTestEnv, filename, contentType, body string) Book {
	t.Helper()

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="file"; filename="%s"`, filename))
	if contentType != "" {
		h.Set("Content-Type", contentType)
	}
	part, err := writer.CreatePart(h)
	if err != nil {
		t.Fatalf("CreatePart: %v", err)
	}
	if _, err := io.Copy(part, strings.NewReader(body)); err != nil {
		t.Fatalf("write multipart file: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close multipart writer: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/shelves/default_shelf/books/import", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := env.do(req)
	assertStatus(t, rec, http.StatusCreated)
	assertJSONContentType(t, rec)
	return decodeJSON[Book](t, rec)
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

func TestAPIUpdateBookContract(t *testing.T) {
	env := newAPITestEnv(t)
	created := importTextBook(t, env, "Patch Me", "old/layer", "patch.txt", "body")

	body := `{"title":"Patched","authors":["Author A","Author B"],"tags":["tag1"],"language":"zh-Hant","comment":"updated comment","star":5,"layer":["new","layer"]}`
	rec := env.do(httptest.NewRequest(http.MethodPatch, "/api/shelves/default_shelf/books/"+created.Meta.ID, strings.NewReader(body)))
	assertStatus(t, rec, http.StatusOK)
	assertJSONContentType(t, rec)
	updated := decodeJSON[Book](t, rec)
	if updated.Meta.Title != "Patched" || updated.Meta.Comments != "updated comment" || updated.Meta.Language != "zh-Hant" || updated.Meta.Star != 5 {
		t.Fatalf("metadata was not updated: %#v", updated.Meta)
	}
	if len(updated.Meta.Authors) != 2 || updated.Meta.Authors[1] != "Author B" {
		t.Fatalf("authors = %#v", updated.Meta.Authors)
	}
	if strings.Join(updated.Layer, "/") != "new/layer" {
		t.Fatalf("layer = %#v, want new/layer", updated.Layer)
	}

	rec = env.do(httptest.NewRequest(http.MethodPatch, "/api/shelves/default_shelf/books/"+created.Meta.ID, strings.NewReader(`{"unexpected":true}`)))
	assertStatus(t, rec, http.StatusBadRequest)

	rec = env.do(httptest.NewRequest(http.MethodPatch, "/api/shelves/default_shelf/books/"+created.Meta.ID, strings.NewReader(`{"star":6}`)))
	assertStatus(t, rec, http.StatusBadRequest)

	// A malformed language tag is a client error, not a server failure.
	rec = env.do(httptest.NewRequest(http.MethodPatch, "/api/shelves/default_shelf/books/"+created.Meta.ID, strings.NewReader(`{"language":"!!!not-a-tag"}`)))
	assertStatus(t, rec, http.StatusBadRequest)
}

func TestAPIUpdateBookIdentifiersContract(t *testing.T) {
	env := newAPITestEnv(t)
	created := importTextBook(t, env, "Identifiers Book", "identifiers/layer", "identifiers.txt", "body")
	bookURL := "/api/shelves/default_shelf/books/" + created.Meta.ID

	// Setting identifiers is reflected in the PATCH response and a subsequent GET.
	rec := env.do(httptest.NewRequest(http.MethodPatch, bookURL, strings.NewReader(`{"identifiers":{"isbn":"978-0-13-468599-1","douban":"123"}}`)))
	assertStatus(t, rec, http.StatusOK)
	updated := decodeJSON[Book](t, rec)
	if updated.Meta.Identifiers["isbn"] != "978-0-13-468599-1" || updated.Meta.Identifiers["douban"] != "123" {
		t.Fatalf("identifiers not set in PATCH response: %#v", updated.Meta.Identifiers)
	}

	rec = env.do(httptest.NewRequest(http.MethodGet, bookURL, nil))
	assertStatus(t, rec, http.StatusOK)
	fetched := decodeJSON[Book](t, rec)
	if fetched.Meta.Identifiers["isbn"] != "978-0-13-468599-1" || fetched.Meta.Identifiers["douban"] != "123" {
		t.Fatalf("identifiers not set after GET: %#v", fetched.Meta.Identifiers)
	}

	// A subsequent PATCH with a new identifiers map fully replaces the old one (not a merge).
	rec = env.do(httptest.NewRequest(http.MethodPatch, bookURL, strings.NewReader(`{"identifiers":{"isbn":"999"}}`)))
	assertStatus(t, rec, http.StatusOK)
	replaced := decodeJSON[Book](t, rec)
	if replaced.Meta.Identifiers["isbn"] != "999" {
		t.Fatalf("identifiers isbn not replaced: %#v", replaced.Meta.Identifiers)
	}
	if _, ok := replaced.Meta.Identifiers["douban"]; ok {
		t.Fatalf("expected douban identifier to be gone after full replace, got: %#v", replaced.Meta.Identifiers)
	}

	// A PATCH that omits the identifiers field entirely leaves the existing value untouched.
	rec = env.do(httptest.NewRequest(http.MethodPatch, bookURL, strings.NewReader(`{"title":"Identifiers Book Renamed"}`)))
	assertStatus(t, rec, http.StatusOK)
	untouched := decodeJSON[Book](t, rec)
	if untouched.Meta.Title != "Identifiers Book Renamed" {
		t.Fatalf("title not updated: %#v", untouched.Meta)
	}
	if untouched.Meta.Identifiers["isbn"] != "999" {
		t.Fatalf("identifiers should be unchanged when omitted from PATCH body: %#v", untouched.Meta.Identifiers)
	}

	// An explicit empty identifiers object clears the map.
	rec = env.do(httptest.NewRequest(http.MethodPatch, bookURL, strings.NewReader(`{"identifiers":{}}`)))
	assertStatus(t, rec, http.StatusOK)
	cleared := decodeJSON[Book](t, rec)
	if len(cleared.Meta.Identifiers) != 0 {
		t.Fatalf("expected identifiers to be cleared, got: %#v", cleared.Meta.Identifiers)
	}

	// An identifiers map with an empty key is rejected.
	rec = env.do(httptest.NewRequest(http.MethodPatch, bookURL, strings.NewReader(`{"identifiers":{"":"x"}}`)))
	assertStatus(t, rec, http.StatusBadRequest)
}

func TestAPILayerMoveAndRenameContract(t *testing.T) {
	env := newAPITestEnv(t)
	created := importTextBook(t, env, "Layer Ops", "alpha/beta", "layer.txt", "body")

	rec := env.do(httptest.NewRequest(http.MethodPost, "/api/shelves/default_shelf/layers/gamma", nil))
	assertStatus(t, rec, http.StatusNoContent)

	rec = env.do(httptest.NewRequest(http.MethodPost, "/api/shelves/default_shelf/layer-moves", strings.NewReader(`{"layer":["alpha","beta"],"target_layer":["gamma"]}`)))
	assertStatus(t, rec, http.StatusNoContent)

	rec = env.do(httptest.NewRequest(http.MethodPatch, "/api/shelves/default_shelf/layers/gamma/beta", strings.NewReader(`{"name":"renamed"}`)))
	assertStatus(t, rec, http.StatusNoContent)

	rec = env.do(httptest.NewRequest(http.MethodGet, "/api/shelves/default_shelf/books/"+created.Meta.ID, nil))
	assertStatus(t, rec, http.StatusOK)
	got := decodeJSON[Book](t, rec)
	if strings.Join(got.Layer, "/") != "gamma/renamed" {
		t.Fatalf("layer = %#v, want gamma/renamed", got.Layer)
	}

	rec = env.do(httptest.NewRequest(http.MethodPost, "/api/shelves/default_shelf/layer-moves", strings.NewReader(`{"layer":["gamma","renamed"],"target_layer":["missing"]}`)))
	assertStatus(t, rec, http.StatusConflict)

	rec = env.do(httptest.NewRequest(http.MethodPatch, "/api/shelves/default_shelf/layers/gamma/renamed", strings.NewReader(`{"name":"bad/name"}`)))
	assertStatus(t, rec, http.StatusBadRequest)
}

func TestAPITrashLifecycleContract(t *testing.T) {
	env := newAPITestEnv(t)
	created := importTextBook(t, env, "Trash API", "origin/layer", "trash.txt", "body")

	rec := env.do(httptest.NewRequest(http.MethodPost, "/api/shelves/default_shelf/books/"+created.Meta.ID+"/trash", nil))
	assertStatus(t, rec, http.StatusNoContent)

	rec = env.do(httptest.NewRequest(http.MethodGet, "/api/shelves/default_shelf/books", nil))
	assertStatus(t, rec, http.StatusOK)
	if books := decodeJSON[[]Book](t, rec); len(books) != 0 {
		t.Fatalf("active books after trash = %d, want 0", len(books))
	}

	rec = env.do(httptest.NewRequest(http.MethodGet, "/api/shelves/default_shelf/trash/books", nil))
	assertStatus(t, rec, http.StatusOK)
	trashed := decodeJSON[[]map[string]any](t, rec)
	if len(trashed) != 1 {
		t.Fatalf("trashed books = %d, want 1", len(trashed))
	}
	if id, _ := trashed[0]["id"].(string); id != created.Meta.ID {
		t.Fatalf("trashed id = %q, want %q", id, created.Meta.ID)
	}

	rec = env.do(httptest.NewRequest(http.MethodPost, "/api/shelves/default_shelf/trash/books/"+created.Meta.ID+"/restore", nil))
	assertStatus(t, rec, http.StatusNoContent)

	rec = env.do(httptest.NewRequest(http.MethodGet, "/api/shelves/default_shelf/books", nil))
	assertStatus(t, rec, http.StatusOK)
	if books := decodeJSON[[]Book](t, rec); len(books) != 1 {
		t.Fatalf("active books after restore = %d, want 1", len(books))
	}

	rec = env.do(httptest.NewRequest(http.MethodDelete, "/api/shelves/default_shelf/books/"+created.Meta.ID, nil))
	assertStatus(t, rec, http.StatusNoContent)
	rec = env.do(httptest.NewRequest(http.MethodDelete, "/api/shelves/default_shelf/trash/books/"+created.Meta.ID, nil))
	assertStatus(t, rec, http.StatusNoContent)
	rec = env.do(httptest.NewRequest(http.MethodPost, "/api/shelves/default_shelf/trash/books/"+created.Meta.ID+"/restore", nil))
	assertStatus(t, rec, http.StatusNotFound)
}

func TestAPISplitConfigContract(t *testing.T) {
	env := newAPITestEnv(t)
	created := importTextBook(t, env, "Split Me", "", "split.txt", "one\ntwo\nthree")
	url := "/api/shelves/default_shelf/books/" + created.Meta.ID + "/split_config"

	rec := env.do(httptest.NewRequest(http.MethodGet, url, nil))
	assertStatus(t, rec, http.StatusOK)
	assertJSONContentType(t, rec)
	initial := decodeJSON[shelf.SplitConfig](t, rec)
	if initial.Type != shelf.SplitTypeNone {
		t.Fatalf("initial split type = %q, want none", initial.Type)
	}

	payload := `{"type":"line_count","line_count":42}`
	rec = env.do(httptest.NewRequest(http.MethodPatch, url, strings.NewReader(payload)))
	assertStatus(t, rec, http.StatusNoContent)

	rec = env.do(httptest.NewRequest(http.MethodGet, url, nil))
	assertStatus(t, rec, http.StatusOK)
	roundTrip := decodeJSON[shelf.SplitConfig](t, rec)
	if roundTrip.Type != shelf.SplitTypeLineCount || roundTrip.LineCount != 42 {
		t.Fatalf("round-trip split config = %#v", roundTrip)
	}
}

func TestAPICoverContract(t *testing.T) {
	env := newAPITestEnv(t)
	created := importTextBook(t, env, "Cover Me", "", "cover.txt", "body")
	url := "/api/shelves/default_shelf/books/" + created.Meta.ID + "/cover"

	rec := env.do(httptest.NewRequest(http.MethodGet, url, nil))
	assertStatus(t, rec, http.StatusNotFound)

	req := httptest.NewRequest(http.MethodPut, url, strings.NewReader("not image"))
	req.Header.Set("Content-Type", "text/plain")
	rec = env.do(req)
	assertStatus(t, rec, http.StatusBadRequest)

	req = httptest.NewRequest(http.MethodPut, url, bytes.NewReader(bytes.Repeat([]byte{'x'}, maxCoverBodySize+1)))
	req.Header.Set("Content-Type", "image/png")
	rec = env.do(req)
	assertStatus(t, rec, http.StatusRequestEntityTooLarge)

	coverBytes := []byte("fake png bytes")
	req = httptest.NewRequest(http.MethodPut, url, bytes.NewReader(coverBytes))
	req.Header.Set("Content-Type", "image/png")
	rec = env.do(req)
	assertStatus(t, rec, http.StatusNoContent)

	rec = env.do(httptest.NewRequest(http.MethodGet, url, nil))
	assertStatus(t, rec, http.StatusOK)
	if got := rec.Header().Get("Content-Type"); got != "image/png" {
		t.Fatalf("cover Content-Type = %q, want image/png", got)
	}
	if !bytes.Equal(rec.Body.Bytes(), coverBytes) {
		t.Fatalf("cover bytes = %q, want %q", rec.Body.Bytes(), coverBytes)
	}

	webpBytes := []byte("fake webp bytes")
	req = httptest.NewRequest(http.MethodPut, url, bytes.NewReader(webpBytes))
	req.Header.Set("Content-Type", "image/webp")
	rec = env.do(req)
	assertStatus(t, rec, http.StatusNoContent)

	rec = env.do(httptest.NewRequest(http.MethodGet, url, nil))
	assertStatus(t, rec, http.StatusOK)
	if got := rec.Header().Get("Content-Type"); got != "image/webp" {
		t.Fatalf("cover Content-Type = %q, want image/webp", got)
	}
	if !bytes.Equal(rec.Body.Bytes(), webpBytes) {
		t.Fatalf("cover bytes = %q, want %q", rec.Body.Bytes(), webpBytes)
	}

	rec = env.do(httptest.NewRequest(http.MethodDelete, url, nil))
	assertStatus(t, rec, http.StatusNoContent)
	rec = env.do(httptest.NewRequest(http.MethodGet, url, nil))
	assertStatus(t, rec, http.StatusNotFound)
}

func TestAPIStoreContract(t *testing.T) {
	env := newAPITestEnv(t)
	created := importTextBook(t, env, "Store Me", "", "store.txt", "body")
	marksURL := "/api/shelves/default_shelf/marks/" + created.Meta.ID

	rec := env.do(httptest.NewRequest(http.MethodGet, marksURL, nil))
	assertStatus(t, rec, http.StatusOK)
	assertJSONContentType(t, rec)
	mark := decodeJSON[store.Bookmark](t, rec)
	if mark.CharOffset != 0 {
		t.Fatalf("default mark char_offset = %d, want 0", mark.CharOffset)
	}

	rec = env.do(httptest.NewRequest(http.MethodPost, marksURL, strings.NewReader(`{"char_offset":123}`)))
	assertStatus(t, rec, http.StatusNoContent)
	rec = env.do(httptest.NewRequest(http.MethodGet, marksURL, nil))
	assertStatus(t, rec, http.StatusOK)
	mark = decodeJSON[store.Bookmark](t, rec)
	if mark.CharOffset != 123 {
		t.Fatalf("mark char_offset = %d, want 123", mark.CharOffset)
	}

	rec = env.do(httptest.NewRequest(http.MethodPost, marksURL, strings.NewReader(`{"char_offset":123,"extra":true}`)))
	assertStatus(t, rec, http.StatusBadRequest)
}

func TestAPICreateBookSourceContract(t *testing.T) {
	env := newAPITestEnv(t)
	created := importTextBook(t, env, "Source Book", "", "src.txt", "content")
	sourcesURL := "/api/shelves/default_shelf/books/" + created.Meta.ID + "/sources"

	// Creating a source on a nonexistent book should return 404.
	rec := env.do(httptest.NewRequest(http.MethodPost, "/api/shelves/default_shelf/books/no-such-book/sources", nil))
	assertStatus(t, rec, http.StatusNotFound)

	// Creating a source returns 200 with the new source metadata.
	rec = env.do(httptest.NewRequest(http.MethodPost, sourcesURL, nil))
	assertStatus(t, rec, http.StatusOK)
	assertJSONContentType(t, rec)
	newSource := decodeJSON[map[string]any](t, rec)
	newSourceID, _ := newSource["id"].(string)
	if newSourceID == "" {
		t.Fatalf("expected non-empty source id in response, got %#v", newSource)
	}

	// The new source should appear in the list.
	rec = env.do(httptest.NewRequest(http.MethodGet, sourcesURL, nil))
	assertStatus(t, rec, http.StatusOK)
	sources := decodeJSON[[]map[string]any](t, rec)
	found := false
	for _, s := range sources {
		if id, _ := s["id"].(string); id == newSourceID {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("newly created source %q not found in list: %#v", newSourceID, sources)
	}
}

func TestAPIDeleteBookSourceContract(t *testing.T) {
	env := newAPITestEnv(t)
	created := importTextBook(t, env, "Delete Source Book", "", "del.txt", "content")
	sourcesURL := "/api/shelves/default_shelf/books/" + created.Meta.ID + "/sources"

	// Create a new source to delete.
	rec := env.do(httptest.NewRequest(http.MethodPost, sourcesURL, nil))
	assertStatus(t, rec, http.StatusOK)
	newSource := decodeJSON[map[string]any](t, rec)
	newSourceID, _ := newSource["id"].(string)
	if newSourceID == "" {
		t.Fatalf("expected non-empty source id in response, got %#v", newSource)
	}

	// Deleting the source should succeed.
	rec = env.do(httptest.NewRequest(http.MethodDelete, sourcesURL+"/"+newSourceID, nil))
	assertStatus(t, rec, http.StatusNoContent)

	// The deleted source should no longer appear in the list.
	rec = env.do(httptest.NewRequest(http.MethodGet, sourcesURL, nil))
	assertStatus(t, rec, http.StatusOK)
	sources := decodeJSON[[]map[string]any](t, rec)
	for _, s := range sources {
		if id, _ := s["id"].(string); id == newSourceID {
			t.Fatalf("deleted source %q still present in list", newSourceID)
		}
	}

	// Deleting a nonexistent source should return 404.
	rec = env.do(httptest.NewRequest(http.MethodDelete, sourcesURL+"/nonexistent-source", nil))
	assertStatus(t, rec, http.StatusNotFound)

	// Deleting a source from a nonexistent book should return 404.
	rec = env.do(httptest.NewRequest(http.MethodDelete, "/api/shelves/default_shelf/books/no-such-book/sources/"+newSourceID, nil))
	assertStatus(t, rec, http.StatusNotFound)
}

func TestAPIImportEPUBBookContract(t *testing.T) {
	env := newAPITestEnv(t)
	archive := string(buildTestEPUB(t))

	imported := importFileBook(t, env, "three-body.epub", "application/epub+zip", archive)
	if imported.Meta == nil {
		t.Fatal("import response missing meta")
	}

	// The book's own dc:title beats the filename.
	if imported.Meta.Title != testEPUBTitle {
		t.Fatalf("title = %q, want %q", imported.Meta.Title, testEPUBTitle)
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
		"# " + testEPUBTitle,
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

	// The split config must let the reader recover the chapter names, which it
	// only does for regex splits.
	rec = env.do(httptest.NewRequest(http.MethodGet, base+"/split_config", nil))
	assertStatus(t, rec, http.StatusOK)
	assertJSONContentType(t, rec)
	split := decodeJSON[map[string]any](t, rec)
	if tp, _ := split["type"].(string); tp != "regex" {
		t.Fatalf("split config type = %q, want regex", tp)
	}
	if re, _ := split["regex"].(string); re != "^## " {
		t.Fatalf("split config regex = %q, want the markdown heading prefix", re)
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
	archive := string(buildTestEPUB(t))

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

	// A bare chapter title has no prefix to anchor a regex on, so the split
	// falls back to explicit boundaries.
	rec = env.do(httptest.NewRequest(http.MethodGet,
		"/api/shelves/default_shelf/books/"+plain.Meta.ID+"/split_config", nil))
	assertStatus(t, rec, http.StatusOK)
	split := decodeJSON[map[string]any](t, rec)
	if tp, _ := split["type"].(string); tp != "boundary" {
		t.Fatalf("split config type = %q, want boundary", tp)
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

// importEPUBWithStrategy uploads an EPUB with an explicit strategy field and
// asserts the import succeeds.
func importEPUBWithStrategy(t *testing.T, env *apiTestEnv, filename, archive, strategy string) Book {
	t.Helper()

	rec := postEPUBImport(t, env, filename, archive, strategy)
	assertStatus(t, rec, http.StatusCreated)
	assertJSONContentType(t, rec)
	return decodeJSON[Book](t, rec)
}

func importEPUBExpectingStatus(t *testing.T, env *apiTestEnv, filename, archive, strategy string, want int) *httptest.ResponseRecorder {
	t.Helper()

	rec := postEPUBImport(t, env, filename, archive, strategy)
	assertStatus(t, rec, want)
	return rec
}

func postEPUBImport(t *testing.T, env *apiTestEnv, filename, archive, strategy string) *httptest.ResponseRecorder {
	t.Helper()

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	if strategy != "" {
		if err := writer.WriteField("strategy", strategy); err != nil {
			t.Fatalf("WriteField strategy: %v", err)
		}
	}

	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="file"; filename="%s"`, filename))
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
	return env.do(req)
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

func TestAPISetCurrentBookSourceContract(t *testing.T) {
	env := newAPITestEnv(t)
	created := importTextBook(t, env, "Set Current Source Book", "", "src.txt", "content")
	sourcesURL := "/api/shelves/default_shelf/books/" + created.Meta.ID + "/sources"

	// Create a second source.
	rec := env.do(httptest.NewRequest(http.MethodPost, sourcesURL, nil))
	assertStatus(t, rec, http.StatusOK)
	newSource := decodeJSON[map[string]any](t, rec)
	newSourceID, _ := newSource["id"].(string)
	if newSourceID == "" {
		t.Fatalf("expected non-empty source id in response, got %#v", newSource)
	}

	// Setting the current source should succeed.
	rec = env.do(httptest.NewRequest(http.MethodPut, sourcesURL+"/"+newSourceID+"/current", nil))
	assertStatus(t, rec, http.StatusNoContent)

	// The book should reflect the new current source.
	rec = env.do(httptest.NewRequest(http.MethodGet, "/api/shelves/default_shelf/books/"+created.Meta.ID, nil))
	assertStatus(t, rec, http.StatusOK)
	bookData := decodeJSON[map[string]any](t, rec)
	meta, _ := bookData["meta"].(map[string]any)
	if currentSource, _ := meta["current_source"].(string); currentSource != newSourceID {
		t.Fatalf("expected current_source %q, got %q", newSourceID, currentSource)
	}

	// Setting current source for a nonexistent source should return 404.
	rec = env.do(httptest.NewRequest(http.MethodPut, sourcesURL+"/nonexistent-source/current", nil))
	assertStatus(t, rec, http.StatusNotFound)

	// Setting current source for a nonexistent book should return 404.
	rec = env.do(httptest.NewRequest(http.MethodPut, "/api/shelves/default_shelf/books/no-such-book/sources/"+newSourceID+"/current", nil))
	assertStatus(t, rec, http.StatusNotFound)
}

func TestAPIRefreshBookSourceMetaContract(t *testing.T) {
	env := newAPITestEnv(t)
	created := importTextBook(t, env, "Refresh Source", "", "refresh.txt", "line one\nline two\nline three")
	sourceID := created.Meta.CurrentSource
	refreshURL := "/api/shelves/default_shelf/books/" + created.Meta.ID + "/sources/" + sourceID + "/refresh"

	rec := env.do(httptest.NewRequest(http.MethodPost, refreshURL, nil))
	assertStatus(t, rec, http.StatusOK)
	assertJSONContentType(t, rec)
	meta := decodeJSON[map[string]any](t, rec)
	if id, _ := meta["id"].(string); id != sourceID {
		t.Fatalf("refreshed source id = %q, want %q", id, sourceID)
	}
	if lc, _ := meta["line_count"].(float64); lc <= 0 {
		t.Fatalf("line_count = %v, want > 0", lc)
	}
	if cc, _ := meta["char_count"].(float64); cc <= 0 {
		t.Fatalf("char_count = %v, want > 0", cc)
	}

	// Refreshing a nonexistent source returns 404.
	rec = env.do(httptest.NewRequest(http.MethodPost, "/api/shelves/default_shelf/books/"+created.Meta.ID+"/sources/nonexistent/refresh", nil))
	assertStatus(t, rec, http.StatusNotFound)

	// Refreshing a nonexistent book returns 404.
	rec = env.do(httptest.NewRequest(http.MethodPost, "/api/shelves/default_shelf/books/no-such-book/sources/"+sourceID+"/refresh", nil))
	assertStatus(t, rec, http.StatusNotFound)
}

func TestAPIVersionContract(t *testing.T) {
	env := newAPITestEnv(t)

	rec := env.do(httptest.NewRequest(http.MethodGet, "/api/version", nil))
	assertStatus(t, rec, http.StatusOK)
	resp := decodeJSON[versionResponse](t, rec)
	if resp.Version == "" {
		t.Fatalf("version is empty, want a non-empty value")
	}
}

func TestAPIReadOnlyModeContract(t *testing.T) {
	env := newAPITestEnv(t)
	env.app.conf.ReadOnly = true

	rec := env.do(httptest.NewRequest(http.MethodGet, "/api/mode", nil))
	assertStatus(t, rec, http.StatusOK)
	mode := decodeJSON[modeResponse](t, rec)
	if !mode.ReadOnly {
		t.Fatalf("read_only = false, want true")
	}

	rec = env.do(httptest.NewRequest(http.MethodPost, "/api/shelves/default_shelf/layers/blocked", nil))
	assertStatus(t, rec, http.StatusForbidden)

	rec = env.do(httptest.NewRequest(http.MethodGet, "/api/shelves/default_shelf/layers", nil))
	assertStatus(t, rec, http.StatusOK)
}

// TestAPIBookSchemaVersionContract asserts schema_version is present on the
// wire. It decodes into map[string]any rather than the Book struct on purpose:
// asserting through the Go type would pass tautologically even if the field
// never reached the JSON response.
func TestAPIBookSchemaVersionContract(t *testing.T) {
	env := newAPITestEnv(t)
	created := importTextBook(t, env, "Schema Version Book", "", "schema.txt", "body")

	rec := env.do(httptest.NewRequest(http.MethodGet, "/api/shelves/default_shelf/books", nil))
	assertStatus(t, rec, http.StatusOK)

	books := decodeJSON[[]map[string]any](t, rec)
	if len(books) != 1 {
		t.Fatalf("list returned %d books, want 1", len(books))
	}
	meta, ok := books[0]["meta"].(map[string]any)
	if !ok {
		t.Fatalf("book has no meta object: %#v", books[0])
	}
	if got := meta["schema_version"]; got != float64(shelf.BookMetaSchemaVersion) {
		t.Fatalf("list schema_version = %#v, want %d", got, shelf.BookMetaSchemaVersion)
	}

	rec = env.do(httptest.NewRequest(http.MethodGet, "/api/shelves/default_shelf/books/"+created.Meta.ID, nil))
	assertStatus(t, rec, http.StatusOK)

	book := decodeJSON[map[string]any](t, rec)
	meta, ok = book["meta"].(map[string]any)
	if !ok {
		t.Fatalf("book has no meta object: %#v", book)
	}
	if got := meta["schema_version"]; got != float64(shelf.BookMetaSchemaVersion) {
		t.Fatalf("get schema_version = %#v, want %d", got, shelf.BookMetaSchemaVersion)
	}
}

// TestAPIUnsupportedSchemaVersionReturns409 verifies the end-to-end behavior for
// a book written by a newer build: still readable over the API, but every
// attempt to modify it fails with 409 and leaves the file untouched.
func TestAPIUnsupportedSchemaVersionReturns409(t *testing.T) {
	env := newAPITestEnv(t)
	created := importTextBook(t, env, "Future Book", "", "future.txt", "body")

	metaPath := env.bookMetaPath(t, created.Meta.ID)
	raw, err := os.ReadFile(metaPath)
	if err != nil {
		t.Fatalf("read book.json: %v", err)
	}

	var onDisk map[string]any
	if err := json.Unmarshal(raw, &onDisk); err != nil {
		t.Fatalf("unmarshal book.json: %v", err)
	}
	onDisk["schema_version"] = shelf.BookMetaSchemaVersion + 1
	onDisk["reading_direction"] = "vertical-rl"
	bumped, err := json.MarshalIndent(onDisk, "", "  ")
	if err != nil {
		t.Fatalf("marshal book.json: %v", err)
	}
	if err := os.WriteFile(metaPath, bumped, 0o644); err != nil {
		t.Fatalf("write book.json: %v", err)
	}

	// The book stays readable and reports its real (newer) version, so a client
	// can tell the user this book needs a newer PlainShelf.
	rec := env.do(httptest.NewRequest(http.MethodGet, "/api/shelves/default_shelf/books/"+created.Meta.ID, nil))
	assertStatus(t, rec, http.StatusOK)
	book := decodeJSON[map[string]any](t, rec)
	meta, ok := book["meta"].(map[string]any)
	if !ok {
		t.Fatalf("book has no meta object: %#v", book)
	}
	if got := meta["schema_version"]; got != float64(shelf.BookMetaSchemaVersion+1) {
		t.Fatalf("schema_version = %#v, want %d", got, shelf.BookMetaSchemaVersion+1)
	}

	// Writing is refused.
	patch := httptest.NewRequest(http.MethodPatch, "/api/shelves/default_shelf/books/"+created.Meta.ID,
		strings.NewReader(`{"title":"Clobbered"}`))
	patch.Header.Set("Content-Type", "application/json")
	rec = env.do(patch)
	assertStatus(t, rec, http.StatusConflict)

	after, err := os.ReadFile(metaPath)
	if err != nil {
		t.Fatalf("re-read book.json: %v", err)
	}
	if !bytes.Equal(bumped, after) {
		t.Fatalf("refused write must leave book.json untouched, got:\n%s", after)
	}
	if !strings.Contains(string(after), "reading_direction") {
		t.Fatalf("unknown key must survive a refused write, got:\n%s", after)
	}
}

// TestAPIUnsupportedSchemaVersionDoesNotMoveLayer verifies the schema guard runs
// before the layer move. HandleAPIUpdateBook moves the book first, so a guard
// that only ran at SetMeta would rename the folder on disk and then report 409,
// leaving the client with a failed response for an applied mutation.
func TestAPIUnsupportedSchemaVersionDoesNotMoveLayer(t *testing.T) {
	env := newAPITestEnv(t)
	created := importTextBook(t, env, "Layer Guard", "origin/layer", "layer.txt", "body")

	metaPath := env.bookMetaPath(t, created.Meta.ID)
	raw, err := os.ReadFile(metaPath)
	if err != nil {
		t.Fatalf("read book.json: %v", err)
	}
	var onDisk map[string]any
	if err := json.Unmarshal(raw, &onDisk); err != nil {
		t.Fatalf("unmarshal book.json: %v", err)
	}
	onDisk["schema_version"] = shelf.BookMetaSchemaVersion + 1
	bumped, err := json.MarshalIndent(onDisk, "", "  ")
	if err != nil {
		t.Fatalf("marshal book.json: %v", err)
	}
	if err := os.WriteFile(metaPath, bumped, 0o644); err != nil {
		t.Fatalf("write book.json: %v", err)
	}

	patch := httptest.NewRequest(http.MethodPatch, "/api/shelves/default_shelf/books/"+created.Meta.ID,
		strings.NewReader(`{"layer":["moved","elsewhere"]}`))
	patch.Header.Set("Content-Type", "application/json")
	rec := env.do(patch)
	assertStatus(t, rec, http.StatusConflict)

	// The book must still be in its original layer, and still on disk there.
	rec = env.do(httptest.NewRequest(http.MethodGet, "/api/shelves/default_shelf/books/"+created.Meta.ID, nil))
	assertStatus(t, rec, http.StatusOK)
	book := decodeJSON[Book](t, rec)
	if got := strings.Join(book.Layer, "/"); got != "origin/layer" {
		t.Fatalf("layer = %q, want origin/layer — the refused request moved the book", got)
	}
	if _, err := os.Stat(metaPath); err != nil {
		t.Fatalf("book.json is no longer at its original path: %v", err)
	}
}
