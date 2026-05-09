package txtlibsrv

import (
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/voilelab/plainshelf/internal/util"
	"github.com/voilelab/plainshelf/txtlib"
	txtlibfrontend "github.com/voilelab/plainshelf/txtlib-frontend"
	"github.com/voilelab/plainshelf/txtlib-srv/bookindex"
	"github.com/voilelab/plainshelf/txtlib-srv/bookmark"
)

type App struct {
	lib        *txtlib.Txtlib
	markLib    *bookmark.DB
	indexLib   *bookindex.DB
	spaFS      fs.FS
	spaHandler http.Handler
}

type AppConf struct {
	LibPath   string `yaml:"lib_path"`
	MarkPath  string `yaml:"mark_path"`
	IndexPath string `yaml:"index_path"`
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

	indexLib, err := bookindex.New(conf.IndexPath)
	if err != nil {
		lib.Close()
		markDB.Close()
		return nil, util.Errorf("%w", err)
	}

	return &App{
		lib:        lib,
		markLib:    markDB,
		indexLib:   indexLib,
		spaFS:      txtlibfrontend.WebFS,
		spaHandler: http.FileServerFS(txtlibfrontend.WebFS),
	}, nil
}

func (app *App) Start() error {
	err := initIndexDBFromLib(app.indexLib, app.lib)
	if err != nil {
		return util.Errorf("%w", err)
	}
	return nil
}

func (app *App) Close() error {
	// TBD: aggregate errors if both fail
	app.markLib.Close()
	app.indexLib.Close()
	return app.lib.Close()
}

func (app *App) Health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("1"))
}

type Book struct {
	Meta  *txtlib.BookMeta `json:"meta"`
	Layer txtlib.Layers    `json:"layer"`
}

type UpdateBookRequest struct {
	Title    *string        `json:"title"`
	Authors  *[]string      `json:"authors"`
	Tags     *[]string      `json:"tags"`
	Language *string        `json:"language"`
	Comment  *string        `json:"comment"`
	Layer    *txtlib.Layers `json:"layer"`
	Layers   *txtlib.Layers `json:"layers"`
}

// GET /api/books
func (app *App) HandleAPIGetBooks(w http.ResponseWriter, r *http.Request) {
	books, err := app.lib.ListBooks()
	if err != nil {
		http.Error(w, "failed to list books", http.StatusInternalServerError)
		return
	}

	jsonBooks := make([]Book, len(books))
	for i, b := range books {
		jsonBooks[i] = Book{
			Meta:  b.GetMeta(),
			Layer: b.Layers(),
		}
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	// TBD: pagination?
	err = json.NewEncoder(w).Encode(jsonBooks)
	if err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
		return
	}
}

// GET /api/books/{book_id}
func (app *App) HandleAPIGetBook(w http.ResponseWriter, r *http.Request) {
	bookID, err := readBookID(r)
	if err != nil {
		http.Error(w, "invalid book_id", http.StatusBadRequest)
		return
	}
	book, err := app.lib.GetBook(bookID)
	if err != nil {
		if errors.Is(err, txtlib.ErrBookNotFound) {
			http.Error(w, "book not found", http.StatusNotFound)
			return
		}
		http.Error(w, "failed to get book", http.StatusInternalServerError)
		return
	}

	jsonBook := Book{
		Meta:  book.GetMeta(),
		Layer: book.Layers(),
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	err = json.NewEncoder(w).Encode(jsonBook)
	if err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
		return
	}
}

