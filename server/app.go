package server

import (
	"errors"
	"net/http"
	"strings"

	"github.com/voilelab/plainshelf/frontend"
	"github.com/voilelab/plainshelf/internal/logutil"
	"github.com/voilelab/plainshelf/internal/taskutil"
	"github.com/voilelab/plainshelf/internal/util"
	"github.com/voilelab/plainshelf/server/store"
	"github.com/voilelab/plainshelf/shelf"
)

// App owns the server's resources - the shelves, the store, the worker pool -
// and starts and stops them. Answering requests belongs to handlers, which
// App assembles once and then only routes through.
type App struct {
	logutil.Logger

	handlers *apiHandlers

	shelfManager *shelf.ShelfManager
	taskChains   *taskutil.Pool
	storeDB      *store.DB

	// bookCacheWriterID names this installation in the book cache every shelf
	// exports; see book_cache_writer.go. Held so shelves opened after startup
	// get it too.
	bookCacheWriterID string

	conf     *AppConf
	security *Security
}

func NewApp(conf *AppConf) (*App, error) {
	if conf == nil {
		return nil, util.Errorf("config cannot be nil")
	}

	security, err := NewSecurity(conf.Security)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	// Cleared on success; until then a failed step closes what is already open.
	failure := true

	// Every logger below shares one retention window so the setting route can
	// change all of them at once. Attached before the first logger is built, and
	// filled in as soon as the store is open — before anything writes a line.
	shareLogRetention(conf, logutil.NewRetention())

	logger, err := logutil.NewLogger(&conf.Logger)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}
	defer func() {
		if failure {
			logger.Close()
		}
	}()
	// Opened before the shelves because each shelf is configured with the book
	// cache writer ID this store holds, and a shelf starts scanning — and
	// exporting — the moment it is created.
	storeDB, err := store.New(conf.StorePath)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}
	defer func() {
		if failure {
			if closeErr := storeDB.Close(); closeErr != nil {
				logger.Error("failed to close store after failed startup", "error", closeErr)
			}
		}
	}()

	settingsSvc := &settings{Logger: logger, db: storeDB, conf: conf}
	settingsSvc.applyLogRetention()

	writerID, err := resolveBookCacheWriterID(storeDB)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	shelfManager := shelf.NewShelfManager()
	defer func() {
		if failure {
			shelfManager.Close()
		}
	}()

	for _, shelfEntry := range conf.Shelves {
		shelfConf := applyAppReadOnly(*shelfEntry, conf.ReadOnly)
		// An operator who pins the ID in the config keeps it; everyone else gets
		// this installation's generated one.
		if shelfConf.BookCacheWriterID == "" && !shelfConf.ReadOnly {
			shelfConf.BookCacheWriterID = writerID
		}
		if err := shelfManager.AddShelf(shelfConf); err != nil {
			return nil, util.Errorf("%w", err)
		}
	}

	// The worker section is optional; every field has a usable zero value.
	workerConf := conf.Worker
	if workerConf == nil {
		workerConf = &WorkerConf{}
	}

	workLogger, err := logutil.NewLogger(&workerConf.Logger)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}
	defer func() {
		if failure {
			workLogger.Close()
		}
	}()

	taskChains := taskutil.NewPool(taskutil.NewWorker(workerConf.MaxLen, workLogger), workerConf.MaxKeep)

	failure = false
	app := &App{
		Logger:       *logger,
		shelfManager: shelfManager,
		taskChains:   taskChains,
		storeDB:      storeDB,
		conf:         conf,
		security:     security,

		bookCacheWriterID: writerID,
	}

	// Assembled after App so the handlers share its logger rather than opening
	// one of their own.
	app.handlers = newAPIHandlers(&app.Logger, shelfManager, security, storeDB, taskChains, frontend.WebFS, conf, settingsSvc)

	return app, nil
}

func (app *App) Start() error {
	app.taskChains.Start()
	return nil
}

func (app *App) Conf() *AppConf {
	return app.conf
}

// SetInsecureNetworkWarning makes the SPA bootstrap surface a persistent
// "API authentication is disabled" warning in the Web UI. Only the network
// server path sets it, computed from the listen address (see
// Security.InsecureNetworkExposure); in-process embedders such as the desktop
// and reader apps open no port and never call it.
func (app *App) SetInsecureNetworkWarning(v bool) {
	if app == nil || app.handlers == nil || app.handlers.spa == nil {
		return
	}
	app.handlers.spa.warnInsecurePublic = v
}

func (app *App) ShelfManager() *shelf.ShelfManager {
	return app.shelfManager
}

// TaskChains lets a test submit a chain directly rather than through whichever
// HTTP route happens to start one. No production caller needs it; it is exported
// only because the contract tests live in external packages under
// server/contract, which an export_test.go here cannot reach.
func (app *App) TaskChains() *taskutil.Pool {
	return app.taskChains
}

// AddShelf opens a shelf after startup — the desktop app's "add shelf" flow.
//
// The writer ID has to be applied here as well as in NewApp: a shelf added this
// way otherwise exports nothing until the app is restarted, and its manual
// export fails. Read-only mode has to be applied here for the same reason, and
// it is what withholds the writer ID rather than granting it.
func (app *App) AddShelf(conf shelf.ShelfConfWithID) error {
	return app.shelfManager.AddShelf(app.resolveShelfConf(conf))
}

