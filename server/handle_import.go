package server

import (
	"mime"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/voilelab/plainshelf/internal/util"
	"github.com/voilelab/plainshelf/shelf"
)

const maxImportBodySize = 100 << 20 // 100 MB

const importTextMediaType = "text/plain"
const importMarkdownMediaType = "text/markdown"
const importXMarkdownMediaType = "text/x-markdown"
const importOctetStreamMediaType = "application/octet-stream"
const importEPUBMediaType = "application/epub+zip"
const importZipMediaType = "application/zip"

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

// isSupportedImportExt reports whether ext (as returned by filepath.Ext, lower-cased
// by the caller) is one of the file types accepted for book import.
func isSupportedImportExt(ext string) bool {
	return ext == ".txt" || ext == ".md" || ext == ".epub"
}

// isEPUBExt reports whether a filename should go through the EPUB conversion
// path rather than being stored as-is.
func isEPUBExt(ext string) bool {
	return ext == ".epub"
}

// bookFormatFromFilename derives the BookMeta.Format value ("txt" or "md") from a
// filename's extension. Callers must have already validated the extension is supported.
// It is not used for EPUB, whose stored format comes from the conversion strategy.
func bookFormatFromFilename(filename string) string {
	if strings.ToLower(filepath.Ext(filename)) == ".md" {
		return "md"
	}
	return "txt"
}

func validateImportFileHeader(header *multipart.FileHeader) error {
	if header == nil {
		return util.NewError("missing required field: file")
	}

	filename := strings.TrimSpace(header.Filename)
	ext := strings.ToLower(filepath.Ext(filename))
	if !isSupportedImportExt(ext) {
		return util.NewError("book file must be a .txt, .md or .epub file")
	}

	contentType := strings.TrimSpace(header.Header.Get("Content-Type"))

	if ext == ".epub" {
		// As with .md, the extension is the primary signal: browsers send
		// application/epub+zip, application/zip or application/octet-stream for
		// the same file depending on the platform's MIME database. The archive
		// itself is validated when it is parsed.
		if contentType == "" {
			return nil
		}

		mediaType, _, err := mime.ParseMediaType(contentType)
		if err != nil {
			return util.NewError("book file content type must be application/epub+zip")
		}
		switch strings.ToLower(mediaType) {
		case importEPUBMediaType, importZipMediaType, importOctetStreamMediaType:
			return nil
		default:
			return util.NewError("book file content type must be application/epub+zip")
		}
	}

	if ext == ".txt" {
		if contentType == "" {
			return util.NewError("book file content type must be text/plain")
		}

		mediaType, _, err := mime.ParseMediaType(contentType)
		if err != nil || strings.ToLower(mediaType) != importTextMediaType {
			return util.NewError("book file content type must be text/plain")
		}

		return nil
	}

	// ext == ".md": browsers disagree on what content type to send for Markdown
	// uploads (some send text/markdown, some text/plain, some application/octet-stream,
	// some nothing at all), so the extension is the primary signal here.
	if contentType == "" {
		return nil
	}

	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return util.NewError("book file content type must be text/markdown or text/plain")
	}
	switch strings.ToLower(mediaType) {
	case importTextMediaType, importMarkdownMediaType, importXMarkdownMediaType, importOctetStreamMediaType:
		return nil
	default:
		return util.NewError("book file content type must be text/markdown or text/plain")
	}
}

func validateLocalImportPath(localPath string) (string, error) {
	cleanPath := filepath.Clean(strings.TrimSpace(localPath))
	if cleanPath == "." {
		return "", util.NewError("book file path is required")
	}
	if !isSupportedImportExt(strings.ToLower(filepath.Ext(cleanPath))) {
		return "", util.NewError("book file must be a .txt, .md or .epub file")
	}

	info, err := os.Stat(cleanPath)
	if err != nil {
		return "", util.Errorf("%w", err)
	}
	if !info.Mode().IsRegular() {
		return "", util.NewError("book file must be a regular file")
	}

	return cleanPath, nil
}

