package server

import (
	"net/http"
	"time"

	"github.com/voilelab/plainshelf/shelf"
)

type downloadHandlers struct {
	*apiCore
}

type DownloadResponse struct {
	Book         Book      `json:"book"`
	DownloadedAt time.Time `json:"downloaded_at"`
	SizeBytes    int64     `json:"size_bytes"`
}

type DownloadsResponse struct {
	Books      []DownloadResponse `json:"books"`
	TotalBytes int64              `json:"total_bytes"`
}

func newDownloadResponse(shelfData *shelf.ShelfData, d shelf.Download) DownloadResponse {
	return DownloadResponse{
		Book:         newBookResponse(shelfData, d.Meta, d.Folder),
		DownloadedAt: d.DownloadedAt.UTC(),
		SizeBytes:    d.SizeBytes,
	}
}

func (h *downloadHandlers) downloadBook(w http.ResponseWriter, r *http.Request) {
	shelfData, ok := h.resolveShelf(w, r)
	if !ok {
		return
	}
	bookID, err := readBookID(r)
	if err != nil {
		http.Error(w, "invalid book_id", http.StatusBadRequest)
		return
	}

	download, err := shelfData.DownloadBook(bookID)
	if err != nil {
		h.writeErr(w, r, err, "failed to download book")
		return
	}
	h.writeJSON(w, http.StatusOK, newDownloadResponse(shelfData, download))
}

func (h *downloadHandlers) removeDownload(w http.ResponseWriter, r *http.Request) {
	shelfData, ok := h.resolveShelf(w, r)
	if !ok {
		return
	}
	bookID, err := readBookID(r)
	if err != nil {
		http.Error(w, "invalid book_id", http.StatusBadRequest)
		return
	}

	if err := shelfData.RemoveDownload(bookID); err != nil {
		h.writeErr(w, r, err, "failed to remove download")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *downloadHandlers) getDownloads(w http.ResponseWriter, r *http.Request) {
	shelfData, ok := h.resolveShelf(w, r)
	if !ok {
		return
	}
	if !shelfData.CanDownload() {
		h.writeErr(w, r, shelf.ErrDownloadUnsupported, "failed to list downloads")
		return
	}

	resp := DownloadsResponse{Books: []DownloadResponse{}}
	for _, d := range shelfData.Downloads() {
		resp.Books = append(resp.Books, newDownloadResponse(shelfData, d))
		resp.TotalBytes += d.SizeBytes
	}
	h.writeJSON(w, http.StatusOK, resp)
}
