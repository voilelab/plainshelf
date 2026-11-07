package server

import (
	"bytes"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"strconv"

	_ "golang.org/x/image/webp"

	"github.com/voilelab/plainshelf/shelf"
)

const maxPagePixels = 100_000_000

const maxPageListRequestSize = 2 << 20

var pageImageExts = map[string]string{
	"jpeg": ".jpg",
	"png":  ".png",
	"gif":  ".gif",
	"webp": ".webp",
}

type replacePagesRequest struct {
	Pages   []string `json:"pages"`
	MD5Hash string   `json:"md5_hash"`
}

type pageDirectionRequest struct {
	PageDirection string `json:"page_direction"`
}

func (h *sourceHandlers) replacePages(w http.ResponseWriter, r *http.Request) {
	source, ok := h.loadWritableSource(w, r, "failed to reorder source pages")
	if !ok {
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxPageListRequestSize)
	var request replacePagesRequest
	if !decodeStrictJSON(w, r, &request) {
		return
	}
	if !requirePageListMD5(w, request.MD5Hash) {
		return
	}

	h.pageEdits.Lock()
	defer h.pageEdits.Unlock()

	pages, err := source.ReplacePages(request.MD5Hash, request.Pages)
	h.writePageEdit(w, r, pages, err, "failed to reorder source pages")
}

func (h *sourceHandlers) insertPage(w http.ResponseWriter, r *http.Request) {
	source, ok := h.loadWritableSource(w, r, "failed to add source page")
	if !ok {
		return
	}

	query := r.URL.Query()
	md5Hash := query.Get("md5_hash")
	if !requirePageListMD5(w, md5Hash) {
		return
	}
	position, err := strconv.Atoi(query.Get("position"))
	if err != nil {
		http.Error(w, "position must be an integer", http.StatusBadRequest)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxAssetBodySize)
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

	ext, status, err := pageImageExt(data)
	if err != nil {
		http.Error(w, err.Error(), status)
		return
	}

	h.pageEdits.Lock()
	defer h.pageEdits.Unlock()

	pages, err := source.InsertPage(md5Hash, position, data, ext)
	h.writePageEdit(w, r, pages, err, "failed to add source page")
}

func (h *sourceHandlers) deletePage(w http.ResponseWriter, r *http.Request) {
	source, ok := h.loadWritableSource(w, r, "failed to delete source page")
	if !ok {
		return
	}

	assetName, ok := resolveAssetName(w, r)
	if !ok {
		return
	}
	md5Hash := r.URL.Query().Get("md5_hash")
	if !requirePageListMD5(w, md5Hash) {
		return
	}

	h.pageEdits.Lock()
	defer h.pageEdits.Unlock()

	pages, err := source.DeletePage(md5Hash, assetName)
	h.writePageEdit(w, r, pages, err, "failed to delete source page")
}

func (h *sourceHandlers) setPageDirection(w http.ResponseWriter, r *http.Request) {
	source, ok := h.loadWritableSource(w, r, "failed to set page direction")
	if !ok {
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxPageListRequestSize)
	var request pageDirectionRequest
	if !decodeStrictJSON(w, r, &request) {
		return
	}

	if err := source.SetPageDirection(request.PageDirection); err != nil {
		h.writeErr(w, r, err, "failed to set page direction")
		return
	}

	h.writeJSON(w, http.StatusOK, source.GetMeta())
}

func (h *sourceHandlers) loadWritableSource(w http.ResponseWriter, r *http.Request, failure string) (*shelf.Source, bool) {
	_, book, source, ok := h.loadBookSource(w, r)
	if !ok {
		return nil, false
	}
	if err := book.EnsureWritable(); err != nil {
		h.writeErr(w, r, err, failure)
		return nil, false
	}
	return source, true
}

func (h *sourceHandlers) writePageEdit(w http.ResponseWriter, r *http.Request, pages *shelf.PageList, err error, failure string) {
	if err != nil {
		h.writeErr(w, r, err, failure)
		return
	}
	h.writeJSON(w, http.StatusOK, pages)
}

func requirePageListMD5(w http.ResponseWriter, md5Hash string) bool {
	if md5Hash == "" {
		http.Error(w, "md5_hash of the page list being edited is required", http.StatusBadRequest)
		return false
	}
	return true
}

func pageImageExt(data []byte) (string, int, error) {
	config, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return "", http.StatusBadRequest, fmt.Errorf("not a supported image (jpeg, png, gif or webp)")
	}
	ext, ok := pageImageExts[format]
	if !ok {
		return "", http.StatusBadRequest, fmt.Errorf("not a supported image (jpeg, png, gif or webp)")
	}
	if config.Width <= 0 || config.Height <= 0 || uint64(config.Width) > maxPagePixels/uint64(config.Height) {
		return "", http.StatusRequestEntityTooLarge, fmt.Errorf("image too large (max %d megapixels)", maxPagePixels/1_000_000)
	}
	return ext, http.StatusOK, nil
}
