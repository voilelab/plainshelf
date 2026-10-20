package server

import (
	"archive/zip"
	"io"
	"net/http"
	"strconv"

	"github.com/voilelab/plainshelf/shelf"
)

const maxAssetBodySize = 20 << 20

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
		h.writeErr(w, r, err, "failed to get source asset")
		return
	}
	defer asset.File.Close()

	w.Header().Set("Content-Type", imageContentTypeForExt(asset.Ext))
	w.Header().Set("Content-Length", strconv.FormatInt(asset.Info.Size(), 10))

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
	if err := source.WriteAssetsToZip(zw, names); err != nil {

		h.Error("failed to add source assets to bundle", "error", err)
		return
	}
	if err := zw.Close(); err != nil {
		h.Error("failed to finalize source asset bundle", "error", err)
	}
}

func (h *sourceHandlers) resolveBundleAssetNames(w http.ResponseWriter, source *shelf.Source, r *http.Request) ([]string, bool) {
	requested := r.URL.Query()["name"]
	if len(requested) == 0 {
		names, err := source.ListAssets()
		if err != nil {
			h.writeErr(w, r, err, "failed to list source assets")
			return nil, false
		}
		return names, true
	}

	for _, name := range requested {
		if _, err := source.AssetPath(name); err != nil {
			h.writeErr(w, r, err, "failed to bundle source assets")
			return nil, false
		}
	}
	return requested, true
}

func (h *sourceHandlers) updateAsset(w http.ResponseWriter, r *http.Request) {
	_, book, source, ok := h.loadBookSource(w, r)
	if !ok {
		return
	}

	assetName, ok := resolveAssetName(w, r)
	if !ok {
		return
	}

	if err := book.EnsureWritable(); err != nil {
		h.writeErr(w, r, err, "failed to store source asset")
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
		h.writeErr(w, r, err, "failed to store source asset")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

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
		h.writeErr(w, r, err, "failed to delete source asset")
		return
	}

	if err := source.DeleteAsset(assetName); err != nil {
		h.writeErr(w, r, err, "failed to delete source asset")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