// POST /api/shelves/{shelf_id}/books/import
func (app *App) HandleAPIImportBook(w http.ResponseWriter, r *http.Request) {
	shelfData, ok := app.resolveShelf(w, r)
	if !ok {
		return
	}

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

	if isEPUBExt(strings.ToLower(filepath.Ext(header.Filename))) {
		strategy, err := parseImportStrategy(r.FormValue("strategy"), app.epubImportStrategy())
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// multipart.File is an io.ReaderAt, so the archive is read randomly
		// rather than buffered whole.
		newBook, err := app.importEPUB(shelfData, f, header.Size, header.Filename, r.FormValue("title"), layerParts, strategy)
		if err != nil {
			app.writeEPUBImportError(w, err)
			return
		}

		writeImportedBook(w, app, newBook)
		return
	}

	// Re-encode before creating the book: this reads the upload, and the book
	// initializer below runs while the exclusive shelf lock is held.
	utf8File, _, err := util.ReEncodeToUTF8(f)
	if err != nil {
		app.Error("failed to re-encode uploaded file to UTF-8", "error", err)
		http.Error(w, "failed to re-encode uploaded file to UTF-8", http.StatusInternalServerError)
		return
	}

	// The source, current-source pointer, and detected metadata are all written
	// while the book is still staged, so an import either lands complete or not
	// at all.
	newBook, err := shelfData.NewBookWith(layerParts, title, func(book *shelf.Book) error {
		source, err := book.NewSource(utf8File)
		if err != nil {
			return err
		}
		if err := book.SetCurrentSource(source.ID()); err != nil {
			return err
		}

		meta := book.GetMeta()
		meta.Language = detectBookLang(book)
		meta.Format = bookFormatFromFilename(header.Filename)
		return book.SetMeta(meta)
	})
	if err != nil {
		app.writeErr(w, err, "failed to import book")
		return
	}

	writeImportedBook(w, app, newBook)
}

// writeEPUBImportError answers a failed EPUB import.
//
// A bad archive is reported with its detail, because the client is the only
// one who can act on it. Everything else goes through the shared mapping: an
// import creates a book, so it can fail for the same reasons any other write
// does, a layer the shelf refuses among them.
func (app *App) writeEPUBImportError(w http.ResponseWriter, err error) {
	if isEPUBInputError(err) {
		app.Error("failed to import epub", "error", err)
		http.Error(w, "failed to import epub: "+err.Error(), http.StatusBadRequest)
		return
	}

	app.writeErr(w, err, "failed to import epub")
}

func writeImportedBook(w http.ResponseWriter, app *App, newBook *shelf.Book) {
	app.writeJSON(w, http.StatusCreated, Book{
		Meta:  newBook.GetMeta(),
		Layer: newBook.Layers(),
	})
}

// ImportFromLocalPath imports a book from a local file path on the server.
// This is intended for desktop application use, where the client can specify a local file path and the server can access it directly.
func (app *App) ImportFromLocalPath(shelfID string, localPath string, layerParts shelf.Layers) (*shelf.Book, error) {
	cleanPath, err := validateLocalImportPath(localPath)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	targetShelfID := strings.TrimSpace(shelfID)
	if targetShelfID == "" {
		return nil, util.Errorf("shelf ID cannot be empty")
	}

	shelfData, ok := app.shelfManager.GetShelf(targetShelfID)
	if !ok {
		return nil, util.Errorf("shelf not found: %s", targetShelfID)
	}

	fp, err := os.Open(cleanPath)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}
	defer fp.Close()

	if isEPUBExt(strings.ToLower(filepath.Ext(cleanPath))) {
		info, err := fp.Stat()
		if err != nil {
			return nil, util.Errorf("%w", err)
		}
		// The desktop client has no per-import options, so the configured
		// default strategy is the whole story here.
		return app.importEPUB(shelfData, fp, info.Size(), filepath.Base(cleanPath), "", layerParts, app.epubImportStrategy())
	}

	// Re-encode before creating the book: this reads the file, and the book
	// initializer below runs while the exclusive shelf lock is held.
	utf8Reader, _, err := util.ReEncodeToUTF8(fp)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	// The source, current-source pointer, and detected metadata are all written
	// while the book is still staged, so an import either lands complete or not
	// at all.
	newBook, err := shelfData.NewBookWith(layerParts, filepath.Base(cleanPath), func(book *shelf.Book) error {
		source, err := book.NewSource(utf8Reader)
		if err != nil {
			return err
		}
		if err := book.SetCurrentSource(source.ID()); err != nil {
			return err
		}

		meta := book.GetMeta()
		meta.Language = detectBookLang(book)
		meta.Format = bookFormatFromFilename(cleanPath)
		return book.SetMeta(meta)
	})
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	return newBook, nil
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
