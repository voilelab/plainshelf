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

	// CharCount is only populated when the request includes
	// include=char_count; it is omitted otherwise, so the default response
	// shape is unchanged.
	CharCount int `json:"char_count,omitempty"`
}

type UpdateBookRequest struct {
	Title       *string            `json:"title"`
	Authors     *[]string          `json:"authors"`
	Tags        *[]string          `json:"tags"`
	Identifiers *map[string]string `json:"identifiers"`
	Language    *string            `json:"language"`
	Comment     *string            `json:"comment"`
	Star        *int               `json:"star"`
	PublishedAt *util.JSONDate     `json:"published_at"`
	Layer       *shelf.Layers      `json:"layer"`
	Layers      *shelf.Layers      `json:"layers"`
}

func (app *App) GetBookFolderPath(shelfID, bookID string) (string, error) {
	shelfID = strings.TrimSpace(shelfID)
	if shelfID == "" {
		return "", util.Errorf("shelf ID cannot be empty")
	}

	bookID = strings.TrimSpace(bookID)
	if bookID == "" {
		return "", util.Errorf("book ID cannot be empty")
	}

	shelfData, ok := app.shelfManager.GetShelf(shelfID)
	if !ok {
		return "", util.Errorf("shelf with ID %q not found", shelfID)
	}

	book, err := shelfData.GetBook(bookID)
	if err != nil {
		return "", util.Errorf("%w", err)
	}

	return book.FolderPath(), nil
}

// GET /api/shelves/{shelf_id}/books
func (app *App) HandleAPIGetBooks(w http.ResponseWriter, r *http.Request) {
	shelfData, ok := app.resolveShelf(w, r)
	if !ok {
		return
	}

	books, err := shelfData.ListBooks()
	if err != nil {
		app.writeErr(w, err, "failed to list books")
		return
	}

	includeCharCount := false
	for inc := range strings.SplitSeq(r.URL.Query().Get("include"), ",") {
		if strings.TrimSpace(inc) == "char_count" {
			includeCharCount = true
			break
		}
	}

	jsonBooks := make([]Book, len(books))
	for i, b := range books {
		jsonBooks[i] = Book{
			Meta:  b.GetMeta(),
			Layer: b.Layers(),
		}
		if includeCharCount {
			// A single book with a broken/missing source shouldn't fail the
			// whole list - just skip char_count for that book.
			if source, srcErr := b.GetSource(b.CurrentSource()); srcErr == nil {
				jsonBooks[i].CharCount = source.GetMeta().CharCount
			}
		}
	}

	app.writeJSON(w, http.StatusOK, jsonBooks)
}

// POST /api/shelves/{shelf_id}/books
func (app *App) HandleAPICreateBook(w http.ResponseWriter, r *http.Request) {
	shelfData, ok := app.resolveShelf(w, r)
	if !ok {
		return
	}

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

	// The empty source and the current-source pointer are written while the book
	// is still staged, so a failure here leaves no half-built book on disk.
	newBook, err := shelfData.NewBookWith(req.Layer, req.Title, func(book *shelf.Book) error {
		source, err := book.NewSource(nil)
		if err != nil {
			return err
		}
		return book.SetCurrentSource(source.ID())
	})
	if err != nil {
		app.Error("failed to create new book", "error", err)
		http.Error(w, "failed to create new book", http.StatusInternalServerError)
		return
	}

	app.writeJSON(w, http.StatusCreated, Book{
		Meta:  newBook.GetMeta(),
		Layer: newBook.Layers(),
	})
}

// GET /api/shelves/{shelf_id}/books/{book_id}
func (app *App) HandleAPIGetBook(w http.ResponseWriter, r *http.Request) {
	_, book, ok := app.loadBook(w, r)
	if !ok {
		return
	}

	app.writeJSON(w, http.StatusOK, Book{
		Meta:  book.GetMeta(),
		Layer: book.Layers(),
	})
}

// PATCH /api/shelves/{shelf_id}/books/{book_id}
func (app *App) HandleAPIUpdateBook(w http.ResponseWriter, r *http.Request) {
	shelfData, ok := app.resolveShelf(w, r)
	if !ok {
		return
	}

	// The body is decoded before the book is loaded so a malformed body is
	// still reported as 400 for a book that does not exist.
	bookID, ok := resolveBookID(w, r)
	if !ok {
		return
	}

	var req UpdateBookRequest
	if !decodeStrictJSON(w, r, &req) {
		return
	}

	book, ok := app.getBook(w, shelfData, bookID)
	if !ok {
		return
	}

	// Refuse a book this build must not modify before doing anything, otherwise
	// a layer move would be applied to disk and then reported as a failure.
	if err := book.EnsureWritable(); err != nil {
		app.writeErr(w, err, "failed to update book metadata")
		return
	}

	if target := req.targetLayers(); target != nil {
		movedBook, err := shelfData.MoveBook(bookID, append(shelf.Layers(nil), (*target)...))
		if err != nil {
			http.Error(w, "failed to move book layer", http.StatusInternalServerError)
			return
		}
		book = movedBook
	}

	meta := *book.GetMeta()
	applyBookPatch(&meta, &req)

	if err := book.SetMeta(&meta); err != nil {
		app.writeErr(w, err, "failed to update book metadata")
		return
	}

	app.writeJSON(w, http.StatusOK, Book{Meta: &meta, Layer: book.Layers()})
}

