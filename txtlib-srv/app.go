package txtlibsrv

import (
	"io/fs"
	"net/http"
	"strings"

	"github.com/voilelab/plainshelf/internal/util"
	"github.com/voilelab/plainshelf/txtlib"
	txtlibfrontend "github.com/voilelab/plainshelf/txtlib-frontend"
	"github.com/voilelab/plainshelf/txtlib-srv/bookmark"
)

type App struct {
	lib        *txtlib.Lib
	markLib    *bookmark.DB
	spaFS      fs.FS
	spaHandler http.Handler

	conf *AppConf
}

type AppConf struct {
	LibPath    string `yaml:"lib_path"`
	MarkPath   string `yaml:"mark_path"`
	CoverToJPG bool   `yaml:"cover_to_jpg"`
}

func NewApp(conf *AppConf) (*App, error) {
	lib, err := txtlib.OpenLocalLib(conf.LibPath)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	markDB, err := bookmark.New(conf.MarkPath)
	if err != nil {
		lib.Close()
		return nil, util.Errorf("%w", err)
	}

	return &App{
		lib:        lib,
		markLib:    markDB,
		spaFS:      txtlibfrontend.WebFS,
		spaHandler: http.FileServerFS(txtlibfrontend.WebFS),
		conf:       conf,
	}, nil
}

func (app *App) Start() error {
	return nil
}

func (app *App) Close() error {
	// TBD: aggregate errors if both fail
	app.markLib.Close()
	return app.lib.Close()
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

	mux.HandleFunc("GET /api/marks/{book_id}", app.HandleAPIGetMarks)
	mux.HandleFunc("POST /api/marks/{book_id}", app.HandleAPIUpdateMarks)

	mux.HandleFunc("GET /api/layers", app.HandleAPIGetLayers)
	mux.HandleFunc("POST /api/layers/{layer_path}", app.HandleAPICreateLayer)
	mux.HandleFunc("DELETE /api/layers/{layer_path}", app.HandleAPIDeleteLayer)

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
