package server

import (
	"io/fs"
	"net/http"
	"strings"

	"github.com/voilelab/plainshelf/frontend"
	"github.com/voilelab/plainshelf/internal/util"
	"github.com/voilelab/plainshelf/server/store"
	"github.com/voilelab/plainshelf/shelf"
)

type App struct {
	shelf      *shelf.Shelf
	storeDB    *store.DB
	spaFS      fs.FS
	spaHandler http.Handler

	conf *AppConf
}

type AppConf struct {
	ShelfPath        string `yaml:"shelf_path"`
	StorePath        string `yaml:"store_path"`
	CoverToJPG       bool   `yaml:"cover_to_jpg"`
	ReadHistoryLimit int    `yaml:"read_history_limit"`
}

func NewApp(conf *AppConf) (*App, error) {
	s, err := shelf.OpenLocalShelf(conf.ShelfPath)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	storeDB, err := store.New(conf.StorePath, conf.ReadHistoryLimit)
	if err != nil {
		s.Close()
		return nil, util.Errorf("%w", err)
	}

	return &App{
		shelf:      s,
		storeDB:    storeDB,
		spaFS:      frontend.WebFS,
		spaHandler: http.FileServerFS(frontend.WebFS),
		conf:       conf,
	}, nil
}

func (app *App) Start() error {
	return nil
}

func (app *App) Close() error {
	// TBD: aggregate errors if both fail
	app.storeDB.Close()
	return app.shelf.Close()
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
		w.Write(data)
		return
	}

	app.spaHandler.ServeHTTP(w, r)
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

	mux.HandleFunc("GET /api/books/{book_id}/snapshots", app.HandleAPIGetBookSnapshots)
	mux.HandleFunc("GET /api/books/{book_id}/snapshots/{snapshot_id}/content", app.HandleAPIGetBookSnapshotContent)
	mux.HandleFunc("PATCH /api/books/{book_id}/snapshots/{snapshot_id}/content", app.HandleAPIUpdateBookSnapshotContent)

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
