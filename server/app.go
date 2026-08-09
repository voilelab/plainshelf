package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
	"strings"

	"github.com/voilelab/plainshelf/frontend"
	"github.com/voilelab/plainshelf/internal/epub"
	"github.com/voilelab/plainshelf/internal/logutil"
	"github.com/voilelab/plainshelf/internal/taskutil"
	"github.com/voilelab/plainshelf/internal/util"
	"github.com/voilelab/plainshelf/server/store"
	"github.com/voilelab/plainshelf/shelf"
)

type App struct {
	logutil.Logger

	shelfManager *shelf.ShelfManager
	taskChains   taskutil.Pool
	storeDB      *store.DB
	spaFS        fs.FS
	spaHandler   http.Handler

	conf     *AppConf
	security *Security
}

type WorkerConf struct {
	Logger logutil.LogConf `yaml:"logger"`
	MaxLen int             `yaml:"max_len"`

	// MaxKeep bounds how many finished task chains stay queryable through the
	// task chain API. Zero selects the package default.
	MaxKeep int `yaml:"max_keep"`
}

type AppConf struct {
	Logger             logutil.LogConf          `yaml:"logger"`
	Shelves            []*shelf.ShelfConfWithID `yaml:"shelves"`
	Worker             *WorkerConf              `yaml:"worker"`
	StorePath          string                   `yaml:"store_path"`
	CoverToJPG         bool                     `yaml:"cover_to_jpg"`
	DefaultSplitConfig *shelf.SplitConfig       `yaml:"default_split_config"`
	EPUBImportStrategy *epub.Strategy           `yaml:"epub_import_strategy"`
	ReadOnly           bool                     `yaml:"read_only"`
	Security           *SecurityConf            `yaml:"security"`
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

	shelfManager := shelf.NewShelfManager()
	defer func() {
		if failure {
			shelfManager.Close()
		}
	}()

	for _, conf := range conf.Shelves {
		if err := shelfManager.AddShelf(*conf); err != nil {
			return nil, util.Errorf("%w", err)
		}
	}

	storeDB, err := store.New(conf.StorePath)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}
	defer func() {
		if failure {
			// Badger holds a lock on the store directory for as long as the
			// handle is open, so skipping this would leave a failed startup
			// blocking every later attempt to open the same store.
			if closeErr := storeDB.Close(); closeErr != nil {
				logger.Error("failed to close store after failed startup", "error", closeErr)
			}
		}
	}()

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
	return &App{
		Logger:       *logger,
		shelfManager: shelfManager,
		taskChains:   taskChains,
		storeDB:      storeDB,
		spaFS:        frontend.WebFS,
		spaHandler:   http.FileServerFS(frontend.WebFS),
		conf:         conf,
		security:     security,
	}, nil
}

func (app *App) Start() error {
	app.taskChains.Start()
	return nil
}

func (app *App) AddShelf(conf shelf.ShelfConfWithID) error {
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

func (app *App) Health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("1"))
}

// Handle SPA fallback for all non-API GET requests
func (app *App) HandleSPAFallback(w http.ResponseWriter, r *http.Request) {
	cleanPath := strings.TrimPrefix(r.URL.Path, "/")
	if cleanPath == "" || !hasFileExtension(cleanPath) {
		// SPA fallback: serve index.html for root and all non-file paths
		data, err := fs.ReadFile(app.spaFS, "index.html")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(app.injectSecurityBootstrap(data))
		return
	}

	app.spaHandler.ServeHTTP(w, r)
}

func (app *App) Handler() http.Handler {
	mux := http.NewServeMux()
	app.Serve(mux)

	loggerHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		app.Info("app handler", "method", r.Method, "path", r.URL.Path, "remote_addr", r.RemoteAddr)
		if app.rejectReadOnlyWrite(w, r) {
			return
		}
		mux.ServeHTTP(w, r)
	})

	return app.security.Middleware(loggerHandler)
}

func (app *App) SecurityToken() string {
	return app.security.Token()
}

func (app *App) SecurityTokenHeader() string {
	return app.security.TokenHeader()
}

