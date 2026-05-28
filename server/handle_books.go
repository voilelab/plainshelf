package server

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/voilelab/plainshelf/internal/imgutil"
	"github.com/voilelab/plainshelf/internal/util"
	"github.com/voilelab/plainshelf/shelf"
)

const maxCoverBodySize = 20 << 20 // 20 MB

func isRequestBodyTooLarge(err error) bool {
	var maxBytesErr *http.MaxBytesError
	return errors.As(err, &maxBytesErr)
}

type Book struct {
	Meta  *shelf.BookMeta `json:"meta"`
	Layer shelf.Layers    `json:"layer"`
}

type UpdateBookRequest struct {
	Title       *string        `json:"title"`
	Authors     *[]string      `json:"authors"`
	Tags        *[]string      `json:"tags"`
	Language    *string        `json:"language"`
	Comment     *string        `json:"comment"`
	PublishedAt *util.JSONTime `json:"published_at"`
	Layer       *shelf.Layers  `json:"layer"`
	Layers      *shelf.Layers  `json:"layers"`
}

// GET /api/books
func (app *App) HandleAPIGetBooks(w http.ResponseWriter, r *http.Request) {
	searchQuery := strings.TrimSpace(r.URL.Query().Get("search"))

	books, err := app.shelf.ListBooks()
	if err != nil {
		app.Error("failed to list books", "error", err)
		http.Error(w, "failed to list books", http.StatusInternalServerError)
		return
	}

	if searchQuery != "" {
		newBooks := make([]*shelf.Book, 0)
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
		app.Error("failed to encode response", "error", err)
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
		return
	}
}

