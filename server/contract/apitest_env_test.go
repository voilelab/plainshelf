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

// DefaultShelfID is the only shelf the contract tests configure, so its ID is
// spelled once here and reused by the URL builders in apitest_book_test.go.
const DefaultShelfID = "default_shelf"

// MaxBinaryUploadSize is the body limit the cover and asset routes enforce. It
// is restated here rather than exported from the server: the limit is part of
// what these tests pin, so a change to it has to be a change to both.
const MaxBinaryUploadSize = 20 << 20

// MaxLogRetentionDays is the ceiling the log_retention_days setting enforces,
// restated for the same reason as MaxBinaryUploadSize: the bound is part of
// what these tests pin.
const MaxLogRetentionDays = 3650

// Option adjusts the configuration a contract-test app is built from.
type Option func(*server.AppConf)

// WithLibRoot pins the shelf root instead of taking a fresh temp directory, for
// tests that need two apps to see the same shelf.
func WithLibRoot(libRoot string) Option {
	return func(conf *server.AppConf) {
		conf.Shelves[0].LibRoot = libRoot
	}
}

// WithStorePath pins the app store, which is what makes state such as the book
// cache writer ID survive a simulated restart.
func WithStorePath(storePath string) Option {
	return func(conf *server.AppConf) {
		conf.StorePath = storePath
	}
}

// WithAppLogFile logs the app to a single named file.
func WithAppLogFile(filename string) Option {
	return func(conf *server.AppConf) {
		conf.Logger = logutil.LogConf{
			LogFile: logutil.LogFileConf{
				Type:     logutil.LogFileTypeName,
				Filename: filename,
			},
		}
	}
}

// WithAppLogDir logs the app to a rotating set of files under dir, which is the
// shape the log API reports one entry per file for.
func WithAppLogDir(dir, prefix string) Option {
	return func(conf *server.AppConf) {
		conf.Logger = rotatingLogConf(dir, prefix)
	}
}

// WithShelfLogDir gives the shelf its own rotating log directory.
func WithShelfLogDir(dir, prefix string) Option {
	return func(conf *server.AppConf) {
		conf.Shelves[0].Logger = rotatingLogConf(dir, prefix)
	}
}

// WithLogRetention pins how long rotated log files are kept. The log tests seed
// files dated well in the past, which the default retention window would expire
// on the first rotation the app's own logging triggers.
func WithLogRetention(days int) Option {
	return func(conf *server.AppConf) {
		conf.Logger.LogFile.RetentionDays = &days
		for _, shelfConf := range conf.Shelves {
			shelfConf.Logger.LogFile.RetentionDays = &days
		}
	}
}

// WithReadOnlyShelf opens the shelf read-only, which is the per-shelf setting
// rather than the app-wide read-only mode SetReadOnly toggles.
func WithReadOnlyShelf() Option {
	return func(conf *server.AppConf) {
		conf.Shelves[0].ReadOnly = true
	}
}

// WithReadOnlyServer starts the whole app in read-only mode, which is the
// app-wide setting an operator writes as `read_only` next to `shelves`. Unlike
// SetReadOnly it is applied before the shelves are opened, so the shelf sees it
// too rather than only the HTTP gate.
func WithReadOnlyServer() Option {
	return func(conf *server.AppConf) {
		conf.ReadOnly = true
	}
}