// targetLayers reports the layer the request asks the book to move to, or nil
// when it asks for no move. "layer" is the current field; "layers" is still
// accepted because older clients send that name.
func (req *UpdateBookRequest) targetLayers() *shelf.Layers {
	if req.Layer != nil {
		return req.Layer
	}
	return req.Layers
}

// applyBookPatch copies the fields the request actually set onto meta and
// stamps the update time. Fields left nil are untouched, which is what makes
// the route a PATCH rather than a replace.
//
// It validates nothing: the field rules belong to shelf, which enforces them in
// SetMeta before anything reaches disk.
func applyBookPatch(meta *shelf.BookMeta, req *UpdateBookRequest) {
	if req.Title != nil {
		meta.Title = *req.Title
	}
	if req.Authors != nil {
		meta.Authors = append([]string(nil), (*req.Authors)...)
	}
	if req.Tags != nil {
		meta.Tags = append([]string(nil), (*req.Tags)...)
	}
	if req.Identifiers != nil {
		meta.Identifiers = *req.Identifiers
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
	if req.Star != nil {
		meta.Star = *req.Star
	}

	meta.UpdatedAt = util.JSONTime(time.Now())
}

// GET /api/shelves/{shelf_id}/books/{book_id}/cover
func (app *App) HandleAPIGetBookCover(w http.ResponseWriter, r *http.Request) {
	_, book, ok := app.loadBook(w, r)
	if !ok {
		return
	}

	if etag := book.CoverETag(); etag != "" {
		w.Header().Set("ETag", etag)
		w.Header().Set("Cache-Control", "public, max-age=86400")
		if r.Header.Get("If-None-Match") == etag {
			w.WriteHeader(http.StatusNotModified)
			return
		}
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
	case ".webp":
		contentType = "image/webp"
	case ".gif":
		contentType = "image/gif"
	}

	w.Header().Set("Content-Type", contentType)
	w.Write(coverData)
}

// PUT /api/shelves/{shelf_id}/books/{book_id}/cover
func (app *App) HandleAPIUpdateBookCover(w http.ResponseWriter, r *http.Request) {
	_, book, ok := app.loadBook(w, r)
	if !ok {
		return
	}

	contentType := r.Header.Get("Content-Type")
	coverToJPG := app.coverToJPG()
	var ext string
	switch contentType {
	case "image/png":
		ext = ".png"
	case "image/jpeg":
		ext = ".jpg"
	case "image/webp":
		ext = ".webp"
	case "image/gif":
		ext = ".gif"
	default:
		if !coverToJPG {
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

	if coverToJPG {
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
		app.writeErr(w, err, "failed to update book cover")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// DELETE /api/shelves/{shelf_id}/books/{book_id}/cover
func (app *App) HandleAPIDeleteBookCover(w http.ResponseWriter, r *http.Request) {
	_, book, ok := app.loadBook(w, r)
	if !ok {
		return
	}

	if err := book.DeleteCover(); err != nil {
		app.writeErr(w, err, "failed to delete book cover")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// GET /api/shelves/{shelf_id}/books/{book_id}/content
func (app *App) HandleAPIGetBookContent(w http.ResponseWriter, r *http.Request) {
	_, book, ok := app.loadBook(w, r)
	if !ok {
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

	app.streamTextFile(w, src, "failed to write book content")
}

// GET /api/shelves/{shelf_id}/books/{book_id}/split_config
func (app *App) HandleAPIGetBookSplitConfig(w http.ResponseWriter, r *http.Request) {
	_, book, ok := app.loadBook(w, r)
	if !ok {
		return
	}

	source, err := book.GetSource(book.CurrentSource())
	if err != nil {
		app.Error("failed to get book source", "error", err)
		http.Error(w, "failed to get book source", http.StatusInternalServerError)
		return
	}

	app.writeJSON(w, http.StatusOK, source.GetMeta().SplitConfig)
}

// PATCH /api/shelves/{shelf_id}/books/{book_id}/split_config
func (app *App) HandleAPIUpdateBookSplitConfig(w http.ResponseWriter, r *http.Request) {
	_, book, ok := app.loadBook(w, r)
	if !ok {
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

// GET /api/shelves/{shelf_id}/books/duplicate
func (app *App) HandleAPIFindDuplicateBooks(w http.ResponseWriter, r *http.Request) {
	shelfData, ok := app.resolveShelf(w, r)
	if !ok {
		return
	}

	md5Groups := map[string][]string{}
	books, err := shelfData.ListBooks()
	if err != nil {
		app.writeErr(w, err, "failed to list books")
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

	app.writeJSON(w, http.StatusOK, groups)
}
