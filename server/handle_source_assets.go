package server

import (
	"io"
	"net/http"
	"strconv"
)

// maxAssetBodySize caps an uploaded illustration. A file placed on the shelf by
// hand has no bound - which is why the read path streams rather than buffers -
// but an upload is read into memory to be written, so it needs one.
const maxAssetBodySize = 20 << 20 // 20 MB

// GET /api/shelves/{shelf_id}/books/{book_id}/sources/{source_id}/assets/{asset_name}
//
// Serves an illustration the source's text references. A plain read, so
// neither the token gate nor read-only mode stands between a reader and it -
// unlike the PUT and DELETE below.
func (h *sourceHandlers) getAsset(w http.ResponseWriter, r *http.Request) {
	_, _, source, ok := h.loadBookSource(w, r)
	if !ok {
		return
	}

	assetName, ok := resolveAssetName(w, r)
	if !ok {
		return
	}

	if h.serveImageValidator(w, r, source.AssetETag(assetName), cacheRevalidateAlways) {
		return
	}

	asset, err := source.OpenAsset(assetName)
	if err != nil {
		h.writeErr(w, err, "failed to get source asset")
		return
	}
	defer asset.File.Close()

	w.Header().Set("Content-Type", imageContentTypeForExt(asset.Ext))
	w.Header().Set("Content-Length", strconv.FormatInt(asset.Info.Size(), 10))

	// Commit the response before copying: a zero-byte asset writes nothing, and
	// a copy that writes nothing never commits a status of its own.
	w.WriteHeader(http.StatusOK)

	// A GET pattern also matches HEAD, and net/http discards the body for it.
	// Copying anyway would read the whole asset off the shelf to write it
	// nowhere - and an asset has no size bound, so on an SMB mount that is real
	// I/O for nothing.
	if r.Method == http.MethodHead {
		return
	}

	if _, err := io.Copy(w, asset.File); err != nil {
		h.Error("failed to write source asset", "error", err, "asset", assetName)
	}
}

// PUT /api/shelves/{shelf_id}/books/{book_id}/sources/{source_id}/assets/{asset_name}
//
// Stores an illustration under the name the request addressed, replacing one
// already there. The name carries the format, so the body is written as sent
// and Content-Type is not consulted: the extension is what the read path
// serves by and what shelf validates, and a second opinion could only
// disagree with it.
//
// This is the first route that writes into assets/, so it is a mutating
// request like any other: the token gate and the read-only mode both apply
// before it is reached.
func (h *sourceHandlers) updateAsset(w http.ResponseWriter, r *http.Request) {
	_, book, source, ok := h.loadBookSource(w, r)
	if !ok {
		return
	}

	assetName, ok := resolveAssetName(w, r)
	if !ok {
		return
	}

	// Refuse before reading the body, not after: a book this build must not
	// write should not have an upload spooled for it either.
	if err := book.EnsureWritable(); err != nil {
		h.writeErr(w, err, "failed to store source asset")
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

	if err := source.WriteAsset(assetName, data); err != nil {
		h.writeErr(w, err, "failed to store source asset")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// DELETE /api/shelves/{shelf_id}/books/{book_id}/sources/{source_id}/assets/{asset_name}
//
// The source's text is left alone. A link pointing at a removed file renders
// as its alt text, and editing someone's prose to preserve an invariant the
// shelf does not enforce would be the worse trade.
func (h *sourceHandlers) deleteAsset(w http.ResponseWriter, r *http.Request) {
	_, book, source, ok := h.loadBookSource(w, r)
	if !ok {
		return
	}

	assetName, ok := resolveAssetName(w, r)
	if !ok {
		return
	}

	if err := book.EnsureWritable(); err != nil {
		h.writeErr(w, err, "failed to delete source asset")
		return
	}

	if err := source.DeleteAsset(assetName); err != nil {
		h.writeErr(w, err, "failed to delete source asset")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
