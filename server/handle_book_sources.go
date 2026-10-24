package server

import (
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"

	"github.com/voilelab/plainshelf/internal/util"
	"github.com/voilelab/plainshelf/shelf"
)

type sourceHandlers struct {
	*apiCore
}

type createSourceJSONRequest struct {
	Content    string `json:"content"`
	Format     string `json:"format"`
	Comment    string `json:"comment"`
	SetCurrent bool   `json:"set_current"`
}

func (h *sourceHandlers) listSources(w http.ResponseWriter, r *http.Request) {
	_, book, ok := h.loadBook(w, r)
	if !ok {
		return
	}

	sources, err := book.ListSource()
	if err != nil {
		h.Error("failed to list book sources", "error", err)
		http.Error(w, "failed to list book sources", http.StatusInternalServerError)
		return
	}

	sourceMetas := make([]*shelf.SourceMeta, len(sources))
	for i, s := range sources {
		sourceMetas[i] = s.GetMeta()
	}

	h.writeJSON(w, http.StatusOK, sourceMetas)
}

func (h *sourceHandlers) getSource(w http.ResponseWriter, r *http.Request) {
	_, _, source, ok := h.loadBookSource(w, r)
	if !ok {
		return
	}

	h.writeJSON(w, http.StatusOK, source.GetMeta())
}

func (h *sourceHandlers) createSource(w http.ResponseWriter, r *http.Request) {
	shelfData, book, ok := h.loadBook(w, r)
	if !ok {
		return
	}

	if err := book.EnsureWritable(); err != nil {
		h.writeErr(w, r, err, "failed to create book source")
		return
	}

	var content io.Reader = strings.NewReader("")
	options := shelf.NewSourceOptions{Format: shelf.BookFormatText}
	setCurrent := false

	contentType := strings.ToLower(r.Header.Get("Content-Type"))
	if strings.HasPrefix(contentType, "multipart/form-data") {
		r.Body = http.MaxBytesReader(w, r.Body, maxImportBodySize)
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			if isRequestBodyTooLarge(err) {
				http.Error(w, "request body too large (max 100 MB)", http.StatusRequestEntityTooLarge)
				return
			}
			http.Error(w, "invalid multipart form: "+err.Error(), http.StatusBadRequest)
			return
		}
		if r.MultipartForm != nil {
			defer func() {
				if err := r.MultipartForm.RemoveAll(); err != nil {
					h.Warn("failed to remove multipart temporary files", "error", err)
				}
			}()
		}

		file, _, err := r.FormFile("content")
		if err == nil {
			defer file.Close()

			content = file
		} else if errors.Is(err, multipart.ErrMessageTooLarge) {
			http.Error(w, "source content too large", http.StatusRequestEntityTooLarge)
			return
		} else if !errors.Is(err, http.ErrMissingFile) {
			http.Error(w, "invalid source content", http.StatusBadRequest)
			return
		} else if text := r.FormValue("content"); text != "" {
			content = strings.NewReader(text)
		}

		if format := strings.TrimSpace(r.FormValue("format")); format != "" {
			options.Format = format
		}
		options.Comment = r.FormValue("comment")
		if raw := strings.TrimSpace(r.FormValue("set_current")); raw != "" {
			parsed, err := strconv.ParseBool(raw)
			if err != nil {
				http.Error(w, "set_current must be true or false", http.StatusBadRequest)
				return
			}
			setCurrent = parsed
		}
	} else if strings.HasPrefix(contentType, "application/json") {
		r.Body = http.MaxBytesReader(w, r.Body, maxImportBodySize)
		var request createSourceJSONRequest
		if !decodeStrictJSON(w, r, &request) {
			return
		}

		content = strings.NewReader(request.Content)
		if format := strings.TrimSpace(request.Format); format != "" {
			options.Format = format
		}
		options.Comment = request.Comment
		setCurrent = request.SetCurrent
	}

	sourceMeta, err := book.NewSourceWithOptions(content, options)
	if err != nil {
		h.writeErr(w, r, err, "failed to create book source")
		return
	}
	if setCurrent {
		if err := book.SetCurrentSource(sourceMeta.ID()); err != nil {
			if cleanupErr := book.DeleteSource(sourceMeta.ID()); cleanupErr != nil {
				h.Error("failed to roll back derived source", "source_id", sourceMeta.ID(), "error", cleanupErr)
			}
			h.writeErr(w, r, err, "failed to activate new book source")
			return
		}
		shelfData.RefreshBookCharCount(book.ID())
	}

	h.writeJSON(w, http.StatusOK, sourceMeta.GetMeta())
}

