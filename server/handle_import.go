package server

import (
	"encoding/json"
	"mime"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/voilelab/plainshelf/internal/util"
	"github.com/voilelab/plainshelf/shelf"
)

const maxImportBodySize = 100 << 20 // 100 MB

const importTextMediaType = "text/plain"

func parseImportLayerParts(rawLayer string) []string {
	trimmed := strings.TrimSpace(rawLayer)
	if trimmed == "" || trimmed == "/" {
		return nil
	}

	trimmed = strings.Trim(trimmed, "/")
	if trimmed == "" {
		return nil
	}

	parts := make([]string, 0)
	for part := range strings.SplitSeq(trimmed, "/") {
		normalizedPart := strings.TrimSpace(part)
		if normalizedPart == "" {
			continue
		}
		parts = append(parts, normalizedPart)
	}

	if len(parts) == 0 {
		return nil
	}

	return parts
}

func validateImportFileHeader(header *multipart.FileHeader) error {
	if header == nil {
		return util.NewError("missing required field: file")
	}

	filename := strings.TrimSpace(header.Filename)
	if strings.ToLower(filepath.Ext(filename)) != ".txt" {
		return util.NewError("book file must be a .txt file")
	}

	contentType := strings.TrimSpace(header.Header.Get("Content-Type"))
	if contentType == "" {
		return util.NewError("book file content type must be text/plain")
	}

	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil || strings.ToLower(mediaType) != importTextMediaType {
		return util.NewError("book file content type must be text/plain")
	}

	return nil
}

// POST /api/books/import
func (app *App) HandleAPIImportBook(w http.ResponseWriter, r *http.Request) {
	// Limit overall request body size.
	r.Body = http.MaxBytesReader(w, r.Body, maxImportBodySize)

	if err := r.ParseMultipartForm(32 << 20); err != nil {
		if isRequestBodyTooLarge(err) {
			http.Error(w, "request body too large (max 100 MB)", http.StatusRequestEntityTooLarge)
			return
		}
		http.Error(w, "invalid multipart form: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Required: file field.
	f, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "missing required field: file", http.StatusBadRequest)
		return
	}
	defer f.Close()

	if err := validateImportFileHeader(header); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Optional fields.
	title := r.FormValue("title")
	if title == "" {
		title = header.Filename
	}
	layerParts := parseImportLayerParts(r.FormValue("layer"))

	newBook, err := app.shelf.NewBook(layerParts, title)
	if err != nil {
		app.Error("failed to create new book", "error", err)
		http.Error(w, "failed to create new book", http.StatusInternalServerError)
		return
	}

	utf8File, _, err := util.ReEncodeToUTF8(f)
	if err != nil {
		app.Error("failed to re-encode uploaded file to UTF-8", "error", err)
		http.Error(w, "failed to re-encode uploaded file to UTF-8", http.StatusInternalServerError)
		return
	}

	source, err := newBook.NewSource(utf8File)
	if err != nil {
		app.Error("failed to create source from uploaded file", "error", err)
		http.Error(w, "failed to create source from uploaded file", http.StatusInternalServerError)
		return
	}

	newBook.SetCurrentSource(source.ID())

	meta := newBook.GetMeta()
	meta.Language = detectBookLang(newBook)
	if err := newBook.SetMeta(meta); err != nil {
		app.Error("failed to set book meta", "error", err)
	}

	resp := Book{
		Meta:  newBook.GetMeta(),
		Layer: newBook.Layers(),
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		app.Error("failed to encode response", "error", err)
	}
}

func detectBookLang(book *shelf.Book) string {
	source, err := book.GetSource(book.CurrentSource())
	if err != nil {
		return ""
	}

	reader, err := source.Open()
	if err != nil {
		return ""
	}
	defer reader.Close()

	lang, err := util.DetectLanguage(reader)
	if err != nil {
		return ""
	}

	return lang
}