// PATCH /api/books/{book_id}
func (app *App) HandleAPIUpdateBook(w http.ResponseWriter, r *http.Request) {
	bookID, err := readBookID(r)
	if err != nil {
		http.Error(w, "invalid book_id", http.StatusBadRequest)
		return
	}

	var req UpdateBookRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	book, err := app.lib.GetBook(bookID)
	if err != nil {
		if errors.Is(err, txtlib.ErrBookNotFound) {
			http.Error(w, "book not found", http.StatusNotFound)
			return
		}
		http.Error(w, "failed to get book", http.StatusInternalServerError)
		return
	}

	targetLayers := req.Layer
	if targetLayers == nil {
		targetLayers = req.Layers
	}
	if targetLayers != nil {
		movedBook, err := app.lib.MoveBook(bookID, append(txtlib.Layers(nil), (*targetLayers)...))
		if err != nil {
			http.Error(w, "failed to move book layer", http.StatusInternalServerError)
			return
		}
		book = movedBook
	}

	meta := *book.GetMeta()
	if req.Title != nil {
		meta.Title = *req.Title
	}
	if req.Authors != nil {
		meta.Authors = append([]string(nil), (*req.Authors)...)
	}
	if req.Tags != nil {
		meta.Tags = append([]string(nil), (*req.Tags)...)
	}
	if req.Language != nil {
		meta.Language = *req.Language
	}
	if req.Comment != nil {
		meta.Comments = *req.Comment
	}
	meta.UpdatedAt = util.JSONTime(time.Now())

	if err := book.SetMeta(&meta); err != nil {
		http.Error(w, "failed to update book metadata", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	err = json.NewEncoder(w).Encode(Book{Meta: &meta, Layer: book.Layers()})
	if err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
		return
	}
}

// DELETE /api/books/{book_id}
func (app *App) HandleAPIDeleteBook(w http.ResponseWriter, r *http.Request) {
	bookID, err := readBookID(r)
	if err != nil {
		http.Error(w, "invalid book_id", http.StatusBadRequest)
		return
	}

	err = app.lib.DeleteBook(bookID)
	if err != nil {
		http.Error(w, "failed to delete book", http.StatusInternalServerError)
		return
	}

	app.indexLib.Remove(bookID)

	w.WriteHeader(http.StatusNoContent)
}

// GET /api/books/{book_id}/cover
func (app *App) HandleAPIGetBookCover(w http.ResponseWriter, r *http.Request) {
	bookID, err := readBookID(r)
	if err != nil {
		http.Error(w, "invalid book_id", http.StatusBadRequest)
		return
	}

	book, err := app.lib.GetBook(bookID)
	if err != nil {
		if errors.Is(err, txtlib.ErrBookNotFound) {
			http.Error(w, "book not found", http.StatusNotFound)
			return
		}
		http.Error(w, "failed to get book", http.StatusInternalServerError)
		return
	}

	coverData, ext, err := book.OpenCover()
	if err != nil {
		http.Error(w, "failed to get book cover", http.StatusInternalServerError)
		return
	}

	if coverData == nil {
		http.Error(w, "cover not found", http.StatusNotFound)
		return
	}

	contentType := "image/jpeg"
	switch ext {
	case ".png":
		contentType = "image/png"
	case ".jpg", ".jpeg":
		contentType = "image/jpeg"
	case ".gif":
		contentType = "image/gif"
	}

	w.Header().Set("Content-Type", contentType)
	w.Write(coverData)
}

// PUT /api/books/{book_id}/cover
func (app *App) HandleAPIUpdateBookCover(w http.ResponseWriter, r *http.Request) {
	bookID, err := readBookID(r)
	if err != nil {
		http.Error(w, "invalid book_id", http.StatusBadRequest)
		return
	}

	book, err := app.lib.GetBook(bookID)
	if err != nil {
		if errors.Is(err, txtlib.ErrBookNotFound) {
			http.Error(w, "book not found", http.StatusNotFound)
			return
		}
		http.Error(w, "failed to get book", http.StatusInternalServerError)
		return
	}

	contentType := r.Header.Get("Content-Type")
	var ext string
	switch contentType {
	case "image/png":
		ext = ".png"
	case "image/jpeg":
		ext = ".jpg"
	case "image/gif":
		ext = ".gif"
	default:
		http.Error(w, "unsupported content type", http.StatusBadRequest)
		return
	}

	data, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read request body", http.StatusInternalServerError)
		return
	}

	err = book.SetCover(data, ext)
	if err != nil {
		http.Error(w, "failed to update book cover", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// DELETE /api/books/{book_id}/cover
func (app *App) HandleAPIDeleteBookCover(w http.ResponseWriter, r *http.Request) {
	bookID, err := readBookID(r)
	if err != nil {
		http.Error(w, "invalid book_id", http.StatusBadRequest)
		return
	}

	book, err := app.lib.GetBook(bookID)
	if err != nil {
		if errors.Is(err, txtlib.ErrBookNotFound) {
			http.Error(w, "book not found", http.StatusNotFound)
			return
		}
		http.Error(w, "failed to get book", http.StatusInternalServerError)
		return
	}

	err = book.DeleteCover()
	if err != nil {
		http.Error(w, "failed to delete book cover", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// GET /api/books/{book_id}/content
func (app *App) HandleAPIGetBookContent(w http.ResponseWriter, r *http.Request) {
	bookID, err := readBookID(r)
	if err != nil {
		http.Error(w, "invalid book_id", http.StatusBadRequest)
		return
	}

	book, err := app.lib.GetBook(bookID)
	if err != nil {
		if errors.Is(err, txtlib.ErrBookNotFound) {
			http.Error(w, "book not found", http.StatusNotFound)
			return
		}
		http.Error(w, "failed to get book", http.StatusInternalServerError)
		return
	}

	snapShot, err := book.GetSnapshot(book.CurrentSnapshot())
	if err != nil {
		http.Error(w, "failed to get book snapshot", http.StatusInternalServerError)
		return
	}

	src, err := snapShot.OpenSource()
	if err != nil {
		http.Error(w, "failed to open book snapshot source", http.StatusInternalServerError)
		return
	}
	defer src.Close()

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, err = io.Copy(w, src)
	if err != nil {
		http.Error(w, "failed to write book content", http.StatusInternalServerError)
		return
	}
}

// PATCH /api/books/{book_id}/split_config
func (app *App) HandleAPIUpdateBookSplitConfig(w http.ResponseWriter, r *http.Request) {
	bookID, err := readBookID(r)
	if err != nil {
		http.Error(w, "invalid book_id", http.StatusBadRequest)
		return
	}

	book, err := app.lib.GetBook(bookID)
	if err != nil {
		if errors.Is(err, txtlib.ErrBookNotFound) {
			http.Error(w, "book not found", http.StatusNotFound)
			return
		}
		http.Error(w, "failed to get book", http.StatusInternalServerError)
		return
	}

	var splitConfig txtlib.SplitConfig
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&splitConfig); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	snapshot, err := book.GetSnapshot(book.CurrentSnapshot())
	if err != nil {
		http.Error(w, "failed to get book snapshot", http.StatusInternalServerError)
		return
	}

	err = snapshot.UpdateSplitConfig(splitConfig)
	if err != nil {
		http.Error(w, "failed to update split config", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// GET /api/marks/{book_id}
func (app *App) HandleAPIGetMarks(w http.ResponseWriter, r *http.Request) {
	bookID, err := readBookID(r)
	if err != nil {
		http.Error(w, "invalid book_id", http.StatusBadRequest)
		return
	}

	mark, err := app.markLib.Get(bookID)
	if err != nil {
		http.Error(w, "failed to get marks", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	err = json.NewEncoder(w).Encode(mark)
	if err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
		return
	}
}

// POST /api/marks/{book_id}
func (app *App) HandleAPIUpdateMarks(w http.ResponseWriter, r *http.Request) {
	bookID, err := readBookID(r)
	if err != nil {
		http.Error(w, "invalid book_id", http.StatusBadRequest)
		return
	}

	var mark bookmark.Mark
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&mark); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	err = app.markLib.Set(bookID, mark)
	if err != nil {
		http.Error(w, "failed to update marks", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// GET /api/layers
func (app *App) HandleAPIGetLayers(w http.ResponseWriter, r *http.Request) {
	layers, err := app.lib.GetAllLayers()
	if err != nil {
		http.Error(w, "failed to get layers", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	err = json.NewEncoder(w).Encode(layers)
	if err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
		return
	}
}

// POST /api/layers/{layer_path}
func (app *App) HandleAPICreateLayer(w http.ResponseWriter, r *http.Request) {
	layerPath := strings.TrimSpace(r.PathValue("layer_path"))
	if layerPath == "" {
		http.Error(w, "layer path cannot be empty", http.StatusBadRequest)
		return
	}

	layerParts := strings.Split(layerPath, "/")

	err := app.lib.NewLayer(layerParts)
	if err != nil {
		http.Error(w, "failed to create layer", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// DELETE /api/layers/{layer_path}
func (app *App) HandleAPIDeleteLayer(w http.ResponseWriter, r *http.Request) {
	// TBD: implement layer deletion if needed (currently layers are deleted implicitly when deleting books)
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

// GET /api/books/duplicate
func (app *App) HandleAPIFindDuplicateBooks(w http.ResponseWriter, r *http.Request) {
	md5Groups := app.indexLib.GetMetaGroup("content_hash")

	groups := make([][]string, 0)
	for _, set := range md5Groups {
		items := set.Items()
		if len(items) > 1 {
			groups = append(groups, items)
		}
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	err := json.NewEncoder(w).Encode(groups)
	if err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
		return
	}
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

	mux.HandleFunc("GET /api/books/{book_id}/cover", app.HandleAPIGetBookCover)
	mux.HandleFunc("PUT /api/books/{book_id}/cover", app.HandleAPIUpdateBookCover)
	mux.HandleFunc("DELETE /api/books/{book_id}/cover", app.HandleAPIDeleteBookCover)

	mux.HandleFunc("GET /api/books/{book_id}/content", app.HandleAPIGetBookContent)
	mux.HandleFunc("PATCH /api/books/{book_id}/split_config", app.HandleAPIUpdateBookSplitConfig)

	mux.HandleFunc("GET /api/marks/{book_id}", app.HandleAPIGetMarks)
	mux.HandleFunc("POST /api/marks/{book_id}", app.HandleAPIUpdateMarks)

	mux.HandleFunc("GET /api/layers", app.HandleAPIGetLayers)
	mux.HandleFunc("POST /api/layers/{layer_path}", app.HandleAPICreateLayer)
	mux.HandleFunc("DELETE /api/layers/{layer_path}", app.HandleAPIDeleteLayer)

	mux.HandleFunc("GET /{path...}", app.HandleSPAFallback)
}

func readBookID(r *http.Request) (string, error) {
	bookID := strings.TrimSpace(r.PathValue("book_id"))
	if bookID == "" {
		bookID = strings.TrimSpace(r.URL.Query().Get("book_id"))
	}
	if bookID == "" {
		return "", errors.New("missing book_id")
	}

	decoded, err := url.PathUnescape(bookID)
	if err != nil {
		return "", util.Errorf("%w", err)
	}

	return decoded, nil
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
