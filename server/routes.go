package server

import (
	"net/http"
)

func (app *App) Serve(mux *http.ServeMux) {
	mux.HandleFunc("GET /health", app.Health)

	// Shelf API

	mux.HandleFunc("GET /api/mode", app.HandleGetMode)
	mux.HandleFunc("GET /api/version", app.HandleGetVersion)
	mux.HandleFunc("GET /api/shelves", app.HandleGetShelves)
	mux.HandleFunc("GET /api/shelves/{shelf_id}/status", app.HandleAPIGetShelfStatus)

	// Book API

	mux.HandleFunc("GET /api/shelves/{shelf_id}/books", app.books.getBooks)
	mux.HandleFunc("POST /api/shelves/{shelf_id}/books", app.books.createBook)
	mux.HandleFunc("POST /api/shelves/{shelf_id}/book-batches", app.HandleAPIBookBatch)
	mux.HandleFunc("POST /api/shelves/{shelf_id}/content-stat-refreshes", app.HandleAPIRefreshContentStats)
	mux.HandleFunc("POST /api/shelves/{shelf_id}/book-cache-exports", app.HandleAPIExportBookCache)

	mux.HandleFunc("POST /api/shelves/{shelf_id}/books/import", app.imports.importBook)
	mux.HandleFunc("GET /api/shelves/{shelf_id}/books/duplicate", app.books.findDuplicateBooks)

	mux.HandleFunc("GET /api/shelves/{shelf_id}/books/{book_id}", app.books.getBook)
	mux.HandleFunc("PATCH /api/shelves/{shelf_id}/books/{book_id}", app.books.updateBook)
	mux.HandleFunc("DELETE /api/shelves/{shelf_id}/books/{book_id}", app.HandleAPITrashBook)
	mux.HandleFunc("POST /api/shelves/{shelf_id}/books/{book_id}/trash", app.HandleAPITrashBook)

	mux.HandleFunc("GET /api/shelves/{shelf_id}/books/{book_id}/sources", app.sources.listSources)
	mux.HandleFunc("GET /api/shelves/{shelf_id}/books/{book_id}/sources/{source_id}", app.sources.getSource)
	mux.HandleFunc("POST /api/shelves/{shelf_id}/books/{book_id}/sources", app.sources.createSource)
	mux.HandleFunc("DELETE /api/shelves/{shelf_id}/books/{book_id}/sources/{source_id}", app.sources.deleteSource)
	mux.HandleFunc("PUT /api/shelves/{shelf_id}/books/{book_id}/sources/{source_id}/current", app.sources.setCurrentSource)
	mux.HandleFunc("GET /api/shelves/{shelf_id}/books/{book_id}/sources/{source_id}/content", app.sources.getSourceContent)
	mux.HandleFunc("PATCH /api/shelves/{shelf_id}/books/{book_id}/sources/{source_id}/content", app.sources.updateSourceContent)
	mux.HandleFunc("POST /api/shelves/{shelf_id}/books/{book_id}/sources/{source_id}/refresh", app.sources.refreshSourceMeta)
	mux.HandleFunc("GET /api/shelves/{shelf_id}/books/{book_id}/sources/{source_id}/assets/{asset_name}", app.sources.getAsset)
	mux.HandleFunc("PUT /api/shelves/{shelf_id}/books/{book_id}/sources/{source_id}/assets/{asset_name}", app.sources.updateAsset)
	mux.HandleFunc("DELETE /api/shelves/{shelf_id}/books/{book_id}/sources/{source_id}/assets/{asset_name}", app.sources.deleteAsset)

	mux.HandleFunc("GET /api/shelves/{shelf_id}/books/{book_id}/cover", app.books.getCover)
	mux.HandleFunc("PUT /api/shelves/{shelf_id}/books/{book_id}/cover", app.books.updateCover)
	mux.HandleFunc("DELETE /api/shelves/{shelf_id}/books/{book_id}/cover", app.books.deleteCover)

	mux.HandleFunc("GET /api/shelves/{shelf_id}/books/{book_id}/content", app.books.getBookContent)
	mux.HandleFunc("GET /api/shelves/{shelf_id}/books/{book_id}/split_config", app.books.getSplitConfig)
	mux.HandleFunc("PATCH /api/shelves/{shelf_id}/books/{book_id}/split_config", app.books.updateSplitConfig)

	mux.HandleFunc("GET /api/shelves/{shelf_id}/trash/books", app.HandleAPIGetTrashedBooks)
	mux.HandleFunc("POST /api/shelves/{shelf_id}/trash/empty", app.HandleAPIEmptyTrash)
	mux.HandleFunc("POST /api/shelves/{shelf_id}/trash/books/{book_id}/restore", app.HandleAPIRestoreTrashedBook)
	mux.HandleFunc("DELETE /api/shelves/{shelf_id}/trash/books/{book_id}", app.HandleAPIDeleteTrashedBook)

	mux.HandleFunc("GET /api/shelves/{shelf_id}/layers", app.HandleAPIGetLayers)
	mux.HandleFunc("POST /api/shelves/{shelf_id}/layer-moves", app.HandleAPIMoveLayer)
	mux.HandleFunc("POST /api/shelves/{shelf_id}/layers/{layer_path...}", app.HandleAPICreateLayer)
	mux.HandleFunc("PATCH /api/shelves/{shelf_id}/layers/{layer_path...}", app.HandleAPIRenameLayer)
	mux.HandleFunc("DELETE /api/shelves/{shelf_id}/layers/{layer_path...}", app.HandleAPIDeleteLayer)

	// Task API

	mux.HandleFunc("GET /api/taskchains/{taskchain_id}", app.HandleAPIGetTaskChain)

	// Log API

	mux.HandleFunc("GET /api/logs", app.HandleAPIGetLogs)
	mux.HandleFunc("GET /api/logs/{log_id}/content", app.HandleAPIGetLogContent)

	// Setting API

	mux.HandleFunc("GET /api/setting/cover_to_jpg", app.HandleGetSettingCoverToJPG)
	mux.HandleFunc("POST /api/setting/cover_to_jpg", app.HandleSetSettingCoverToJPG)
	mux.HandleFunc("DELETE /api/setting/cover_to_jpg", app.HandleDeleteSettingCoverToJPG)
	mux.HandleFunc("GET /api/setting/default_split_config", app.HandleGetSettingDefaultSplitConfig)
	mux.HandleFunc("POST /api/setting/default_split_config", app.HandleSetSettingDefaultSplitConfig)
	mux.HandleFunc("DELETE /api/setting/default_split_config", app.HandleDeleteSettingDefaultSplitConfig)
	mux.HandleFunc("GET /api/setting/epub_import_strategy", app.HandleGetSettingEPUBImportStrategy)
	mux.HandleFunc("POST /api/setting/epub_import_strategy", app.HandleSetSettingEPUBImportStrategy)
	mux.HandleFunc("DELETE /api/setting/epub_import_strategy", app.HandleDeleteSettingEPUBImportStrategy)

	// Unknown API paths must not fall through to the SPA index.
	mux.HandleFunc("GET /api/{path...}", http.NotFound)

	mux.HandleFunc("GET /{path...}", app.HandleSPAFallback)
}