func (h *sourceHandlers) deleteSource(w http.ResponseWriter, r *http.Request) {
	shelfData, ok := h.resolveShelf(w, r)
	if !ok {
		return
	}

	bookID, ok := resolveBookID(w, r)
	if !ok {
		return
	}

	sourceID, ok := resolveSourceID(w, r)
	if !ok {
		return
	}

	book, ok := h.lookupVisibleBook(w, r, shelfData, bookID)
	if !ok {
		return
	}

	if err := book.DeleteSource(sourceID); err != nil {
		h.writeErr(w, r, err, "failed to delete book source")
		return
	}

	shelfData.RefreshBookCharCount(book.ID())

	w.WriteHeader(http.StatusNoContent)
}

func (h *sourceHandlers) deleteSourceComment(w http.ResponseWriter, r *http.Request) {
	_, _, source, ok := h.loadBookSource(w, r)
	if !ok {
		return
	}

	if err := source.UpdateComment(""); err != nil {
		h.writeErr(w, r, err, "failed to delete book source comment")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *sourceHandlers) setCurrentSource(w http.ResponseWriter, r *http.Request) {
	shelfData, book, _, ok := h.loadBookSource(w, r)
	if !ok {
		return
	}

	sourceID, ok := resolveSourceID(w, r)
	if !ok {
		return
	}

	if err := book.SetCurrentSource(sourceID); err != nil {
		h.writeErr(w, r, err, "failed to set current book source")
		return
	}

	shelfData.RefreshBookCharCount(book.ID())

	w.WriteHeader(http.StatusNoContent)
}

func (h *sourceHandlers) getSourceContent(w http.ResponseWriter, r *http.Request) {
	_, _, source, ok := h.loadBookSource(w, r)
	if !ok {
		return
	}

	src, err := source.Open()
	if err != nil {
		h.Error("failed to open book source", "error", err)
		http.Error(w, "failed to open book source", http.StatusInternalServerError)
		return
	}
	defer src.Close()

	h.streamTextFile(w, src, "failed to write book source content")
}

func (h *sourceHandlers) refreshSourceMeta(w http.ResponseWriter, r *http.Request) {
	shelfData, book, source, ok := h.loadBookSource(w, r)
	if !ok {
		return
	}

	if err := source.RefreshContentMetadata(); err != nil {
		h.writeErr(w, r, err, "failed to refresh source metadata")
		return
	}

	shelfData.RefreshBookCharCount(book.ID())

	h.writeJSON(w, http.StatusOK, source.GetMeta())
}

func (h *sourceHandlers) updateSourceContent(w http.ResponseWriter, r *http.Request) {
	shelfData, book, source, ok := h.loadBookSource(w, r)
	if !ok {
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxImportBodySize)
	utf8Reader, _, err := util.ReEncodeToUTF8(r.Body)
	if err != nil {
		if bodyExceededLimit(r.Body, err) {
			http.Error(w, "request body too large (max 100 MB)", http.StatusRequestEntityTooLarge)
			return
		}
		h.Error("failed to re-encode request body to UTF-8", "error", err)
		http.Error(w, "failed to re-encode request body to UTF-8", http.StatusInternalServerError)
		return
	}

	if err := source.UpdateContent(utf8Reader); err != nil {
		if isRequestBodyTooLarge(err) {
			http.Error(w, "request body too large (max 100 MB)", http.StatusRequestEntityTooLarge)
			return
		}
		h.writeErr(w, r, err, "failed to update book source content")
		return
	}

	shelfData.RefreshBookCharCount(book.ID())

	w.WriteHeader(http.StatusNoContent)
}

func bodyExceededLimit(body io.Reader, readErr error) bool {
	if isRequestBodyTooLarge(readErr) {
		return true
	}
	_, drainErr := io.Copy(io.Discard, body)
	return isRequestBodyTooLarge(drainErr)
}
