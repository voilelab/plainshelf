package server

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/voilelab/plainshelf/frontend"
	"github.com/voilelab/plainshelf/internal/fsutil"
	"github.com/voilelab/plainshelf/internal/logutil"
	"github.com/voilelab/plainshelf/internal/taskutil"
	"github.com/voilelab/plainshelf/internal/util"
	"github.com/voilelab/plainshelf/server/store"
	"github.com/voilelab/plainshelf/shelf"
)

type App struct {
	logutil.Logger

	handlers *apiHandlers

	shelfManager *shelf.ShelfManager
	taskChains   *taskutil.Pool
	storeDB      *store.DB

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

	failure := true

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

		if shelfConf.BookCacheWriterID == "" && !shelfConf.ReadOnly {
			shelfConf.BookCacheWriterID = writerID
		}
		if err := addShelfEntry(shelfManager, shelfConf, conf.StorePath, conf.Credentials); err != nil {
			return nil, util.Errorf("%w", err)
		}
	}

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

func (app *App) SetInsecureNetworkWarning(v bool) {
	if app == nil || app.handlers == nil || app.handlers.spa == nil {
		return
	}
	app.handlers.spa.warnInsecurePublic = v
}

func (app *App) ShelfManager() *shelf.ShelfManager {
	return app.shelfManager
}

func (app *App) TaskChains() *taskutil.Pool {
	return app.taskChains
}

func (app *App) AddShelf(conf shelf.ShelfConfWithID) error {
	return addShelfEntry(app.shelfManager, app.resolveShelfConf(conf), app.conf.StorePath, app.conf.Credentials)
}

func (app *App) AddVirtualShelf(conf shelf.ShelfConfWithID, root fsutil.ReadFS, scope string) error {
	if scope == "" {
		return util.NewError("virtual shelf scope cannot be empty")
	}

	conf.ReadOnly = true
	return app.shelfManager.AddVirtualShelf(app.resolveShelfConf(conf), root, offlineDir(app.conf.StorePath, scope))
}

func (app *App) SetShelfShowNSFW(shelfID string, show bool) {
	app.handlers.core.showNSFWByShelf.Store(shelfID, show)
}

func (app *App) ClearShelfShowNSFW(shelfID string) {
	app.handlers.core.showNSFWByShelf.Delete(shelfID)
}

func offlineDir(storePath, scope string) string {
	sum := sha256.Sum256([]byte(scope))
	return filepath.Join(storePath, "offline", hex.EncodeToString(sum[:16]))
}

func (app *App) RemoveOfflineScope(scope string) error {
	if scope == "" {
		return util.NewError("virtual shelf scope cannot be empty")
	}
	return os.RemoveAll(offlineDir(app.conf.StorePath, scope))
}

func (app *App) resolveShelfConf(conf shelf.ShelfConfWithID) shelf.ShelfConfWithID {
	shelfConf := applyAppReadOnly(conf, app.conf.ReadOnly)
	if shelfConf.BookCacheWriterID == "" && !shelfConf.ReadOnly {
		shelfConf.BookCacheWriterID = app.bookCacheWriterID
	}

	shelfConf.Logger.LogFile.Retention = app.conf.Logger.LogFile.Retention
	return shelfConf
}

func applyAppReadOnly(conf shelf.ShelfConfWithID, appReadOnly bool) shelf.ShelfConfWithID {
	if appReadOnly {
		conf.ReadOnly = true
	}
	return conf
}

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

func (app *App) ImportFromLocalPath(shelfID string, localPath string, folderParts shelf.FolderPath) (*shelf.Book, error) {
	return app.handlers.imports.fromLocalPath(shelfID, localPath, folderParts)
}

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

func isReadOnlySafeRequest(r *http.Request) bool {
	return isShelfScanRequest(r) || isDownloadRequest(r)
}

func isShelfScanRequest(r *http.Request) bool {
	return r.Method == http.MethodPost && isShelfScanPath(r.URL.Path)
}

func isDownloadRequest(r *http.Request) bool {
	if r.Method != http.MethodPut && r.Method != http.MethodDelete {
		return false
	}
	rest, ok := strings.CutPrefix(r.URL.Path, "/api/shelves/")
	if !ok {
		return false
	}
	parts := strings.Split(rest, "/")
	return len(parts) == 4 && parts[0] != "" && parts[1] == "books" && parts[2] != "" && parts[3] == "download"
}

func isShelfScanPath(urlPath string) bool {
	rest, ok := strings.CutPrefix(urlPath, "/api/shelves/")
	if !ok {
		return false
	}

	shelfID, tail, ok := strings.Cut(rest, "/")
	return ok && shelfID != "" && tail == "scans"
}