// resolveShelfConf finishes a shelf configuration the app was handed: it folds
// in the app-wide read-only mode, the log retention window every logger shares
// and, unless the shelf is read-only, this installation's book cache writer ID.
func (app *App) resolveShelfConf(conf shelf.ShelfConfWithID) shelf.ShelfConfWithID {
	shelfConf := applyAppReadOnly(conf, app.conf.ReadOnly)
	if shelfConf.BookCacheWriterID == "" && !shelfConf.ReadOnly {
		shelfConf.BookCacheWriterID = app.bookCacheWriterID
	}
	// A shelf opened or reconfigured after startup writes its own log file, so
	// it joins the retention window the setting controls rather than keeping
	// the configured default.
	shelfConf.Logger.LogFile.Retention = app.conf.Logger.LogFile.Retention
	return shelfConf
}

// applyAppReadOnly carries AppConf.ReadOnly down into the shelf configuration.
//
// rejectReadOnlyWrite only turns away requests, which is not the same as not
// writing: a shelf creates its folders, clears app/tmp/, takes the lock file and
// exports the book cache on a timer with no request behind any of it — and that
// export would prune the files other installations wrote into a shared shelf.
//
// The app-wide setting can only add the restriction; a shelf already configured
// read_only stays read-only on a writable server.
func applyAppReadOnly(conf shelf.ShelfConfWithID, appReadOnly bool) shelf.ShelfConfWithID {
	if appReadOnly {
		conf.ReadOnly = true
	}
	return conf
}

// UpdateShelf reconfigures an open shelf. conf carries the shelf's whole
// configuration, not only what changed, so it goes through the same resolution
// as AddShelf - a shelf that stops being read-only has to be given the writer
// ID that read-only mode withheld from it, and one that becomes read-only has
// to give it up again.
func (app *App) UpdateShelf(conf shelf.ShelfConfWithID) error {
	return app.shelfManager.UpdateShelf(app.resolveShelfConf(conf))
}

func (app *App) RemoveShelf(id string) error {
	return app.shelfManager.RemoveShelf(id)
}

func (app *App) Close() error {
	err1 := app.storeDB.Close()
	err2 := app.shelfManager.Close()
	err3 := app.Logger.Close()
	err4 := app.taskChains.Close()

	err := errors.Join(err1, err2, err3, err4)
	if err != nil {
		return util.Errorf("%w", err)
	}
	return nil
}

func (app *App) Handler() http.Handler {
	mux := http.NewServeMux()
	app.handlers.serve(mux)

	loggerHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// One ID per request, minted before anything can answer it, so the
		// response header, every log line about the request and the error
		// envelope all quote the same string.
		requestID := logutil.NewRequestID()
		w.Header().Set(RequestIDHeader, requestID)
		r = r.WithContext(logutil.WithRequestID(r.Context(), requestID))

		app.Info("app handler", "request_id", requestID, "method", r.Method, "path", r.URL.Path, "remote_addr", r.RemoteAddr)
		if app.rejectReadOnlyWrite(w, r) {
			return
		}
		mux.ServeHTTP(w, r)
	})

	return app.security.Middleware(loggerHandler)
}

// ImportFromLocalPath imports a book the desktop client picked from disk,
// without going through an upload.
func (app *App) ImportFromLocalPath(shelfID string, localPath string, folderParts shelf.FolderPath) (*shelf.Book, error) {
	return app.handlers.imports.fromLocalPath(shelfID, localPath, folderParts)
}

// GetBookFolderPath locates a book on disk for the desktop client's "show in
// file manager" action.
func (app *App) GetBookFolderPath(shelfID, bookID string) (string, error) {
	return app.handlers.books.folderPath(shelfID, bookID)
}

func (app *App) SecurityToken() string {
	return app.security.Token()
}

func (app *App) SecurityTokenHeader() string {
	return app.security.TokenHeader()
}

func (app *App) rejectReadOnlyWrite(w http.ResponseWriter, r *http.Request) bool {
	if app == nil || app.conf == nil || !app.conf.ReadOnly {
		return false
	}

	if !IsMutatingMethod(r.Method) {
		return false
	}

	if isReadOnlySafeRequest(r) {
		return false
	}

	http.Error(w, "server is in read-only mode", http.StatusForbidden)
	return true
}

// isReadOnlySafeRequest reports a POST that writes nothing to the shelf.
//
// The rescan endpoint is the only one: it walks the shelf and rebuilds the
// in-memory cache, which is what a read does. A named exception rather than a
// general "reads may POST" rule, so adding a second one has to be written here.
//
// The token gate draws the same exception, for the same reason, in
// Security.isTokenExemptScan: a rescan reads, so protect_read governs it rather
// than its method. The two gates stay separate -- this one answers "may the
// shelf change", that one "who is asking" -- but they agree on which requests
// are reads.
func isReadOnlySafeRequest(r *http.Request) bool {
	return r.Method == http.MethodPost && isShelfScanPath(r.URL.Path)
}

// isShelfScanPath matches /api/shelves/{shelf_id}/scans. The gate runs before
// routing, so the pattern the mux will apply is not available here and the path
// is taken apart by hand.
func isShelfScanPath(urlPath string) bool {
	rest, ok := strings.CutPrefix(urlPath, "/api/shelves/")
	if !ok {
		return false
	}

	shelfID, tail, ok := strings.Cut(rest, "/")
	return ok && shelfID != "" && tail == "scans"
}
