package server

import (
	"errors"
	"io"
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

// importHandlers turns an uploaded or local file into a book. It reads settings
// because an EPUB import that carries no strategy of its own falls back to the
// configured one.
type importHandlers struct {
	*apiCore

	settings *settings
}

func parseImportFolderParts(rawFolder string) []string {
	trimmed := strings.TrimSpace(rawFolder)
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

// deriveTitleFromFilename strips a single trailing extension from a filename to
// use it as a book title, mirroring the frontend's deriveTitleFromFilename
// (frontend/src/utils/file.ts) so the web-upload and desktop local-path imports
// agree. A leading-dot name (".bashrc") and an empty stem both fall back to the
// full filename rather than producing an empty title.
func deriveTitleFromFilename(filename string) string {
	trimmed := strings.TrimSpace(filename)
	if trimmed == "" {
		return filename
	}

	dotIndex := strings.LastIndex(trimmed, ".")
	withoutExt := trimmed
	if dotIndex > 0 {
		withoutExt = trimmed[:dotIndex]
	}

	title := strings.TrimSpace(withoutExt)
	if title == "" {
		return trimmed
	}
	return title
}

// bookFormatFromFilename derives the BookMeta.Format value ("txt" or "md") from a
// filename's extension. Callers must have already validated the extension is supported.
// It is not used for EPUB, whose stored format comes from the conversion strategy.
func bookFormatFromFilename(filename string) string {
	if strings.ToLower(filepath.Ext(filename)) == ".md" {
		return shelf.BookFormatMarkdown
	}
	return shelf.BookFormatText
}

// validateImportFileHeader returns the message shown to the client separately
// from the error, which is logged and carries util.Errorf's function prefix.
func validateImportFileHeader(header *multipart.FileHeader) (string, error) {
	reject := func(message string) (string, error) {
		return message, util.Errorf("%s", message)
	}

	if header == nil {
		return reject("missing required field: file")
	}

	filename := strings.TrimSpace(header.Filename)
	ext := strings.ToLower(filepath.Ext(filename))
	if !isSupportedImportExt(ext) {
		return reject("book file must be a .txt, .md or .epub file")
	}

	contentType := strings.TrimSpace(header.Header.Get("Content-Type"))

	if ext == ".epub" {
		// As with .md, the extension is the primary signal: browsers send
		// application/epub+zip, application/zip or application/octet-stream for
		// the same file depending on the platform's MIME database. The archive
		// itself is validated when it is parsed.
		if contentType == "" {
			return "", nil
		}

		mediaType, _, err := mime.ParseMediaType(contentType)
		if err != nil {
			return reject("book file content type must be application/epub+zip")
		}
		switch strings.ToLower(mediaType) {
		case importEPUBMediaType, importZipMediaType, importOctetStreamMediaType:
			return "", nil
		default:
			return reject("book file content type must be application/epub+zip")
		}
	}

	if ext == ".txt" {
		if contentType == "" {
			return reject("book file content type must be text/plain")
		}

		mediaType, _, err := mime.ParseMediaType(contentType)
		if err != nil || strings.ToLower(mediaType) != importTextMediaType {
			return reject("book file content type must be text/plain")
		}

		return "", nil
	}

	// ext == ".md": browsers disagree on what content type to send for Markdown
	// uploads (some send text/markdown, some text/plain, some application/octet-stream,
	// some nothing at all), so the extension is the primary signal here.
	if contentType == "" {
		return "", nil
	}

	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return reject("book file content type must be text/markdown or text/plain")
	}
	switch strings.ToLower(mediaType) {
	case importTextMediaType, importMarkdownMediaType, importXMarkdownMediaType, importOctetStreamMediaType:
		return "", nil
	default:
		return reject("book file content type must be text/markdown or text/plain")
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

// newPlainTextBook stores an already re-encoded .txt or .md file as a new book.
//
// The source, the current-source pointer and the detected metadata are all
// written while the book is still staged, so an import either lands complete or
// not at all. The caller re-encodes before calling: that reads the file, while
// this initializer runs with the exclusive shelf lock held.
func newPlainTextBook(
	shelfData *shelf.ShelfData,
	utf8Reader io.Reader,
	folderParts shelf.FolderPath,
	title string,
	filename string,
) (*shelf.Book, error) {
	return shelfData.NewBookWith(folderParts, title, func(book *shelf.Book) error {
		format := bookFormatFromFilename(filename)
		source, err := book.NewSourceWithOptions(utf8Reader, shelf.NewSourceOptions{Format: format})
		if err != nil {
			return err
		}
		if err := book.SetCurrentSource(source.ID()); err != nil {
			return err
		}

		meta := book.GetMeta()
		meta.Language = detectBookLang(book)
		meta.Format = format
		return book.SetMeta(meta)
	})
}

// POST /api/shelves/{shelf_id}/books/import
func (h *importHandlers) importBook(w http.ResponseWriter, r *http.Request) {
	shelfData, ok := h.resolveShelf(w, r)
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

	if message, err := validateImportFileHeader(header); err != nil {
		h.Warn("rejected import upload", "error", err)
		http.Error(w, message, http.StatusBadRequest)
		return
	}

	// Optional fields.
	title := r.FormValue("title")
	if title == "" {
		title = header.Filename
	}
	folderParts := parseImportFolderParts(r.FormValue("folder"))

	if isEPUBExt(strings.ToLower(filepath.Ext(header.Filename))) {
		strategy, message, err := parseImportStrategy(r.FormValue("strategy"), h.settings.epubImportStrategy())
		if err != nil {
			h.Warn("rejected import strategy", "error", err)
			http.Error(w, message, http.StatusBadRequest)
			return
		}

		// multipart.File is an io.ReaderAt, so the archive is read randomly
		// rather than buffered whole.
		newBook, err := h.importEPUB(shelfData, f, header.Size, header.Filename, r.FormValue("title"), folderParts, strategy)
		if err != nil {
			h.writeEPUBImportError(w, err)
			return
		}

		h.writeImportedBook(w, newBook, folderParts)
		return
	}

	// Re-encode before creating the book: this reads the upload, and the book
	// initializer runs while the exclusive shelf lock is held.
	utf8File, _, err := util.ReEncodeToUTF8(f)
	if err != nil {
		var unsupported *util.UnsupportedEncodingError
		if errors.As(err, &unsupported) {
			// The file's encoding is the problem, not the server: report it as a
			// client error and name the detected encoding so the user knows why.
			h.Warn("rejected import upload: unsupported encoding", "error", err)
			http.Error(w, "unsupported text encoding: "+unsupported.Encoding, http.StatusBadRequest)
			return
		}
		h.Error("failed to re-encode uploaded file to UTF-8", "error", err)
		http.Error(w, "failed to re-encode uploaded file to UTF-8", http.StatusInternalServerError)
		return
	}

	newBook, err := newPlainTextBook(shelfData, utf8File, folderParts, title, header.Filename)
	if err != nil {
		h.writeErr(w, err, "failed to import book")
		return
	}

	h.writeImportedBook(w, newBook, folderParts)
}

// writeEPUBImportError reports a bad archive with its detail, because the
// client is the only one who can act on it, and maps everything else.
func (h *importHandlers) writeEPUBImportError(w http.ResponseWriter, err error) {
	if isEPUBInputError(err) {
		h.Error("failed to import epub", "error", err)
		http.Error(w, "failed to import epub: "+err.Error(), http.StatusBadRequest)
		return
	}

	h.writeErr(w, err, "failed to import epub")
}

// writeImportedBook responds with the freshly imported book. The book was
// created under folderParts, which is where it now sits; the book itself does
// not carry its folder back.
func (h *importHandlers) writeImportedBook(w http.ResponseWriter, newBook *shelf.Book, folderParts shelf.FolderPath) {
	h.writeJSON(w, http.StatusCreated, Book{
		Meta:  newBook.GetMeta(),
		Folder: folderParts,
	})
}

// fromLocalPath imports a book from a local file path on the server.
// This is intended for desktop application use, where the client can specify a
// local file path and the server can access it directly.
func (h *importHandlers) fromLocalPath(shelfID string, localPath string, folderParts shelf.FolderPath) (*shelf.Book, error) {
	cleanPath, err := validateLocalImportPath(localPath)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	targetShelfID := strings.TrimSpace(shelfID)
	if targetShelfID == "" {
		return nil, util.Errorf("shelf ID cannot be empty")
	}

	shelfData, ok := h.shelves.GetShelf(targetShelfID)
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
		return h.importEPUB(shelfData, fp, info.Size(), filepath.Base(cleanPath), "", folderParts, h.settings.epubImportStrategy())
	}

	// Re-encode before creating the book: this reads the file, and the book
	// initializer runs while the exclusive shelf lock is held.
	utf8Reader, _, err := util.ReEncodeToUTF8(fp)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	// Strip the extension so the desktop local-path import yields "遮天" rather
	// than "遮天.txt", matching the web-upload path (the frontend already sends a
	// de-extensioned title).
	title := deriveTitleFromFilename(filepath.Base(cleanPath))

	newBook, err := newPlainTextBook(shelfData, utf8Reader, folderParts, title, cleanPath)
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
