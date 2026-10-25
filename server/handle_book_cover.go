package server

import (
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/voilelab/plainshelf/internal/imgutil"
)

const maxCoverBodySize = 20 << 20

func (h *bookHandlers) getCover(w http.ResponseWriter, r *http.Request) {
	_, book, ok := h.loadBook(w, r)
	if !ok {
		return
	}

	if h.serveImageValidator(w, r, book.CoverETag(), cacheFreshForOneDay) {
		return
	}

	coverData, ext, err := book.OpenCover()
	if err != nil {

		h.writeErr(w, r, err, "failed to get book cover")
		return
	}

	if coverData == nil {
		http.Error(w, "cover not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", imageContentTypeForExt(ext))
	w.Write(coverData)
}

func (h *bookHandlers) updateCover(w http.ResponseWriter, r *http.Request) {
	_, book, ok := h.loadBook(w, r)
	if !ok {
		return
	}

	contentType := r.Header.Get("Content-Type")
	coverToJPG := h.settings.coverToJPG()
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
		h.Error("failed to read request body", "error", err)
		http.Error(w, "failed to read request body", http.StatusInternalServerError)
		return
	}

	if coverToJPG {
		data, err = imgutil.AnyToJPG(data)
		if errors.Is(err, imgutil.ErrTooManyPixels) {
			http.Error(w, fmt.Sprintf("image too large (max %d megapixels)", imgutil.MaxPixels/1_000_000), http.StatusRequestEntityTooLarge)
			return
		}
		if err != nil {
			h.Error("failed to convert image to JPEG", "error", err)
			http.Error(w, "failed to convert image to JPEG", http.StatusInternalServerError)
			return
		}
		ext = ".jpg"
	}

	err = book.SetCover(data, ext)
	if err != nil {
		h.writeErr(w, r, err, "failed to update book cover")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *bookHandlers) deleteCover(w http.ResponseWriter, r *http.Request) {
	_, book, ok := h.loadBook(w, r)
	if !ok {
		return
	}

	if err := book.DeleteCover(); err != nil {
		h.writeErr(w, r, err, "failed to delete book cover")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
