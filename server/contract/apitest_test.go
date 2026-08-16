package contract_test

import (
	"bytes"
	"context"
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
	"sync"
	"testing"
	"time"

	"github.com/voilelab/plainshelf/internal/taskutil"
	"github.com/voilelab/plainshelf/server"
	"github.com/voilelab/plainshelf/shelf"
)

type apiTestEnv struct {
	app     *server.App
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

	app, err := server.NewApp(&server.AppConf{
		Shelves: []*shelf.ShelfConfWithID{
			{
				ID: "default_shelf",
				ShelfConf: shelf.ShelfConf{
					LibRoot:           libRoot,
					BookCacheInterval: "1s",
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
func waitForShelves(t *testing.T, app *server.App) {
	t.Helper()

	for _, shelfData := range app.ShelfManager().GetAllShelves() {
		if err := shelfData.WaitReady(t.Context()); err != nil {
			t.Fatalf("WaitReady for shelf %s: %v", shelfData.ID, err)
		}
	}
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
	if server.IsMutatingMethod(req.Method) && req.Header.Get(env.app.SecurityTokenHeader()) == "" && req.Header.Get("Authorization") == "" {
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

func importTextBook(t *testing.T, env *apiTestEnv, title, layer, filename, body string) server.Book {
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
	return decodeJSON[server.Book](t, rec)
}

// importFileBook uploads a book file with an explicit filename and content type,
// asserting the import succeeds. It complements importTextBook (which always
// uses "text/plain; charset=utf-8") by letting callers exercise other
// browser-supplied content types (e.g. for Markdown uploads).
func importFileBook(t *testing.T, env *apiTestEnv, filename, contentType, body string) server.Book {
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
	return decodeJSON[server.Book](t, rec)
}

// currentSourceID returns the source the imported book is reading from.
func (env *apiTestEnv) currentSourceID(t *testing.T, bookID string) string {
	t.Helper()

	rec := env.do(httptest.NewRequest(http.MethodGet, "/api/shelves/default_shelf/books/"+bookID+"/sources", nil))
	assertStatus(t, rec, http.StatusOK)

	metas := decodeJSON[[]shelf.SourceMeta](t, rec)
	if len(metas) == 0 {
		t.Fatalf("book %s has no sources", bookID)
	}
	return metas[0].ID
}

// writeSourceAsset drops a file into a source's assets/ directory, which is how
// an illustration gets there today: the API serves assets but cannot store them.
func (env *apiTestEnv) writeSourceAsset(t *testing.T, bookID, sourceID, name string, data []byte) {
	t.Helper()

	bookDir := filepath.Dir(env.bookMetaPath(t, bookID))
	assetDir := filepath.Join(bookDir, shelf.SourcesFolder, sourceID, shelf.SourceAssetsFolder)
	if err := os.MkdirAll(assetDir, 0755); err != nil {
		t.Fatalf("MkdirAll(%q): %v", assetDir, err)
	}
	if err := os.WriteFile(filepath.Join(assetDir, name), data, 0644); err != nil {
		t.Fatalf("WriteFile(%q): %v", name, err)
	}
}

// importEPUBWithStrategy uploads an EPUB with an explicit strategy field and
// asserts the import succeeds.
func importEPUBWithStrategy(t *testing.T, env *apiTestEnv, filename, archive, strategy string) server.Book {
	t.Helper()

	rec := postEPUBImport(t, env, filename, archive, strategy)
	assertStatus(t, rec, http.StatusCreated)
	assertJSONContentType(t, rec)
	return decodeJSON[server.Book](t, rec)
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

// gateTask occupies the worker until it is released.
type gateTask struct {
	started chan struct{}
	release chan struct{}
	once    sync.Once
}

func (t *gateTask) Run(ctx context.Context) error {
	t.once.Do(func() { close(t.started) })
	select {
	case <-t.release:
	case <-ctx.Done():
	}
	return nil
}

func (t *gateTask) Name() string { return "gate" }

func (t *gateTask) Title() string { return "gate" }

func (t *gateTask) Description() string { return "gate" }

func (t *gateTask) Percentage() float64 { return 0 }

func (t *gateTask) Status() taskutil.Status { return taskutil.StatusRunning }

// blockWorker occupies the single worker goroutine so that chains submitted
// afterwards stay queued. The returned function releases it.
func blockWorker(t *testing.T, env *apiTestEnv) func() {
	t.Helper()

	gate := &gateTask{started: make(chan struct{}), release: make(chan struct{})}
	if _, err := env.app.TaskChains().Submit(&taskutil.TaskChain{
		Name:  "gate_chain",
		Tasks: []taskutil.Task{gate},
	}); err != nil {
		t.Fatalf("Submit gate chain: %v", err)
	}

	<-gate.started

	var once sync.Once
	release := func() { once.Do(func() { close(gate.release) }) }
	t.Cleanup(release)
	return release
}

// waitForTaskChain polls the task chain endpoint until the chain reaches a
// terminal status, mirroring what the trash page does.
func waitForTaskChain(t *testing.T, env *apiTestEnv, taskChainID string) server.TaskChain {
	t.Helper()

	deadline := time.Now().Add(10 * time.Second)
	for {
		rec := env.do(httptest.NewRequest(http.MethodGet, "/api/taskchains/"+taskChainID, nil))
		assertStatus(t, rec, http.StatusOK)
		chain := decodeJSON[server.TaskChain](t, rec)

		switch chain.Status {
		case "completed", "partially_completed", "failed":
			return chain
		}

		if time.Now().After(deadline) {
			t.Fatalf("task chain %s did not finish, last status %q", taskChainID, chain.Status)
		}
		time.Sleep(5 * time.Millisecond)
	}
}
