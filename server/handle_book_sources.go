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

// GET /api/shelves/{shelf_id}/books/{book_id}/sources
func (app *App) HandleAPIGetBookSources(w http.ResponseWriter, r *http.Request) {
	_, book, ok := app.loadBook(w, r)
	if !ok {
		return
	}

	sources, err := book.ListSource()
	if err != nil {
		app.Error("failed to list book sources", "error", err)
		http.Error(w, "failed to list book sources", http.StatusInternalServerError)
		return
	}

	sourceMetas := make([]*shelf.SourceMeta, len(sources))
	for i, s := range sources {
		sourceMetas[i] = s.GetMeta()
	}

	app.writeJSON(w, http.StatusOK, sourceMetas)
}

// GET /api/shelves/{shelf_id}/books/{book_id}/sources/{source_id}
func (app *App) HandleAPIGetBookSource(w http.ResponseWriter, r *http.Request) {
	_, source, ok := app.loadBookSource(w, r)
	if !ok {
		return
	}

	app.writeJSON(w, http.StatusOK, source.GetMeta())
}

// POST /api/shelves/{shelf_id}/books/{book_id}/sources
func (app *App) HandleAPICreateBookSource(w http.ResponseWriter, r *http.Request) {
	_, book, ok := app.loadBook(w, r)
	if !ok {
		return
	}

	if err := book.EnsureWritable(); err != nil {
		app.writeErr(w, err, "failed to create book source")
		return
	}

	var content io.Reader = strings.NewReader("")
	options := shelf.NewSourceOptions{Format: shelf.BookFormatText}
	setCurrent := false

	if strings.HasPrefix(strings.ToLower(r.Header.Get("Content-Type")), "multipart/form-data") {
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
	}

	sourceMeta, err := book.NewSourceWithOptions(content, options)
	if err != nil {
		app.writeErr(w, err, "failed to create book source")
		return
	}
	if setCurrent {
		if err := book.SetCurrentSource(sourceMeta.ID()); err != nil {
			if cleanupErr := book.DeleteSource(sourceMeta.ID()); cleanupErr != nil {
				app.Error("failed to roll back derived source", "source_id", sourceMeta.ID(), "error", cleanupErr)
			}
			app.writeErr(w, err, "failed to activate new book source")
			return
		}
	}

	app.writeJSON(w, http.StatusOK, sourceMeta.GetMeta())
}

// DELETE /api/shelves/{shelf_id}/books/{book_id}/sources/{source_id}
func (app *App) HandleAPIDeleteBookSource(w http.ResponseWriter, r *http.Request) {
	shelfData, ok := app.resolveShelf(w, r)
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

	book, ok := app.getBook(w, shelfData, bookID)
	if !ok {
		return
	}

	// DeleteSource reports a missing source itself, so the source is not
	// loaded up front here.
	if err := book.DeleteSource(sourceID); err != nil {
		app.writeErr(w, err, "failed to delete book source")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// PUT /api/shelves/{shelf_id}/books/{book_id}/sources/{source_id}/current
func (app *App) HandleAPISetCurrentBookSource(w http.ResponseWriter, r *http.Request) {
	book, _, ok := app.loadBookSource(w, r)
	if !ok {
		return
	}

	// loadBookSource has already rejected an unknown source_id.
	sourceID, ok := resolveSourceID(w, r)
	if !ok {
		return
	}

	if err := book.SetCurrentSource(sourceID); err != nil {
		app.writeErr(w, err, "failed to set current book source")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// GET /api/shelves/{shelf_id}/books/{book_id}/sources/{source_id}/content
func (app *App) HandleAPIGetBookSourceContent(w http.ResponseWriter, r *http.Request) {
	_, source, ok := app.loadBookSource(w, r)
	if !ok {
		return
	}

	src, err := source.Open()
	if err != nil {
		app.Error("failed to open book source", "error", err)
		http.Error(w, "failed to open book source", http.StatusInternalServerError)
		return
	}
	defer src.Close()

	app.streamTextFile(w, src, "failed to write book source content")
}

// GET /api/shelves/{shelf_id}/books/{book_id}/sources/{source_id}/assets/{asset_name}
//
// Serves an illustration the source's text references. A plain read, so
// neither the token gate nor read-only mode stands between a reader and it -
// unlike the PUT and DELETE below.
func (app *App) HandleAPIGetBookSourceAsset(w http.ResponseWriter, r *http.Request) {
	_, source, ok := app.loadBookSource(w, r)
	if !ok {
		return
	}

	assetName, ok := resolveAssetName(w, r)
	if !ok {
		return
	}

	if app.serveImageValidator(w, r, source.AssetETag(assetName), cacheRevalidateAlways) {
		return
	}

	asset, err := source.OpenAsset(assetName)
	if err != nil {
		app.writeErr(w, err, "failed to get source asset")
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
		app.Error("failed to write source asset", "error", err, "asset", assetName)
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
func (app *App) HandleAPIUpdateBookSourceAsset(w http.ResponseWriter, r *http.Request) {
	book, source, ok := app.loadBookSource(w, r)
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
		app.writeErr(w, err, "failed to store source asset")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxAssetBodySize)
	data, err := io.ReadAll(r.Body)
	if err != nil {
		if isRequestBodyTooLarge(err) {
			http.Error(w, "request body too large (max 20 MB)", http.StatusRequestEntityTooLarge)
			return
		}
		app.Error("failed to read request body", "error", err)
		http.Error(w, "failed to read request body", http.StatusInternalServerError)
		return
	}

	if err := source.WriteAsset(assetName, data); err != nil {
		app.writeErr(w, err, "failed to store source asset")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// DELETE /api/shelves/{shelf_id}/books/{book_id}/sources/{source_id}/assets/{asset_name}
//
// The source's text is left alone. A link pointing at a removed file renders
// as its alt text, and editing someone's prose to preserve an invariant the
// shelf does not enforce would be the worse trade.
func (app *App) HandleAPIDeleteBookSourceAsset(w http.ResponseWriter, r *http.Request) {
	book, source, ok := app.loadBookSource(w, r)
	if !ok {
		return
	}

	assetName, ok := resolveAssetName(w, r)
	if !ok {
		return
	}

	if err := book.EnsureWritable(); err != nil {
		app.writeErr(w, err, "failed to delete source asset")
		return
	}

	if err := source.DeleteAsset(assetName); err != nil {
		app.writeErr(w, err, "failed to delete source asset")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// POST /api/shelves/{shelf_id}/books/{book_id}/sources/{source_id}/refresh
func (app *App) HandleAPIRefreshBookSourceMeta(w http.ResponseWriter, r *http.Request) {
	_, source, ok := app.loadBookSource(w, r)
	if !ok {
		return
	}

	if err := source.RefreshContentMetadata(); err != nil {
		app.writeErr(w, err, "failed to refresh source metadata")
		return
	}

	app.writeJSON(w, http.StatusOK, source.GetMeta())
}

// PATCH /api/shelves/{shelf_id}/books/{book_id}/sources/{source_id}/content
func (app *App) HandleAPIUpdateBookSourceContent(w http.ResponseWriter, r *http.Request) {
	_, source, ok := app.loadBookSource(w, r)
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
		app.Error("failed to re-encode request body to UTF-8", "error", err)
		http.Error(w, "failed to re-encode request body to UTF-8", http.StatusInternalServerError)
		return
	}

	if err := source.UpdateContent(utf8Reader); err != nil {
		app.writeErr(w, err, "failed to update book source content")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
