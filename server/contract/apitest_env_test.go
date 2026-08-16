package contract_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/voilelab/plainshelf/internal/logutil"
	"github.com/voilelab/plainshelf/server"
	"github.com/voilelab/plainshelf/shelf"
)

// defaultShelfID is the only shelf the contract tests configure, so its ID is
// spelled once here and reused by the URL builders in apitest_book_test.go.
const defaultShelfID = "default_shelf"

// maxBinaryUploadSize is the body limit the cover and asset routes enforce. It
// is restated here rather than exported from the server: the limit is part of
// what these tests pin, so a change to it has to be a change to both.
const maxBinaryUploadSize = 20 << 20

// appConfOption adjusts the configuration a contract-test app is built from.
type appConfOption func(*server.AppConf)

// withLibRoot pins the shelf root instead of taking a fresh temp directory, for
// tests that need two apps to see the same shelf.
func withLibRoot(libRoot string) appConfOption {
	return func(conf *server.AppConf) {
		conf.Shelves[0].LibRoot = libRoot
	}
}

// withStorePath pins the app store, which is what makes state such as the book
// cache writer ID survive a simulated restart.
func withStorePath(storePath string) appConfOption {
	return func(conf *server.AppConf) {
		conf.StorePath = storePath
	}
}

// withAppLogFile logs the app to a single named file.
func withAppLogFile(filename string) appConfOption {
	return func(conf *server.AppConf) {
		conf.Logger = logutil.LogConf{
			LogFile: logutil.LogFileConf{
				Type:     logutil.LogFileTypeName,
				Filename: filename,
			},
		}
	}
}

// withAppLogDir logs the app to a rotating set of files under dir, which is the
// shape the log API reports one entry per file for.
func withAppLogDir(dir, prefix string) appConfOption {
	return func(conf *server.AppConf) {
		conf.Logger = rotatingLogConf(dir, prefix)
	}
}

// withShelfLogDir gives the shelf its own rotating log directory.
func withShelfLogDir(dir, prefix string) appConfOption {
	return func(conf *server.AppConf) {
		conf.Shelves[0].Logger = rotatingLogConf(dir, prefix)
	}
}

// withSecurity pins the security configuration, which is what the local-token
// and CORS gates are asserted against.
func withSecurity(security *server.SecurityConf) appConfOption {
	return func(conf *server.AppConf) {
		conf.Security = security
	}
}

func rotatingLogConf(dir, prefix string) logutil.LogConf {
	return logutil.LogConf{
		LogFile: logutil.LogFileConf{
			Type:   logutil.LogFileTypeNameRotate,
			Dir:    dir,
			Prefix: prefix,
		},
	}
}

// apiAppConf is the configuration every contract-test app starts from: one shelf
// on a fresh temp root, a fresh store, and no cover conversion. The book cache
// interval is short so tests that observe an exported cache do not wait an hour
// for it.
func apiAppConf(t *testing.T, opts ...appConfOption) *server.AppConf {
	t.Helper()

	conf := &server.AppConf{
		Shelves: []*shelf.ShelfConfWithID{
			{
				ID: defaultShelfID,
				ShelfConf: shelf.ShelfConf{
					LibRoot:           t.TempDir(),
					BookCacheInterval: "1s",
				},
			},
		},
		StorePath:  t.TempDir(),
		CoverToJPG: false,
	}
	for _, opt := range opts {
		opt(conf)
	}
	return conf
}

type apiTestEnv struct {
	app     *server.App
	handler http.Handler
	// libRoot is the shelf's on-disk root, so tests can inspect or tamper with
	// the files behind the API.
	libRoot string
}

// newAPITestEnv builds a started app: the background worker is running and every
// shelf has finished its initial scan, which is what an API contract test wants.
func newAPITestEnv(t *testing.T, opts ...appConfOption) *apiTestEnv {
	t.Helper()

	env := newUnstartedAPITestEnv(t, opts...)

	// Start the background worker so endpoints backed by task chains behave as
	// they do in production.
	if err := env.app.Start(); err != nil {
		t.Fatalf("Start app: %v", err)
	}

	// Contract tests assert response shapes and status codes, so they must not
	// race the shelf's initial scan: a read issued before it finishes is
	// answered 503, which is correct behaviour and a meaningless failure here.
	waitForShelves(t, env.app)

	return env
}

