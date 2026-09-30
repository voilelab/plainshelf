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

// sourceHandlers serves the source routes: the alternative texts a book can
// hold, their content, and the illustrations stored beside them.
type sourceHandlers struct {
	*apiCore
}

type createSourceJSONRequest struct {
	Content    string `json:"content"`
	Format     string `json:"format"`
	Comment    string `json:"comment"`
	SetCurrent bool   `json:"set_current"`
}

// GET /api/shelves/{shelf_id}/books/{book_id}/sources
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

// GET /api/shelves/{shelf_id}/books/{book_id}/sources/{source_id}
func (h *sourceHandlers) getSource(w http.ResponseWriter, r *http.Request) {
	_, _, source, ok := h.loadBookSource(w, r)
	if !ok {
		return
	}

	h.writeJSON(w, http.StatusOK, source.GetMeta())
}

// POST /api/shelves/{shelf_id}/books/{book_id}/sources
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
			defer r.MultipartForm.RemoveAll() //nolint:errcheck // request temp cleanup
		}

		file, _, err := r.FormFile("content")
		if err == nil {
			defer file.Close()
			// The derived-source API receives the editor's canonical UTF-8 draft.
			// Do not run heuristic encoding detection here: short Unicode content
			// can be misidentified, and unlike file import there is no unknown
			// external encoding to recover.
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

// DELETE /api/shelves/{shelf_id}/books/{book_id}/sources/{source_id}
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

	book, ok := h.lookupBook(w, r, shelfData, bookID)
	if !ok {
		return
	}

	// DeleteSource reports a missing source itself, so the source is not
	// loaded up front here.
	if err := book.DeleteSource(sourceID); err != nil {
		h.writeErr(w, r, err, "failed to delete book source")
		return
	}

	// Deleting the current source hands the pointer to another one, so the
	// cached character count is now somebody else's.
	shelfData.RefreshBookCharCount(book.ID())

	w.WriteHeader(http.StatusNoContent)
}

// DELETE /api/shelves/{shelf_id}/books/{book_id}/sources/{source_id}/comment
//
// Clears the source's comment. The note is written by an import or a conversion
// to record where this text came from, never by the user, so the only thing a
// client can do to it is remove one it no longer wants on the book page. There
// is deliberately no way to rewrite it: an editable note would no longer be a
// record of what actually happened.
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

// PUT /api/shelves/{shelf_id}/books/{book_id}/sources/{source_id}/current
func (h *sourceHandlers) setCurrentSource(w http.ResponseWriter, r *http.Request) {
	shelfData, book, _, ok := h.loadBookSource(w, r)
	if !ok {
		return
	}

	// loadBookSource has already rejected an unknown source_id.
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

// GET /api/shelves/{shelf_id}/books/{book_id}/sources/{source_id}/content
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

// POST /api/shelves/{shelf_id}/books/{book_id}/sources/{source_id}/refresh
func (h *sourceHandlers) refreshSourceMeta(w http.ResponseWriter, r *http.Request) {
	shelfData, book, source, ok := h.loadBookSource(w, r)
	if !ok {
		return
	}

	if err := source.RefreshContentMetadata(); err != nil {
		h.writeErr(w, r, err, "failed to refresh source metadata")
		return
	}

	// Recomputed unconditionally rather than only for the current source: one
	// meta.json read is cheaper than deciding, and a listing that reports a
	// count this request just changed must not report the previous one.
	shelfData.RefreshBookCharCount(book.ID())

	h.writeJSON(w, http.StatusOK, source.GetMeta())
}

// PATCH /api/shelves/{shelf_id}/books/{book_id}/sources/{source_id}/content
func (h *sourceHandlers) updateSourceContent(w http.ResponseWriter, r *http.Request) {
	shelfData, book, source, ok := h.loadBookSource(w, r)
	if !ok {
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxImportBodySize)
	utf8Reader, _, err := util.ReEncodeToUTF8(r.Body)
	if err != nil {
		if isRequestBodyTooLarge(err) {
			http.Error(w, "request body too large (max 100 MB)", http.StatusRequestEntityTooLarge)
			return
		}
		h.Error("failed to re-encode request body to UTF-8", "error", err)
		http.Error(w, "failed to re-encode request body to UTF-8", http.StatusInternalServerError)
		return
	}

	if err := source.UpdateContent(utf8Reader); err != nil {
		h.writeErr(w, r, err, "failed to update book source content")
		return
	}

	shelfData.RefreshBookCharCount(book.ID())

	w.WriteHeader(http.StatusNoContent)
}
