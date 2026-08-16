package server

import (
	"errors"
	"net/http"

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
	taskChains   taskutil.Pool
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

	// Set to true to ensure that if any initialization step fails,
	// all previously initialized resources will be properly closed.
	failure := true

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

	for _, conf := range conf.Shelves {
		shelfConf := *conf
		// An operator who pins the ID in the config keeps it; everyone else gets
		// this installation's generated one.
		if shelfConf.BookCacheWriterID == "" {
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
	app.handlers = newAPIHandlers(&app.Logger, shelfManager, security, storeDB, taskChains, frontend.WebFS, conf)

	return app, nil
}

func (app *App) Start() error {
	app.taskChains.Start()
	return nil
}

func (app *App) Conf() *AppConf {
	return app.conf
}

func (app *App) ShelfManager() *shelf.ShelfManager {
	return app.shelfManager
}

func (app *App) TaskChains() taskutil.Pool {
	return app.taskChains
}

// AddShelf opens a shelf after startup — the desktop app's "add shelf" flow.
//
// The writer ID has to be applied here as well as in NewApp: a shelf added this
// way otherwise exports nothing until the app is restarted, and its manual
// export fails.
func (app *App) AddShelf(conf shelf.ShelfConfWithID) error {
	if conf.BookCacheWriterID == "" {
		conf.BookCacheWriterID = app.bookCacheWriterID
	}
	return app.shelfManager.AddShelf(conf)
}

func (app *App) UpdateShelf(id, name, scanInterval string) error {
	return app.shelfManager.UpdateShelf(id, name, scanInterval)
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
		app.Info("app handler", "method", r.Method, "path", r.URL.Path, "remote_addr", r.RemoteAddr)
		if app.rejectReadOnlyWrite(w, r) {
			return
		}
		mux.ServeHTTP(w, r)
	})

	return app.security.Middleware(loggerHandler)
}

// ImportFromLocalPath imports a book the desktop client picked from disk,
// without going through an upload.
func (app *App) ImportFromLocalPath(shelfID string, localPath string, layerParts shelf.Layers) (*shelf.Book, error) {
	return app.handlers.imports.fromLocalPath(shelfID, localPath, layerParts)
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

	http.Error(w, "server is in read-only mode", http.StatusForbidden)
	return true
}
