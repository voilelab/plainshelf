package txtlibsrv

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"github.com/voilelab/plainshelf/internal/util"
	"github.com/voilelab/plainshelf/shelf"
)

const maxImportBodySize = 100 << 20 // 100 MB

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

// POST /api/books/import
func (app *App) HandleAPIImportBook(w http.ResponseWriter, r *http.Request) {
	// Limit overall request body size.
	r.Body = http.MaxBytesReader(w, r.Body, maxImportBodySize)

	if err := r.ParseMultipartForm(32 << 20); err != nil {
		if err.Error() == "http: request body too large" {
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

	// Optional fields.
	title := r.FormValue("title")
	if title == "" {
		title = header.Filename
	}
	layerParts := parseImportLayerParts(r.FormValue("layer"))

	newBook, err := app.lib.NewBook(layerParts, title)
	if err != nil {
		log.Printf("NewBook error: %v", err)
		http.Error(w, "failed to create new book", http.StatusInternalServerError)
		return
	}

	utf8File, _, err := util.ReEncodeToUTF8(f)
	if err != nil {
		log.Printf("ReEncodeToUTF8 error: %v", err)
		http.Error(w, "failed to re-encode uploaded file to UTF-8", http.StatusInternalServerError)
		return
	}

	snapshot, err := newBook.NewSnapshot(utf8File)
	if err != nil {
		log.Printf("NewSnapshot error: %v", err)
		http.Error(w, "failed to create snapshot from uploaded file", http.StatusInternalServerError)
		return
	}

	newBook.SetCurrentSnapshot(snapshot.ID())

	meta := newBook.GetMeta()
	meta.Language = detectBookLang(newBook)
	if err := newBook.SetMeta(meta); err != nil {
		log.Printf("SetMeta error: %v", err)
	}

	resp := Book{
		Meta:  newBook.GetMeta(),
		Layer: newBook.Layers(),
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.Printf("HandleAPIImportBook encode response: %v", err)
	}
}

func detectBookLang(book *shelf.Book) string {
	snapshot, err := book.GetSnapshot(book.CurrentSnapshot())
	if err != nil {
		return ""
	}

	reader, err := snapshot.OpenSource()
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