// WithSecurity pins the security configuration, which is what the local-token
// and CORS gates are asserted against.
func WithSecurity(security *server.SecurityConf) Option {
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

// AppConf is the configuration every contract-test app starts from: one shelf
// on a fresh temp root, a fresh store, and no cover conversion. The book cache
// interval is short so tests that observe an exported cache do not wait an hour
// for it.
func AppConf(t *testing.T, opts ...Option) *server.AppConf {
	t.Helper()

	conf := &server.AppConf{
		Shelves: []*shelf.ShelfConfWithID{
			{
				ID: DefaultShelfID,
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

type Env struct {
	App     *server.App
	handler http.Handler
	// LibRoot is the shelf's on-disk root, so tests can inspect or tamper with
	// the files behind the API.
	LibRoot string
}

// New builds a started app: the background worker is running and every
// shelf has finished its initial scan, which is what an API contract test wants.
func New(t *testing.T, opts ...Option) *Env {
	t.Helper()

	env := NewUnstarted(t, opts...)

	// Start the background worker so endpoints backed by task chains behave as
	// they do in production.
	if err := env.App.Start(); err != nil {
		t.Fatalf("Start app: %v", err)
	}

	// Contract tests assert response shapes and status codes, so they must not
	// race the shelf's initial scan: a read issued before it finishes is
	// answered 503, which is correct behaviour and a meaningless failure here.
	waitForShelves(t, env.App)

	return env
}

// NewUnstarted builds the app and its handler without starting the
// worker or waiting for the initial scan. Endpoints that never touch the shelf —
// /api/logs above all — are tested against it, so starting a worker whose own
// logging would add files to the directory under test is avoided.
func NewUnstarted(t *testing.T, opts ...Option) *Env {
	t.Helper()

	conf := AppConf(t, opts...)
	app, err := server.NewApp(conf)
	if err != nil {
		t.Fatalf("NewApp: %v", err)
	}
	t.Cleanup(func() {
		if err := app.Close(); err != nil {
			t.Fatalf("Close app: %v", err)
		}
	})

	return &Env{App: app, handler: app.Handler(), LibRoot: conf.Shelves[0].LibRoot}
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

// SetReadOnly flips read-only mode and restores the previous value when the test
// ends, so a test that keeps making requests afterwards is not left in it.
func (env *Env) SetReadOnly(t *testing.T, readOnly bool) {
	t.Helper()

	previous := env.App.Conf().ReadOnly
	env.App.Conf().ReadOnly = readOnly
	t.Cleanup(func() { env.App.Conf().ReadOnly = previous })
}

func (env *Env) Get(url string) *httptest.ResponseRecorder {
	return env.Request(http.MethodGet, url, nil)
}

// GetIfNoneMatch issues a revalidating GET. An empty header sends no
// If-None-Match at all, which is a distinct case from sending an empty one.
func (env *Env) GetIfNoneMatch(url, ifNoneMatch string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, url, nil)
	if ifNoneMatch != "" {
		req.Header.Set("If-None-Match", ifNoneMatch)
	}
	return env.Do(req)
}

func (env *Env) Post(url string, body io.Reader) *httptest.ResponseRecorder {
	return env.Request(http.MethodPost, url, body)
}

func (env *Env) Put(url string, body io.Reader) *httptest.ResponseRecorder {
	return env.Request(http.MethodPut, url, body)
}

// PutContent uploads a body whose declared content type matters, which is how a
// cover's stored image format is chosen.
func (env *Env) PutContent(url, contentType string, body io.Reader) *httptest.ResponseRecorder {
	return env.RequestTyped(http.MethodPut, url, contentType, body)
}

// PatchContent replaces text whose declared content type tells the server how to
// decode it.
func (env *Env) PatchContent(url, contentType string, body io.Reader) *httptest.ResponseRecorder {
	return env.RequestTyped(http.MethodPatch, url, contentType, body)
}

func (env *Env) RequestTyped(method, url, contentType string, body io.Reader) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, url, body)
	req.Header.Set("Content-Type", contentType)
	return env.Do(req)
}

func (env *Env) Patch(url string, body io.Reader) *httptest.ResponseRecorder {
	return env.Request(http.MethodPatch, url, body)
}

func (env *Env) Delete(url string) *httptest.ResponseRecorder {
	return env.Request(http.MethodDelete, url, nil)
}

// request issues a request through the app's handler with the local token
// attached when the method needs it. Tests that set their own headers build the
// request themselves and call do.
func (env *Env) Request(method, url string, body io.Reader) *httptest.ResponseRecorder {
	return env.Do(httptest.NewRequest(method, url, body))
}

// do sends the request, filling in the local token so each test does not have
// to. Use DoRaw to exercise the token gate itself.
func (env *Env) Do(req *http.Request) *httptest.ResponseRecorder {
	return env.DoRaw(env.withToken(req))
}

// withToken attaches the local token wherever the security gate asks for one:
// every mutation, and the log API whatever the method. The rule is read from
// the server rather than restated, so a route moving behind the token does not
// need every test that reaches it to be found by hand.
func (env *Env) withToken(req *http.Request) *http.Request {
	needsToken := server.IsMutatingMethod(req.Method) || server.IsLogAPIPath(req.URL.Path)
	if needsToken && req.Header.Get(env.App.SecurityTokenHeader()) == "" && req.Header.Get("Authorization") == "" {
		req.Header.Set(env.App.SecurityTokenHeader(), env.App.SecurityToken())
	}
	return req
}

func (env *Env) DoRaw(req *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	env.handler.ServeHTTP(rec, req)
	return rec
}

// GetWailsLike issues a GET through a recorder that behaves like the Wails asset
// server's: it reports no status until the handler writes one.
func (env *Env) GetWailsLike(url string) *wailsLikeRecorder {
	rec := newWailsLikeRecorder()
	env.handler.ServeHTTP(rec, env.withToken(httptest.NewRequest(http.MethodGet, url, nil)))
	return rec
}
