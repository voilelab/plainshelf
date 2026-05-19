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
	"strings"
	"testing"

	"github.com/voilelab/plainshelf/server/store"
	"github.com/voilelab/plainshelf/shelf"
)

type apiTestEnv struct {
	app     *App
	handler http.Handler
}

func newAPITestEnv(t *testing.T) *apiTestEnv {
	t.Helper()

	app, err := NewApp(&AppConf{
		ShelfPath:        t.TempDir(),
		StorePath:        t.TempDir(),
		CoverToJPG:       false,
		ReadHistoryLimit: 2,
	})
	if err != nil {
		t.Fatalf("NewApp: %v", err)
	}
	t.Cleanup(func() {
		if err := app.Close(); err != nil {
			t.Fatalf("Close app: %v", err)
		}
	})

	return &apiTestEnv{app: app, handler: app.Handler()}
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

	req := httptest.NewRequest(http.MethodPost, "/api/books/import", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := env.do(req)
	assertStatus(t, rec, http.StatusCreated)
	assertJSONContentType(t, rec)
	return decodeJSON[Book](t, rec)
}

func TestAPIGetBooksContract(t *testing.T) {
	env := newAPITestEnv(t)

	rec := env.do(httptest.NewRequest(http.MethodGet, "/api/books", nil))
	assertStatus(t, rec, http.StatusOK)
	assertJSONContentType(t, rec)
	if got := decodeJSON[[]Book](t, rec); len(got) != 0 {
		t.Fatalf("empty library returned %d books", len(got))
	}

	alpha := importTextBook(t, env, "Alpha Tale", "/fiction/adventure", "alpha.txt", "alpha body")
	_ = importTextBook(t, env, "Beta Notes", "/notes", "beta.txt", "beta body")

	patchBody := `{"authors":["Ada"],"tags":["contract","api"],"language":"en","comment":"needle comment"}`
	rec = env.do(httptest.NewRequest(http.MethodPatch, "/api/books/"+alpha.Meta.ID, strings.NewReader(patchBody)))
	assertStatus(t, rec, http.StatusOK)

	rec = env.do(httptest.NewRequest(http.MethodGet, "/api/books?search=needle", nil))
	assertStatus(t, rec, http.StatusOK)
	books := decodeJSON[[]Book](t, rec)
	if len(books) != 1 {
		t.Fatalf("search returned %d books, want 1", len(books))
	}
	got := books[0]
	if got.Meta == nil || got.Meta.ID != alpha.Meta.ID || got.Meta.Title != "Alpha Tale" {
		t.Fatalf("unexpected searched book meta: %#v", got.Meta)
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
		t.Fatal("import response missing current_snapshot")
	}

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	if err := writer.WriteField("title", "Missing File"); err != nil {
		t.Fatalf("WriteField: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close multipart writer: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/books/import", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := env.do(req)
	assertStatus(t, rec, http.StatusBadRequest)

	buf.Reset()
	writer = multipart.NewWriter(&buf)
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", `form-data; name="file"; filename="book.epub"`)
	h.Set("Content-Type", "text/plain")
	part, err := writer.CreatePart(h)
	if err != nil {
		t.Fatalf("CreatePart: %v", err)
	}
	if _, err := part.Write([]byte("not a txt upload")); err != nil {
		t.Fatalf("write bad file: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close multipart writer: %v", err)
	}
	req = httptest.NewRequest(http.MethodPost, "/api/books/import", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec = env.do(req)
	assertStatus(t, rec, http.StatusBadRequest)
}

func TestAPIUpdateBookContract(t *testing.T) {
	env := newAPITestEnv(t)
	created := importTextBook(t, env, "Patch Me", "old/layer", "patch.txt", "body")

	body := `{"title":"Patched","authors":["Author A","Author B"],"tags":["tag1"],"language":"zh-Hant","comment":"updated comment","layer":["new","layer"]}`
	rec := env.do(httptest.NewRequest(http.MethodPatch, "/api/books/"+created.Meta.ID, strings.NewReader(body)))
	assertStatus(t, rec, http.StatusOK)
	assertJSONContentType(t, rec)
	updated := decodeJSON[Book](t, rec)
	if updated.Meta.Title != "Patched" || updated.Meta.Comments != "updated comment" || updated.Meta.Language != "zh-Hant" {
		t.Fatalf("metadata was not updated: %#v", updated.Meta)
	}
	if len(updated.Meta.Authors) != 2 || updated.Meta.Authors[1] != "Author B" {
		t.Fatalf("authors = %#v", updated.Meta.Authors)
	}
	if strings.Join(updated.Layer, "/") != "new/layer" {
		t.Fatalf("layer = %#v, want new/layer", updated.Layer)
	}

	rec = env.do(httptest.NewRequest(http.MethodPatch, "/api/books/"+created.Meta.ID, strings.NewReader(`{"unexpected":true}`)))
	assertStatus(t, rec, http.StatusBadRequest)
}

func TestAPISplitConfigContract(t *testing.T) {
	env := newAPITestEnv(t)
	created := importTextBook(t, env, "Split Me", "", "split.txt", "one\ntwo\nthree")
	url := "/api/books/" + created.Meta.ID + "/split_config"

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
	url := "/api/books/" + created.Meta.ID + "/cover"

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

	rec = env.do(httptest.NewRequest(http.MethodDelete, url, nil))
	assertStatus(t, rec, http.StatusNoContent)
	rec = env.do(httptest.NewRequest(http.MethodGet, url, nil))
	assertStatus(t, rec, http.StatusNotFound)
}

func TestAPIStoreContract(t *testing.T) {
	env := newAPITestEnv(t)
	created := importTextBook(t, env, "Store Me", "", "store.txt", "body")
	marksURL := "/api/marks/" + created.Meta.ID

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

	rec = env.do(httptest.NewRequest(http.MethodGet, "/api/read_history", nil))
	assertStatus(t, rec, http.StatusOK)
	assertJSONContentType(t, rec)
	if history := decodeJSON[[]string](t, rec); len(history) != 0 {
		t.Fatalf("initial read history = %#v, want empty", history)
	}

	rec = env.do(httptest.NewRequest(http.MethodPost, "/api/read_history", nil))
	assertStatus(t, rec, http.StatusBadRequest)
	rec = env.do(httptest.NewRequest(http.MethodPost, "/api/read_history?book_id="+created.Meta.ID, nil))
	assertStatus(t, rec, http.StatusNoContent)
	rec = env.do(httptest.NewRequest(http.MethodGet, "/api/read_history", nil))
	assertStatus(t, rec, http.StatusOK)
	history := decodeJSON[[]string](t, rec)
	if len(history) != 1 || history[0] != created.Meta.ID {
		t.Fatalf("read history = %#v, want [%s]", history, created.Meta.ID)
	}

	rec = env.do(httptest.NewRequest(http.MethodDelete, "/api/read_history", nil))
	assertStatus(t, rec, http.StatusNoContent)
	rec = env.do(httptest.NewRequest(http.MethodGet, "/api/read_history", nil))
	assertStatus(t, rec, http.StatusOK)
	if history = decodeJSON[[]string](t, rec); len(history) != 0 {
		t.Fatalf("cleared read history = %#v, want empty", history)
	}
}