func (app *App) injectSecurityBootstrap(data []byte) []byte {
	if app.security == nil || !app.security.IsEnabled() || app.security.Token() == "" {
		return data
	}
	token, err := json.Marshal(app.security.Token())
	if err != nil {
		return data
	}
	header, err := json.Marshal(app.security.TokenHeader())
	if err != nil {
		return data
	}
	bootstrap := []byte(`<script>window.__PLAINSHELF_SECURITY__={token:` + string(token) + `,tokenHeader:` + string(header) + `};</script>`)
	marker := []byte("</head>")
	if idx := bytes.Index(data, marker); idx >= 0 {
		out := make([]byte, 0, len(data)+len(bootstrap))
		out = append(out, data[:idx]...)
		out = append(out, bootstrap...)
		out = append(out, data[idx:]...)
		return out
	}
	return append(bootstrap, data...)
}

func (app *App) Serve(mux *http.ServeMux) {
	mux.HandleFunc("GET /health", app.Health)

	// Shelf API

	mux.HandleFunc("GET /api/mode", app.HandleGetMode)
	mux.HandleFunc("GET /api/version", app.HandleGetVersion)
	mux.HandleFunc("GET /api/shelves", app.HandleGetShelves)
	mux.HandleFunc("GET /api/shelves/{shelf_id}/status", app.HandleAPIGetShelfStatus)

	// Book API

	mux.HandleFunc("GET /api/shelves/{shelf_id}/books", app.HandleAPIGetBooks)
	mux.HandleFunc("POST /api/shelves/{shelf_id}/books", app.HandleAPICreateBook)
	mux.HandleFunc("POST /api/shelves/{shelf_id}/book-batches", app.HandleAPIBookBatch)

	mux.HandleFunc("POST /api/shelves/{shelf_id}/books/import", app.HandleAPIImportBook)
	mux.HandleFunc("GET /api/shelves/{shelf_id}/books/duplicate", app.HandleAPIFindDuplicateBooks)

	mux.HandleFunc("GET /api/shelves/{shelf_id}/books/{book_id}", app.HandleAPIGetBook)
	mux.HandleFunc("PATCH /api/shelves/{shelf_id}/books/{book_id}", app.HandleAPIUpdateBook)
	mux.HandleFunc("DELETE /api/shelves/{shelf_id}/books/{book_id}", app.HandleAPITrashBook)
	mux.HandleFunc("POST /api/shelves/{shelf_id}/books/{book_id}/trash", app.HandleAPITrashBook)

	mux.HandleFunc("GET /api/shelves/{shelf_id}/books/{book_id}/sources", app.HandleAPIGetBookSources)
	mux.HandleFunc("GET /api/shelves/{shelf_id}/books/{book_id}/sources/{source_id}", app.HandleAPIGetBookSource)
	mux.HandleFunc("POST /api/shelves/{shelf_id}/books/{book_id}/sources", app.HandleAPICreateBookSource)
	mux.HandleFunc("DELETE /api/shelves/{shelf_id}/books/{book_id}/sources/{source_id}", app.HandleAPIDeleteBookSource)
	mux.HandleFunc("PUT /api/shelves/{shelf_id}/books/{book_id}/sources/{source_id}/current", app.HandleAPISetCurrentBookSource)
	mux.HandleFunc("GET /api/shelves/{shelf_id}/books/{book_id}/sources/{source_id}/content", app.HandleAPIGetBookSourceContent)
	mux.HandleFunc("PATCH /api/shelves/{shelf_id}/books/{book_id}/sources/{source_id}/content", app.HandleAPIUpdateBookSourceContent)
	mux.HandleFunc("POST /api/shelves/{shelf_id}/books/{book_id}/sources/{source_id}/refresh", app.HandleAPIRefreshBookSourceMeta)

	mux.HandleFunc("GET /api/shelves/{shelf_id}/books/{book_id}/cover", app.HandleAPIGetBookCover)
	mux.HandleFunc("PUT /api/shelves/{shelf_id}/books/{book_id}/cover", app.HandleAPIUpdateBookCover)
	mux.HandleFunc("DELETE /api/shelves/{shelf_id}/books/{book_id}/cover", app.HandleAPIDeleteBookCover)

	mux.HandleFunc("GET /api/shelves/{shelf_id}/books/{book_id}/content", app.HandleAPIGetBookContent)
	mux.HandleFunc("GET /api/shelves/{shelf_id}/books/{book_id}/split_config", app.HandleAPIGetBookSplitConfig)
	mux.HandleFunc("PATCH /api/shelves/{shelf_id}/books/{book_id}/split_config", app.HandleAPIUpdateBookSplitConfig)

	mux.HandleFunc("GET /api/shelves/{shelf_id}/trash/books", app.HandleAPIGetTrashedBooks)
	mux.HandleFunc("POST /api/shelves/{shelf_id}/trash/empty", app.HandleAPIEmptyTrash)
	mux.HandleFunc("POST /api/shelves/{shelf_id}/trash/books/{book_id}/restore", app.HandleAPIRestoreTrashedBook)
	mux.HandleFunc("DELETE /api/shelves/{shelf_id}/trash/books/{book_id}", app.HandleAPIDeleteTrashedBook)

	mux.HandleFunc("GET /api/shelves/{shelf_id}/layers", app.HandleAPIGetLayers)
	mux.HandleFunc("POST /api/shelves/{shelf_id}/layer-moves", app.HandleAPIMoveLayer)
	mux.HandleFunc("POST /api/shelves/{shelf_id}/layers/{layer_path...}", app.HandleAPICreateLayer)
	mux.HandleFunc("PATCH /api/shelves/{shelf_id}/layers/{layer_path...}", app.HandleAPIRenameLayer)
	mux.HandleFunc("DELETE /api/shelves/{shelf_id}/layers/{layer_path...}", app.HandleAPIDeleteLayer)

	// Store API

	mux.HandleFunc("GET /api/shelves/{shelf_id}/marks/{book_id}", app.HandleAPIGetMarks)
	mux.HandleFunc("POST /api/shelves/{shelf_id}/marks/{book_id}", app.HandleAPIUpdateMarks)

	// Task API

	mux.HandleFunc("GET /api/taskchains/{taskchain_id}", app.HandleAPIGetTaskChain)

	// Log API

	mux.HandleFunc("GET /api/logs", app.HandleAPIGetLogs)
	mux.HandleFunc("GET /api/logs/{log_id}/content", app.HandleAPIGetLogContent)

	// Setting API

	mux.HandleFunc("GET /api/setting/cover_to_jpg", app.HandleGetSettingCoverToJPG)
	mux.HandleFunc("POST /api/setting/cover_to_jpg", app.HandleSetSettingCoverToJPG)
	mux.HandleFunc("DELETE /api/setting/cover_to_jpg", app.HandleDeleteSettingCoverToJPG)
	mux.HandleFunc("GET /api/setting/default_split_config", app.HandleGetSettingDefaultSplitConfig)
	mux.HandleFunc("POST /api/setting/default_split_config", app.HandleSetSettingDefaultSplitConfig)
	mux.HandleFunc("DELETE /api/setting/default_split_config", app.HandleDeleteSettingDefaultSplitConfig)
	mux.HandleFunc("GET /api/setting/epub_import_strategy", app.HandleGetSettingEPUBImportStrategy)
	mux.HandleFunc("POST /api/setting/epub_import_strategy", app.HandleSetSettingEPUBImportStrategy)
	mux.HandleFunc("DELETE /api/setting/epub_import_strategy", app.HandleDeleteSettingEPUBImportStrategy)

	mux.HandleFunc("GET /{path...}", app.HandleSPAFallback)
}

func hasFileExtension(path string) bool {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' {
			return false
		}
		if path[i] == '.' {
			return true
		}
	}
	return false
}

func (app *App) rejectReadOnlyWrite(w http.ResponseWriter, r *http.Request) bool {
	if app == nil || app.conf == nil || !app.conf.ReadOnly {
		return false
	}

	switch r.Method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		http.Error(w, "server is in read-only mode", http.StatusForbidden)
		return true
	default:
		return false
	}
}
