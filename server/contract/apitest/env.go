package apitest

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/voilelab/plainshelf/internal/logutil"
	"github.com/voilelab/plainshelf/server"
	"github.com/voilelab/plainshelf/shelf"
)

const DefaultShelfID = "default_shelf"

const MaxBinaryUploadSize = 20 << 20

const MaxTextUploadSize = 100 << 20

const MaxLogRetentionDays = 3650

type Option func(*server.AppConf)

func WithLibRoot(libRoot string) Option {
	return func(conf *server.AppConf) {
		conf.Shelves[0].LibRoot = libRoot
	}
}

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

func WithAppLogDir(dir, prefix string) Option {
	return func(conf *server.AppConf) {
		conf.Logger = rotatingLogConf(dir, prefix)
	}
}

func WithShelfLogDir(dir, prefix string) Option {
	return func(conf *server.AppConf) {
		conf.Shelves[0].Logger = rotatingLogConf(dir, prefix)
	}
}

func WithLogRetention(days int) Option {
	return func(conf *server.AppConf) {
		conf.Logger.LogFile.RetentionDays = &days
		for _, shelfConf := range conf.Shelves {
			shelfConf.Logger.LogFile.RetentionDays = &days
		}
	}
}

func WithReadOnlyShelf() Option {
	return func(conf *server.AppConf) {
		conf.Shelves[0].ReadOnly = true
	}
}

func WithReadOnlyServer() Option {
	return func(conf *server.AppConf) {
		conf.ReadOnly = true
	}
}

func WithBookCacheInterval(interval string) Option {
	return func(conf *server.AppConf) {
		conf.Shelves[0].BookCacheInterval = interval
	}
}

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

func AppConf(t *testing.T, opts ...Option) *server.AppConf {
	t.Helper()

	conf := &server.AppConf{
		Shelves: []*shelf.ShelfConfWithID{
			{
				ID:                DefaultShelfID,
				LibRoot:           t.TempDir(),
				BookCacheInterval: "1s",
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
	LibRoot string
}

func New(t *testing.T, opts ...Option) *Env {
	t.Helper()

	env := NewUnstarted(t, opts...)

	if err := env.App.Start(); err != nil {
		t.Fatalf("Start app: %v", err)
	}

	waitForInitialShelfScans(t, env.App)

	return env
}

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

func waitForInitialShelfScans(t *testing.T, app *server.App) {
	t.Helper()

	for _, shelfData := range app.ShelfManager().GetAllShelves() {
		if err := shelfData.WaitReady(t.Context()); err != nil {
			t.Fatalf("WaitReady for shelf %s: %v", shelfData.ID, err)
		}
	}
}

func (env *Env) SetReadOnly(t *testing.T, readOnly bool) {
	t.Helper()

	previous := env.App.Conf().ReadOnly
	env.App.Conf().ReadOnly = readOnly
	t.Cleanup(func() { env.App.Conf().ReadOnly = previous })
}

func (env *Env) Get(url string) *httptest.ResponseRecorder {
	return env.Request(http.MethodGet, url, nil)
}

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

func (env *Env) PutContent(url, contentType string, body io.Reader) *httptest.ResponseRecorder {
	return env.RequestTyped(http.MethodPut, url, contentType, body)
}

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

func (env *Env) Request(method, url string, body io.Reader) *httptest.ResponseRecorder {
	return env.Do(httptest.NewRequest(method, url, body))
}

func (env *Env) Do(req *http.Request) *httptest.ResponseRecorder {
	return env.DoRaw(env.withToken(req))
}

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

func (env *Env) GetWailsLike(url string) *wailsLikeRecorder {
	rec := newWailsLikeRecorder()
	env.handler.ServeHTTP(rec, env.withToken(httptest.NewRequest(http.MethodGet, url, nil)))
	return rec
}

const SecondShelfID = "second_shelf"

func WithSecondShelf(libRoot string) Option {
	return func(conf *server.AppConf) {
		conf.Shelves = append(conf.Shelves, &shelf.ShelfConfWithID{
			ID:                SecondShelfID,
			LibRoot:           libRoot,
			BookCacheInterval: "1s",
		})
	}
}

func WithReadOnlySecondShelf(libRoot string) Option {
	return func(conf *server.AppConf) {
		conf.Shelves = append(conf.Shelves, &shelf.ShelfConfWithID{
			ID:                SecondShelfID,
			LibRoot:           libRoot,
			BookCacheInterval: "1s",
			ReadOnly:          true,
		})
	}
}

func LocalTokenSecurity() *server.SecurityConf {
	return &server.SecurityConf{
		Mode:                        server.SecurityModeLocalToken,
		AllowMissingOriginWithToken: new(true),
		AllowedOrigins:              []string{"http://localhost:20000"},
	}
}