// newUnstartedAPITestEnv builds the app and its handler without starting the
// worker or waiting for the initial scan. Endpoints that never touch the shelf —
// /api/logs above all — are tested against it, so starting a worker whose own
// logging would add files to the directory under test is avoided.
func newUnstartedAPITestEnv(t *testing.T, opts ...appConfOption) *apiTestEnv {
	t.Helper()

	conf := apiAppConf(t, opts...)
	app, err := server.NewApp(conf)
	if err != nil {
		t.Fatalf("NewApp: %v", err)
	}
	t.Cleanup(func() {
		if err := app.Close(); err != nil {
			t.Fatalf("Close app: %v", err)
		}
	})

	return &apiTestEnv{app: app, handler: app.Handler(), libRoot: conf.Shelves[0].LibRoot}
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

// setReadOnly flips read-only mode and restores the previous value when the test
// ends, so a test that keeps making requests afterwards is not left in it.
func (env *apiTestEnv) setReadOnly(t *testing.T, readOnly bool) {
	t.Helper()

	previous := env.app.Conf().ReadOnly
	env.app.Conf().ReadOnly = readOnly
	t.Cleanup(func() { env.app.Conf().ReadOnly = previous })
}

func (env *apiTestEnv) get(url string) *httptest.ResponseRecorder {
	return env.request(http.MethodGet, url, nil)
}

// getIfNoneMatch issues a revalidating GET. An empty header sends no
// If-None-Match at all, which is a distinct case from sending an empty one.
func (env *apiTestEnv) getIfNoneMatch(url, ifNoneMatch string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, url, nil)
	if ifNoneMatch != "" {
		req.Header.Set("If-None-Match", ifNoneMatch)
	}
	return env.do(req)
}

func (env *apiTestEnv) post(url string, body io.Reader) *httptest.ResponseRecorder {
	return env.request(http.MethodPost, url, body)
}

func (env *apiTestEnv) put(url string, body io.Reader) *httptest.ResponseRecorder {
	return env.request(http.MethodPut, url, body)
}

// putContent uploads a body whose declared content type matters, which is how a
// cover's stored image format is chosen.
func (env *apiTestEnv) putContent(url, contentType string, body io.Reader) *httptest.ResponseRecorder {
	return env.requestTyped(http.MethodPut, url, contentType, body)
}

// patchContent replaces text whose declared content type tells the server how to
// decode it.
func (env *apiTestEnv) patchContent(url, contentType string, body io.Reader) *httptest.ResponseRecorder {
	return env.requestTyped(http.MethodPatch, url, contentType, body)
}

func (env *apiTestEnv) requestTyped(method, url, contentType string, body io.Reader) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, url, body)
	req.Header.Set("Content-Type", contentType)
	return env.do(req)
}

func (env *apiTestEnv) patch(url string, body io.Reader) *httptest.ResponseRecorder {
	return env.request(http.MethodPatch, url, body)
}

func (env *apiTestEnv) delete(url string) *httptest.ResponseRecorder {
	return env.request(http.MethodDelete, url, nil)
}

// request issues a request through the app's handler with the local token
// attached when the method needs it. Tests that set their own headers build the
// request themselves and call do.
func (env *apiTestEnv) request(method, url string, body io.Reader) *httptest.ResponseRecorder {
	return env.do(httptest.NewRequest(method, url, body))
}

// do sends the request, filling in the local token for mutating methods so each
// test does not have to. Use doRaw to exercise the token gate itself.
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

// getWailsLike issues a GET through a recorder that behaves like the Wails asset
// server's: it reports no status until the handler writes one.
func (env *apiTestEnv) getWailsLike(url string) *wailsLikeRecorder {
	rec := newWailsLikeRecorder()
	env.handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, url, nil))
	return rec
}
