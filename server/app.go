package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
	"strings"

	"github.com/voilelab/plainshelf/frontend"
	"github.com/voilelab/plainshelf/internal/logutil"
	"github.com/voilelab/plainshelf/internal/util"
	"github.com/voilelab/plainshelf/server/store"
	"github.com/voilelab/plainshelf/shelf"
)

type App struct {
	logutil.Logger

	shelf      *shelf.Shelf
	storeDB    *store.DB
	spaFS      fs.FS
	spaHandler http.Handler

	conf     *AppConf
	security *Security
}

type AppConf struct {
	Logger           logutil.LogConf  `yaml:"logger"`
	Shelf            *shelf.ShelfConf `yaml:"shelf"`
	StorePath        string           `yaml:"store_path"`
	CoverToJPG       bool             `yaml:"cover_to_jpg"`
	ReadHistoryLimit int              `yaml:"read_history_limit"`
	Security         *SecurityConf    `yaml:"security"`
}

func NewApp(conf *AppConf) (*App, error) {
	logger, err := logutil.NewLogger(&conf.Logger)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	s, err := shelf.NewShelf(conf.Shelf)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	security, err := NewSecurity(conf.Security)
	if err != nil {
		s.Close()
		return nil, util.Errorf("%w", err)
	}

	storeDB, err := store.New(conf.StorePath, conf.ReadHistoryLimit)
	if err != nil {
		s.Close()
		return nil, util.Errorf("%w", err)
	}

	return &App{
		Logger:     *logger,
		shelf:      s,
		storeDB:    storeDB,
		spaFS:      frontend.WebFS,
		spaHandler: http.FileServerFS(frontend.WebFS),
		conf:       conf,
		security:   security,
	}, nil
}

func (app *App) Start() error {
	return nil
}

func (app *App) Close() error {
	err1 := app.storeDB.Close()
	err2 := app.shelf.Close()
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
	return app.security.Middleware(mux)
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

	// Book API

	mux.HandleFunc("GET /api/books", app.HandleAPIGetBooks)

	mux.HandleFunc("POST /api/books/import", app.HandleAPIImportBook)
	mux.HandleFunc("GET /api/books/duplicate", app.HandleAPIFindDuplicateBooks)

	mux.HandleFunc("GET /api/books/{book_id}", app.HandleAPIGetBook)
	mux.HandleFunc("PATCH /api/books/{book_id}", app.HandleAPIUpdateBook)
	mux.HandleFunc("DELETE /api/books/{book_id}", app.HandleAPIDeleteBook)

	mux.HandleFunc("GET /api/books/{book_id}/sources", app.HandleAPIGetBookSources)
	mux.HandleFunc("GET /api/books/{book_id}/sources/{source_id}", app.HandleAPIGetBookSource)
	mux.HandleFunc("GET /api/books/{book_id}/sources/{source_id}/content", app.HandleAPIGetBookSourceContent)
	mux.HandleFunc("PATCH /api/books/{book_id}/sources/{source_id}/content", app.HandleAPIUpdateBookSourceContent)

	mux.HandleFunc("GET /api/books/{book_id}/cover", app.HandleAPIGetBookCover)
	mux.HandleFunc("PUT /api/books/{book_id}/cover", app.HandleAPIUpdateBookCover)
	mux.HandleFunc("DELETE /api/books/{book_id}/cover", app.HandleAPIDeleteBookCover)

	mux.HandleFunc("GET /api/books/{book_id}/content", app.HandleAPIGetBookContent)
	mux.HandleFunc("GET /api/books/{book_id}/split_config", app.HandleAPIGetBookSplitConfig)
	mux.HandleFunc("PATCH /api/books/{book_id}/split_config", app.HandleAPIUpdateBookSplitConfig)

	mux.HandleFunc("GET /api/layers", app.HandleAPIGetLayers)
	mux.HandleFunc("POST /api/layers/{layer_path...}", app.HandleAPICreateLayer)
	mux.HandleFunc("DELETE /api/layers/{layer_path...}", app.HandleAPIDeleteLayer)

	// Store API

	mux.HandleFunc("GET /api/marks/{book_id}", app.HandleAPIGetMarks)
	mux.HandleFunc("POST /api/marks/{book_id}", app.HandleAPIUpdateMarks)

	mux.HandleFunc("GET /api/read_history", app.HandleAPIGetReadHistory)
	mux.HandleFunc("POST /api/read_history", app.HandleAPIUpdateReadHistory)
	mux.HandleFunc("DELETE /api/read_history", app.HandleAPIClearReadHistory)

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
