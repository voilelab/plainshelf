package txtlibsrv

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/voilelab/plainshelf/internal/imgutil"
	"github.com/voilelab/plainshelf/internal/util"
	"github.com/voilelab/plainshelf/txtlib"
)

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
	searchQuery := strings.TrimSpace(r.URL.Query().Get("search"))

	books, err := app.lib.ListBooks()
	if err != nil {
		http.Error(w, "failed to list books", http.StatusInternalServerError)
		return
	}

	if searchQuery != "" {
		newBooks := make([]*txtlib.Book, 0)
		for _, b := range books {
			meta := b.GetMeta()
			if strings.Contains(meta.Title, searchQuery) ||
				strings.Contains(meta.Comments, searchQuery) {
				newBooks = append(newBooks, b)
				continue
			}
		}
		books = newBooks
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
		if !app.conf.CoverToJPG {
			http.Error(w, "unsupported content type", http.StatusBadRequest)
			return
		}
	}

	data, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read request body", http.StatusInternalServerError)
		return
	}

	if app.conf.CoverToJPG {
		data, err = imgutil.AnyToJPG(data)
		if err != nil {
			http.Error(w, "failed to convert image to JPEG", http.StatusInternalServerError)
			return
		}
		ext = ".jpg"
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

// GET /api/books/{book_id}/snapshots
func (app *App) HandleAPIGetBookSnapshots(w http.ResponseWriter, r *http.Request) {
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

	snapshots, err := book.ListSnapshot()
	if err != nil {
		http.Error(w, "failed to list book snapshots", http.StatusInternalServerError)
		return
	}

	snapshotMetas := make([]*txtlib.SnapshotMeta, len(snapshots))
	for i, s := range snapshots {
		snapshotMetas[i] = s.GetMeta()
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	err = json.NewEncoder(w).Encode(snapshotMetas)
	if err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
		return
	}
}

// GET /api/books/{book_id}/snapshots/{snapshot_id}/content
func (app *App) HandleAPIGetBookSnapshotContent(w http.ResponseWriter, r *http.Request) {
	bookID, err := readBookID(r)
	if err != nil {
		http.Error(w, "invalid book_id", http.StatusBadRequest)
		return
	}

	snapshotID, err := readSnapshotID(r)
	if err != nil {
		http.Error(w, "invalid snapshot_id", http.StatusBadRequest)
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

	snapshot, err := book.GetSnapshot(snapshotID)
	if err != nil {
		http.Error(w, "failed to get book snapshot", http.StatusInternalServerError)
		return
	}

	src, err := snapshot.OpenSource()
	if err != nil {
		http.Error(w, "failed to open book snapshot source", http.StatusInternalServerError)
		return
	}
	defer src.Close()

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, err = io.Copy(w, src)
	if err != nil {
		http.Error(w, "failed to write book snapshot content", http.StatusInternalServerError)
		return
	}
}

// PATCH /api/books/{book_id}/snapshots/{snapshot_id}/content
func (app *App) HandleAPIUpdateBookSnapshotContent(w http.ResponseWriter, r *http.Request) {
	bookID, err := readBookID(r)
	if err != nil {
		http.Error(w, "invalid book_id", http.StatusBadRequest)
		return
	}

	snapshotID, err := readSnapshotID(r)
	if err != nil {
		http.Error(w, "invalid snapshot_id", http.StatusBadRequest)
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

	snapshot, err := book.GetSnapshot(snapshotID)
	if err != nil {
		http.Error(w, "failed to get book snapshot", http.StatusInternalServerError)
		return
	}

	err = snapshot.UpdateContent(r.Body)
	if err != nil {
		http.Error(w, "failed to update book snapshot content", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// GET /api/books/{book_id}/split_config
func (app *App) HandleAPIGetBookSplitConfig(w http.ResponseWriter, r *http.Request) {
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

	snapshot, err := book.GetSnapshot(book.CurrentSnapshot())
	if err != nil {
		http.Error(w, "failed to get book snapshot", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	err = json.NewEncoder(w).Encode(snapshot.GetMeta().SplitConfig)
	if err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
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

// GET /api/books/duplicate
func (app *App) HandleAPIFindDuplicateBooks(w http.ResponseWriter, r *http.Request) {
	md5Groups := map[string][]string{}
	books, err := app.lib.ListBooks()
	if err != nil {
		http.Error(w, "failed to list books", http.StatusInternalServerError)
		return
	}

	for _, b := range books {
		snapshot, err := b.GetSnapshot(b.CurrentSnapshot())
		if err != nil {
			log.Printf("failed to get snapshot for book %s: %v", b.ID(), err)
			continue
		}
		meta := snapshot.GetMeta()
		md5Groups[meta.MD5Hash] = append(md5Groups[meta.MD5Hash], b.ID())
	}

	groups := [][]string{}
	for _, ids := range md5Groups {
		if len(ids) > 1 {
			groups = append(groups, ids)
		}
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	err = json.NewEncoder(w).Encode(groups)
	if err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
		return
	}
}