// POST /api/books
func (app *App) HandleAPICreateBook(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Title string       `json:"title"`
		Layer shelf.Layers `json:"layer"`
	}

	bs, err := io.ReadAll(r.Body)
	if err != nil {
		app.Error("failed to read request body", "error", err)
		http.Error(w, "failed to read request body", http.StatusInternalServerError)
		return
	}

	err = json.Unmarshal(bs, &req)
	if err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	newBook, err := app.shelf.NewBook(req.Layer, req.Title)
	if err != nil {
		app.Error("failed to create new book", "error", err)
		http.Error(w, "failed to create new book", http.StatusInternalServerError)
		return
	}

	source, err := newBook.NewSource(nil)
	if err != nil {
		app.Error("failed to create source for new book", "error", err)
		http.Error(w, "failed to create source for new book", http.StatusInternalServerError)
		return
	}

	err = newBook.SetCurrentSource(source.ID())
	if err != nil {
		app.Error("failed to set current source for new book", "error", err)
		http.Error(w, "failed to set current source for new book", http.StatusInternalServerError)
		return
	}

	jsonBook := Book{
		Meta:  newBook.GetMeta(),
		Layer: newBook.Layers(),
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusCreated)
	err = json.NewEncoder(w).Encode(jsonBook)
	if err != nil {
		app.Error("failed to encode response", "error", err)
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
	book, err := app.shelf.GetBook(bookID)
	if err != nil {
		if errors.Is(err, shelf.ErrBookNotFound) {
			http.Error(w, "book not found", http.StatusNotFound)
			return
		}
		app.Error("failed to get book", "error", err)
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
		app.Error("failed to encode response", "error", err)
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

	book, err := app.shelf.GetBook(bookID)
	if err != nil {
		if errors.Is(err, shelf.ErrBookNotFound) {
			http.Error(w, "book not found", http.StatusNotFound)
			return
		}
		app.Error("failed to get book", "error", err)
		http.Error(w, "failed to get book", http.StatusInternalServerError)
		return
	}

	targetLayers := req.Layer
	if targetLayers == nil {
		targetLayers = req.Layers
	}
	if targetLayers != nil {
		movedBook, err := app.shelf.MoveBook(bookID, append(shelf.Layers(nil), (*targetLayers)...))
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
	if req.PublishedAt != nil {
		meta.PublishedAt = *req.PublishedAt
	}
	meta.UpdatedAt = util.JSONTime(time.Now())

	if err := book.SetMeta(&meta); err != nil {
		app.Error("failed to update book metadata", "error", err)
		http.Error(w, "failed to update book metadata", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	err = json.NewEncoder(w).Encode(Book{Meta: &meta, Layer: book.Layers()})
	if err != nil {
		app.Error("failed to encode response", "error", err)
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

	err = app.shelf.MoveBookToTrash(bookID)
	if err != nil {
		if errors.Is(err, shelf.ErrBookNotFound) {
			http.Error(w, "book not found", http.StatusNotFound)
			return
		}
		app.Error("failed to trash book", "error", err)
		http.Error(w, "failed to trash book", http.StatusInternalServerError)
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

	book, err := app.shelf.GetBook(bookID)
	if err != nil {
		if errors.Is(err, shelf.ErrBookNotFound) {
			http.Error(w, "book not found", http.StatusNotFound)
			return
		}
		app.Error("failed to get book", "error", err)
		http.Error(w, "failed to get book", http.StatusInternalServerError)
		return
	}

	coverData, ext, err := book.OpenCover()
	if err != nil {
		app.Error("failed to open book cover", "error", err)
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

	book, err := app.shelf.GetBook(bookID)
	if err != nil {
		if errors.Is(err, shelf.ErrBookNotFound) {
			http.Error(w, "book not found", http.StatusNotFound)
			return
		}
		app.Error("failed to get book", "error", err)
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

	r.Body = http.MaxBytesReader(w, r.Body, maxCoverBodySize)
	data, err := io.ReadAll(r.Body)
	if err != nil {
		if isRequestBodyTooLarge(err) {
			http.Error(w, "request body too large (max 20 MB)", http.StatusRequestEntityTooLarge)
			return
		}
		app.Error("failed to read request body", "error", err)
		http.Error(w, "failed to read request body", http.StatusInternalServerError)
		return
	}

	if app.conf.CoverToJPG {
		data, err = imgutil.AnyToJPG(data)
		if err != nil {
			app.Error("failed to convert image to JPEG", "error", err)
			http.Error(w, "failed to convert image to JPEG", http.StatusInternalServerError)
			return
		}
		ext = ".jpg"
	}

	err = book.SetCover(data, ext)
	if err != nil {
		app.Error("failed to update book cover", "error", err)
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

	book, err := app.shelf.GetBook(bookID)
	if err != nil {
		if errors.Is(err, shelf.ErrBookNotFound) {
			http.Error(w, "book not found", http.StatusNotFound)
			return
		}
		app.Error("failed to get book", "error", err)
		http.Error(w, "failed to get book", http.StatusInternalServerError)
		return
	}

	err = book.DeleteCover()
	if err != nil {
		app.Error("failed to delete book cover", "error", err)
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

	book, err := app.shelf.GetBook(bookID)
	if err != nil {
		if errors.Is(err, shelf.ErrBookNotFound) {
			http.Error(w, "book not found", http.StatusNotFound)
			return
		}
		app.Error("failed to get book", "error", err)
		http.Error(w, "failed to get book", http.StatusInternalServerError)
		return
	}

	source, err := book.GetSource(book.CurrentSource())
	if err != nil {
		http.Error(w, "failed to get book source", http.StatusInternalServerError)
		return
	}

	src, err := source.Open()
	if err != nil {
		app.Error("failed to open book source", "error", err)
		http.Error(w, "failed to open book source", http.StatusInternalServerError)
		return
	}
	defer src.Close()

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, err = io.Copy(w, src)
	if err != nil {
		app.Error("failed to write book content", "error", err)
		http.Error(w, "failed to write book content", http.StatusInternalServerError)
		return
	}
}

// GET /api/books/{book_id}/split_config
func (app *App) HandleAPIGetBookSplitConfig(w http.ResponseWriter, r *http.Request) {
	bookID, err := readBookID(r)
	if err != nil {
		http.Error(w, "invalid book_id", http.StatusBadRequest)
		return
	}

	book, err := app.shelf.GetBook(bookID)
	if err != nil {
		if errors.Is(err, shelf.ErrBookNotFound) {
			http.Error(w, "book not found", http.StatusNotFound)
			return
		}
		app.Error("failed to get book", "error", err)
		http.Error(w, "failed to get book", http.StatusInternalServerError)
		return
	}

	source, err := book.GetSource(book.CurrentSource())
	if err != nil {
		app.Error("failed to get book source", "error", err)
		http.Error(w, "failed to get book source", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	err = json.NewEncoder(w).Encode(source.GetMeta().SplitConfig)
	if err != nil {
		app.Error("failed to encode response", "error", err)
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

	book, err := app.shelf.GetBook(bookID)
	if err != nil {
		if errors.Is(err, shelf.ErrBookNotFound) {
			http.Error(w, "book not found", http.StatusNotFound)
			return
		}
		app.Error("failed to get book", "error", err)
		http.Error(w, "failed to get book", http.StatusInternalServerError)
		return
	}

	var splitConfig shelf.SplitConfig
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&splitConfig); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	source, err := book.GetSource(book.CurrentSource())
	if err != nil {
		app.Error("failed to get book source", "error", err)
		http.Error(w, "failed to get book source", http.StatusInternalServerError)
		return
	}

	err = source.UpdateSplitConfig(splitConfig)
	if err != nil {
		app.Error("failed to update book split config", "error", err)
		http.Error(w, "failed to update split config", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// GET /api/books/duplicate
func (app *App) HandleAPIFindDuplicateBooks(w http.ResponseWriter, r *http.Request) {
	md5Groups := map[string][]string{}
	books, err := app.shelf.ListBooks()
	if err != nil {
		app.Error("failed to list books", "error", err)
		http.Error(w, "failed to list books", http.StatusInternalServerError)
		return
	}

	for _, b := range books {
		source, err := b.GetSource(b.CurrentSource())
		if err != nil {
			app.Warn("failed to get source for book", "book_id", b.ID(), "error", err)
			continue
		}
		meta := source.GetMeta()
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
		app.Error("failed to encode response", "error", err)
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
		return
	}
}
