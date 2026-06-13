package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
	"slices"
	"strings"

	"github.com/voilelab/plainshelf/frontend"
	"github.com/voilelab/plainshelf/internal/logutil"
	"github.com/voilelab/plainshelf/internal/util"
	"github.com/voilelab/plainshelf/server/store"
)

type App struct {
	logutil.Logger

	shelfManager *ShelfManager
	storeDB      *store.DB
	spaFS        fs.FS
	spaHandler   http.Handler

	conf     *AppConf
	security *Security
}

type AppConf struct {
	Logger           logutil.LogConf    `yaml:"logger"`
	Shelves          []*ShelfConfWithID `yaml:"shelves"`
	StorePath        string             `yaml:"store_path"`
	CoverToJPG       bool               `yaml:"cover_to_jpg"`
	ReadHistoryLimit int                `yaml:"read_history_limit"`
	Security         *SecurityConf      `yaml:"security"`
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

	if len(conf.Shelves) == 0 {
		return nil, util.Errorf("at least one shelf must be configured")
	}

	shelfManager := NewShelfManager()
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

	failure = false
	return &App{
		Logger:       *logger,
		shelfManager: shelfManager,
		storeDB:      storeDB,
		spaFS:        frontend.WebFS,
		spaHandler:   http.FileServerFS(frontend.WebFS),
		conf:         conf,
		security:     security,
	}, nil
}

func (app *App) Start() error {
	return nil
}

// GetShelf returns a registered shelf by ID for in-process callers.
func (app *App) GetShelf(id string) (*ShelfData, bool) {
	return app.shelfManager.GetShelf(id)
}

// AddShelf registers a shelf in the running app without exposing shelf mutation
// through the public HTTP API. It is intended for in-process desktop usage.
func (app *App) AddShelf(conf ShelfConfWithID) error {
	if err := app.shelfManager.AddShelf(conf); err != nil {
		return util.Errorf("%w", err)
	}
	app.conf.Shelves = append(app.conf.Shelves, &conf)
	return nil
}

// RemoveShelf unregisters a shelf from the running app. It is intended for
// in-process rollback paths, not as a public HTTP capability.
func (app *App) RemoveShelf(id string) error {
	if err := app.shelfManager.RemoveShelf(id); err != nil {
		return util.Errorf("%w", err)
	}
	app.conf.Shelves = slices.DeleteFunc(app.conf.Shelves, func(conf *ShelfConfWithID) bool {
		return conf != nil && conf.ID == id
	})
	return nil
}

func (app *App) Close() error {
	err1 := app.storeDB.Close()
	err2 := app.shelfManager.Close()
	err3 := app.Logger.Close()

	err := errors.Join(err1, err2, err3)
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

	mux.HandleFunc("GET /api/shelves", app.HandleGetShelves)

	// Book API

	mux.HandleFunc("GET /api/shelves/{shelf_id}/books", app.HandleAPIGetBooks)
	mux.HandleFunc("POST /api/shelves/{shelf_id}/books", app.HandleAPICreateBook)

	mux.HandleFunc("POST /api/shelves/{shelf_id}/books/import", app.HandleAPIImportBook)
	mux.HandleFunc("GET /api/shelves/{shelf_id}/books/duplicate", app.HandleAPIFindDuplicateBooks)

	mux.HandleFunc("GET /api/shelves/{shelf_id}/books/{book_id}", app.HandleAPIGetBook)
	mux.HandleFunc("PATCH /api/shelves/{shelf_id}/books/{book_id}", app.HandleAPIUpdateBook)
	mux.HandleFunc("DELETE /api/shelves/{shelf_id}/books/{book_id}", app.HandleAPIDeleteBook)
	mux.HandleFunc("POST /api/shelves/{shelf_id}/books/{book_id}/trash", app.HandleAPITrashBook)

	mux.HandleFunc("GET /api/shelves/{shelf_id}/books/{book_id}/sources", app.HandleAPIGetBookSources)
	mux.HandleFunc("GET /api/shelves/{shelf_id}/books/{book_id}/sources/{source_id}", app.HandleAPIGetBookSource)
	mux.HandleFunc("POST /api/shelves/{shelf_id}/books/{book_id}/sources", app.HandleAPICreateBookSource)
	mux.HandleFunc("DELETE /api/shelves/{shelf_id}/books/{book_id}/sources/{source_id}", app.HandleAPIDeleteBookSource)
	mux.HandleFunc("PUT /api/shelves/{shelf_id}/books/{book_id}/sources/{source_id}/current", app.HandleAPISetCurrentBookSource)
	mux.HandleFunc("GET /api/shelves/{shelf_id}/books/{book_id}/sources/{source_id}/content", app.HandleAPIGetBookSourceContent)
	mux.HandleFunc("PATCH /api/shelves/{shelf_id}/books/{book_id}/sources/{source_id}/content", app.HandleAPIUpdateBookSourceContent)

	mux.HandleFunc("GET /api/shelves/{shelf_id}/books/{book_id}/cover", app.HandleAPIGetBookCover)
	mux.HandleFunc("PUT /api/shelves/{shelf_id}/books/{book_id}/cover", app.HandleAPIUpdateBookCover)
	mux.HandleFunc("DELETE /api/shelves/{shelf_id}/books/{book_id}/cover", app.HandleAPIDeleteBookCover)

	mux.HandleFunc("GET /api/shelves/{shelf_id}/books/{book_id}/content", app.HandleAPIGetBookContent)
	mux.HandleFunc("GET /api/shelves/{shelf_id}/books/{book_id}/split_config", app.HandleAPIGetBookSplitConfig)
	mux.HandleFunc("PATCH /api/shelves/{shelf_id}/books/{book_id}/split_config", app.HandleAPIUpdateBookSplitConfig)

	mux.HandleFunc("GET /api/shelves/{shelf_id}/trash/books", app.HandleAPIGetTrashedBooks)
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

	mux.HandleFunc("GET /api/shelves/{shelf_id}/read_history", app.HandleAPIGetReadHistory)
	mux.HandleFunc("POST /api/shelves/{shelf_id}/read_history", app.HandleAPIUpdateReadHistory)
	mux.HandleFunc("DELETE /api/shelves/{shelf_id}/read_history", app.HandleAPIClearReadHistory)

	// Log API

	mux.HandleFunc("GET /api/logs", app.HandleAPIGetLogs)
	mux.HandleFunc("GET /api/logs/{log_id}/content", app.HandleAPIGetLogContent)

	// Setting API

	mux.HandleFunc("GET /api/setting/cover_to_jpg", app.HandleGetSettingCoverToJPG)
	mux.HandleFunc("POST /api/setting/cover_to_jpg", app.HandleSetSettingCoverToJPG)
	mux.HandleFunc("DELETE /api/setting/cover_to_jpg", app.HandleDeleteSettingCoverToJPG)
	mux.HandleFunc("GET /api/setting/read_history_limit", app.HandleGetSettingReadHistoryLimit)
	mux.HandleFunc("POST /api/setting/read_history_limit", app.HandleSetSettingReadHistoryLimit)
	mux.HandleFunc("DELETE /api/setting/read_history_limit", app.HandleDeleteSettingReadHistoryLimit)

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
