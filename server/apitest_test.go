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

	"github.com/voilelab/plainshelf/shelf"
)

type apiTestEnv struct {
	app     *App
	handler http.Handler
	// libRoot is the shelf's on-disk root, so tests can inspect or tamper with
	// the files behind the API.
	libRoot string
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

	// Contract tests assert response shapes and status codes, so they must not
	// race the shelf's initial scan: a read issued before it finishes is
	// answered 503, which is correct behaviour and a meaningless failure here.
	waitForShelves(t, app)

	return &apiTestEnv{app: app, handler: app.Handler(), libRoot: libRoot}
}

// waitForShelves blocks until every configured shelf has finished its initial
// scan, so a test's first request cannot arrive while the shelf is still
// initializing. Endpoints that report "shelf is initializing" have their own
// tests; everywhere else it is noise.
func waitForShelves(t *testing.T, app *App) {
	t.Helper()

	for _, shelfData := range app.shelfManager.GetAllShelves() {
		if err := shelfData.WaitReady(t.Context()); err != nil {
			t.Fatalf("WaitReady for shelf %s: %v", shelfData.ID, err)
		}
	}
}

// do sends the request, filling in the local token for mutating methods so each
// test does not have to. The token gate itself is exercised by the contract
// tests, which have a doRaw that leaves the request untouched.
func (env *apiTestEnv) do(req *http.Request) *httptest.ResponseRecorder {
	if IsMutatingMethod(req.Method) && req.Header.Get(env.app.SecurityTokenHeader()) == "" && req.Header.Get("Authorization") == "" {
		req.Header.Set(env.app.SecurityTokenHeader(), env.app.SecurityToken())
	}

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

// importFileBook uploads a book file with an explicit filename and content type,
// asserting the import succeeds.
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

// importEPUBWithStrategy uploads an EPUB with an explicit strategy field and
// asserts the import succeeds.
func importEPUBWithStrategy(t *testing.T, env *apiTestEnv, filename, archive, strategy string) Book {
	t.Helper()

	rec := postEPUBImport(t, env, filename, archive, strategy)
	assertStatus(t, rec, http.StatusCreated)
	assertJSONContentType(t, rec)
	return decodeJSON[Book](t, rec)
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
