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

const maxImportBodySize = 100 << 20

const importTextMediaType = "text/plain"
const importMarkdownMediaType = "text/markdown"
const importXMarkdownMediaType = "text/x-markdown"
const importOctetStreamMediaType = "application/octet-stream"
const importEPUBMediaType = "application/epub+zip"
const importZipMediaType = "application/zip"

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

func isSupportedImportExt(ext string) bool {
	return ext == ".txt" || ext == ".md" || ext == ".epub"
}

func isEPUBExt(ext string) bool {
	return ext == ".epub"
}

func deriveTitleFromFilename(filename string) string {
	base := filepath.Base(filename)
	title := strings.TrimSuffix(base, filepath.Ext(base))
	if title == "" {
		return base
	}
	return title
}

func bookFormatFromFilename(filename string) string {
	if strings.ToLower(filepath.Ext(filename)) == ".md" {
		return shelf.BookFormatMarkdown
	}
	return shelf.BookFormatText
}

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

func (h *importHandlers) importBook(w http.ResponseWriter, r *http.Request) {
	shelfData, ok := h.resolveShelf(w, r)
	if !ok {
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxImportBodySize)

	if err := r.ParseMultipartForm(32 << 20); err != nil {
		if isRequestBodyTooLarge(err) {
			http.Error(w, "request body too large (max 100 MB)", http.StatusRequestEntityTooLarge)
			return
		}
		http.Error(w, "invalid multipart form: "+err.Error(), http.StatusBadRequest)
		return
	}

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

		newBook, err := h.importEPUB(shelfData, f, header.Size, header.Filename, r.FormValue("title"), folderParts, strategy)
		if err != nil {
			h.writeEPUBImportError(w, r, err)
			return
		}

		h.writeImportedBook(w, shelfData, newBook, folderParts)
		return
	}

	utf8File, _, err := util.ReEncodeToUTF8(f)
	if err != nil {
		if unsupported, ok := errors.AsType[*util.UnsupportedEncodingError](err); ok {

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
		h.writeErr(w, r, err, "failed to import book")
		return
	}

	h.writeImportedBook(w, shelfData, newBook, folderParts)
}

func (h *importHandlers) writeEPUBImportError(w http.ResponseWriter, r *http.Request, err error) {
	if isEPUBInputError(err) {
		h.Error("failed to import epub", "error", err)
		http.Error(w, "failed to import epub: "+err.Error(), http.StatusBadRequest)
		return
	}

	h.writeErr(w, r, err, "failed to import epub")
}

func (h *importHandlers) writeImportedBook(w http.ResponseWriter, shelfData *shelf.ShelfData, newBook *shelf.Book, folderParts shelf.FolderPath) {
	h.writeJSON(w, http.StatusCreated, newBookResponse(shelfData, newBook.GetMeta(), folderParts))
}

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

		return h.importEPUB(shelfData, fp, info.Size(), filepath.Base(cleanPath), "", folderParts, h.settings.epubImportStrategy())
	}

	utf8Reader, _, err := util.ReEncodeToUTF8(fp)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	title := deriveTitleFromFilename(cleanPath)

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
