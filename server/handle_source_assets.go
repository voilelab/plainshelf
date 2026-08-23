package server

import (
	"archive/zip"
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/voilelab/plainshelf/shelf"
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

// GET /api/shelves/{shelf_id}/books/{book_id}/sources/{source_id}/assets.zip
//
// Packs the source's illustrations into one zip so a download fetches them in a
// single request instead of one per file — the round trips, not the bytes, are
// what a high-latency mobile connection pays for. The path is assets.zip, not a
// child of assets/, so it cannot collide with the assets/{asset_name} route.
//
// A plain GET, so it inherits getAsset's security exactly: gated only under
// protect_read. The repeated `name` query parameter picks which files to pack
// (none = the whole assets/ directory); each is validated like getAsset's path
// segment, so an unsafe name is a 400 before any byte is written, while an
// absent one is skipped rather than failing the request.
func (h *sourceHandlers) getAssetsBundle(w http.ResponseWriter, r *http.Request) {
	_, _, source, ok := h.loadBookSource(w, r)
	if !ok {
		return
	}

	names, ok := h.resolveBundleAssetNames(w, source, r)
	if !ok {
		return
	}

	w.Header().Set("Content-Type", "application/zip")
	w.WriteHeader(http.StatusOK)

	// A GET pattern also matches HEAD, and net/http discards the body for it, so
	// building the archive would read every asset off the shelf to write it
	// nowhere.
	if r.Method == http.MethodHead {
		return
	}

	zw := zip.NewWriter(w)
	for _, name := range names {
		if err := h.writeAssetToZip(zw, source, name); err != nil {
			// The 200 and the archive so far are already on the wire, so the
			// status cannot change now. Stop and log: the client receives a
			// truncated zip, which its unzip rejects, and falls back to
			// per-file fetches.
			h.Error("failed to add source asset to bundle", "error", err, "asset", name)
			return
		}
	}
	if err := zw.Close(); err != nil {
		h.Error("failed to finalize source asset bundle", "error", err)
	}
}

// resolveBundleAssetNames returns the names to pack: the whole assets/ directory
// when none were requested, otherwise the requested names validated up front —
// before the archive's first byte — so an unsafe name is a 400, not a truncated
// 200. A valid-but-absent name stays in the list and is skipped while packing.
func (h *sourceHandlers) resolveBundleAssetNames(w http.ResponseWriter, source *shelf.Source, r *http.Request) ([]string, bool) {
	requested := r.URL.Query()["name"]
	if len(requested) == 0 {
		names, err := source.ListAssets()
		if err != nil {
			h.writeErr(w, err, "failed to list source assets")
			return nil, false
		}
		return names, true
	}

	for _, name := range requested {
		if _, err := source.AssetPath(name); err != nil {
			h.writeErr(w, err, "failed to bundle source assets")
			return nil, false
		}
	}
	return requested, true
}

// writeAssetToZip adds one illustration to the archive under its flat name. A
// named file that is not on the shelf is skipped, not fatal: the text may
// reference an image never uploaded, and the reader falls back to alt text.
func (h *sourceHandlers) writeAssetToZip(zw *zip.Writer, source *shelf.Source, name string) error {
	asset, err := source.OpenAsset(name)
	if err != nil {
		if errors.Is(err, shelf.ErrAssetNotFound) {
			return nil
		}
		return err
	}
	defer asset.File.Close()

	// Store, not Deflate: jpeg/png/webp are already compressed, so the endpoint's
	// gain is the request count, not bytes saved.
	entry, err := zw.CreateHeader(&zip.FileHeader{
		Name:     name,
		Method:   zip.Store,
		Modified: asset.Info.ModTime(),
	})
	if err != nil {
		return err
	}
	if _, err := io.Copy(entry, asset.File); err != nil {
		return err
	}
	return nil
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
