package server

import (
	"io/fs"
	"net/http"

	"github.com/voilelab/plainshelf/internal/logutil"
	"github.com/voilelab/plainshelf/internal/taskutil"
	"github.com/voilelab/plainshelf/server/store"
	"github.com/voilelab/plainshelf/shelf"
)

// apiHandlers is everything that answers a request, assembled from the three
// shared components. App owns the lifecycle of what these read; this owns the
// routing.
type apiHandlers struct {
	core     *apiCore
	settings *settings
	tasks    *taskSubmitter

	books   *bookHandlers
	sources *sourceHandlers
	imports *importHandlers
	layers  *layerHandlers
	trash   *trashHandlers
	batches *batchHandlers
	taskAPI *taskHandlers
	setting *settingHandlers
	logs    *logHandlers
	shelves *shelfHandlers
	meta    *metaHandlers
	spa     *spaHandlers
}

func newAPIHandlers(
	logger *logutil.Logger,
	shelfManager *shelf.ShelfManager,
	security *Security,
	storeDB *store.DB,
	pool taskutil.Pool,
	spaFS fs.FS,
	conf *AppConf,
) *apiHandlers {
	core := &apiCore{Logger: logger, shelves: shelfManager, security: security}
	settingsSvc := &settings{Logger: logger, db: storeDB, conf: conf}
	tasks := &taskSubmitter{apiCore: core, pool: pool}

	return &apiHandlers{
		core:     core,
		settings: settingsSvc,
		tasks:    tasks,

		books:   &bookHandlers{apiCore: core, settings: settingsSvc},
		sources: &sourceHandlers{apiCore: core},
		imports: &importHandlers{apiCore: core, settings: settingsSvc},
		layers:  &layerHandlers{apiCore: core},
		trash:   &trashHandlers{taskSubmitter: tasks},
		batches: &batchHandlers{taskSubmitter: tasks},
		taskAPI: &taskHandlers{taskSubmitter: tasks},
		setting: &settingHandlers{apiCore: core, settings: settingsSvc},
		logs:    &logHandlers{apiCore: core, conf: conf},
		shelves: &shelfHandlers{apiCore: core},
		meta:    &metaHandlers{apiCore: core, conf: conf},
		spa:     &spaHandlers{fs: spaFS, files: http.FileServerFS(spaFS), security: security},
	}
}

// serve is the one place the whole HTTP surface is written down. The patterns
// stay literal: the frontend and the end-to-end tests find a route by searching
// for its path.
//
// mode decides how much of it is mounted. A reader server stops after the
// reading routes — not because the rest would be refused anyway (read-only mode
// already does that for writes), but because a binary whose whole purpose is to
// read a shelf has no reason to expose the log, setting, task, trash or
// duplicate-scan surfaces at all.
func (h *apiHandlers) serve(mux *http.ServeMux, mode ServerMode) {
	h.serveReading(mux)
	if mode != ServerModeReader {
		h.serveLibrary(mux)
	}
	h.serveFallbacks(mux)
}

// serveReading mounts what it takes to browse a shelf and read a book, and
// nothing else. Every route here is a GET, and none of them reaches a handler
// that can write.
//
// This is the whole API surface of the standalone reader binary
// (cmd/plainshelf-read), so a route added here is a route that binary starts
// serving. Reads that belong to running a library rather than to reading one —
// the duplicate scan, the trash listing, the logs, the stored settings — belong
// in serveLibrary below.
func (h *apiHandlers) serveReading(mux *http.ServeMux) {
	mux.HandleFunc("GET /health", h.meta.health)

	mux.HandleFunc("GET /api/mode", h.meta.getMode)
	mux.HandleFunc("GET /api/version", h.meta.getVersion)
	mux.HandleFunc("GET /api/shelves", h.shelves.getShelves)
	mux.HandleFunc("GET /api/shelves/{shelf_id}/status", h.shelves.getShelfStatus)

	mux.HandleFunc("GET /api/shelves/{shelf_id}/books", h.books.getBooks)
	mux.HandleFunc("GET /api/shelves/{shelf_id}/books/{book_id}", h.books.getBook)
	mux.HandleFunc("GET /api/shelves/{shelf_id}/books/{book_id}/content", h.books.getBookContent)
	mux.HandleFunc("GET /api/shelves/{shelf_id}/books/{book_id}/cover", h.books.getCover)

	mux.HandleFunc("GET /api/shelves/{shelf_id}/books/{book_id}/sources", h.sources.listSources)
	mux.HandleFunc("GET /api/shelves/{shelf_id}/books/{book_id}/sources/{source_id}", h.sources.getSource)
	mux.HandleFunc("GET /api/shelves/{shelf_id}/books/{book_id}/sources/{source_id}/content", h.sources.getSourceContent)
	mux.HandleFunc("GET /api/shelves/{shelf_id}/books/{book_id}/sources/{source_id}/assets/{asset_name}", h.sources.getAsset)

	mux.HandleFunc("GET /api/shelves/{shelf_id}/layers", h.layers.getLayers)
}

// serveLibrary mounts everything else: the writes, the background work they
// schedule, and the reads that only make sense while running a library.
func (h *apiHandlers) serveLibrary(mux *http.ServeMux) {
	// Shelf API

	mux.HandleFunc("POST /api/shelves/{shelf_id}/scans", h.shelves.rescanShelf)

	// Book API

	mux.HandleFunc("POST /api/shelves/{shelf_id}/books", h.books.createBook)
	mux.HandleFunc("POST /api/shelves/{shelf_id}/book-batches", h.batches.bookBatch)
	mux.HandleFunc("POST /api/shelves/{shelf_id}/content-stat-refreshes", h.batches.refreshContentStats)
	mux.HandleFunc("POST /api/shelves/{shelf_id}/book-cache-exports", h.shelves.exportBookCache)

	mux.HandleFunc("POST /api/shelves/{shelf_id}/books/import", h.imports.importBook)
	mux.HandleFunc("GET /api/shelves/{shelf_id}/books/duplicate", h.books.findDuplicateBooks)

	mux.HandleFunc("PATCH /api/shelves/{shelf_id}/books/{book_id}", h.books.updateBook)
	mux.HandleFunc("DELETE /api/shelves/{shelf_id}/books/{book_id}", h.trash.trashBook)
	mux.HandleFunc("POST /api/shelves/{shelf_id}/books/{book_id}/trash", h.trash.trashBook)

	mux.HandleFunc("POST /api/shelves/{shelf_id}/books/{book_id}/sources", h.sources.createSource)
	mux.HandleFunc("DELETE /api/shelves/{shelf_id}/books/{book_id}/sources/{source_id}", h.sources.deleteSource)
	mux.HandleFunc("PUT /api/shelves/{shelf_id}/books/{book_id}/sources/{source_id}/current", h.sources.setCurrentSource)
	mux.HandleFunc("PATCH /api/shelves/{shelf_id}/books/{book_id}/sources/{source_id}/content", h.sources.updateSourceContent)
	mux.HandleFunc("POST /api/shelves/{shelf_id}/books/{book_id}/sources/{source_id}/refresh", h.sources.refreshSourceMeta)
	mux.HandleFunc("PUT /api/shelves/{shelf_id}/books/{book_id}/sources/{source_id}/assets/{asset_name}", h.sources.updateAsset)
	mux.HandleFunc("DELETE /api/shelves/{shelf_id}/books/{book_id}/sources/{source_id}/assets/{asset_name}", h.sources.deleteAsset)

	mux.HandleFunc("PUT /api/shelves/{shelf_id}/books/{book_id}/cover", h.books.updateCover)
	mux.HandleFunc("DELETE /api/shelves/{shelf_id}/books/{book_id}/cover", h.books.deleteCover)

	mux.HandleFunc("GET /api/shelves/{shelf_id}/trash/books", h.trash.getTrashedBooks)
	mux.HandleFunc("POST /api/shelves/{shelf_id}/trash/empty", h.trash.emptyTrash)
	mux.HandleFunc("POST /api/shelves/{shelf_id}/trash/books/{book_id}/restore", h.trash.restoreTrashedBook)
	mux.HandleFunc("DELETE /api/shelves/{shelf_id}/trash/books/{book_id}", h.trash.deleteTrashedBook)

	mux.HandleFunc("POST /api/shelves/{shelf_id}/layer-moves", h.layers.moveLayer)
	mux.HandleFunc("POST /api/shelves/{shelf_id}/layers/{layer_path...}", h.layers.createLayer)
	mux.HandleFunc("PATCH /api/shelves/{shelf_id}/layers/{layer_path...}", h.layers.renameLayer)
	mux.HandleFunc("DELETE /api/shelves/{shelf_id}/layers/{layer_path...}", h.layers.deleteLayer)

	// Task API

	mux.HandleFunc("GET /api/taskchains/{taskchain_id}", h.taskAPI.getTaskChain)

	// Log API

	mux.HandleFunc("GET /api/logs", h.logs.getLogs)
	mux.HandleFunc("GET /api/logs/{log_id}/content", h.logs.getLogContent)

	// Setting API

	mux.HandleFunc("GET /api/setting/cover_to_jpg", h.setting.getCoverToJPG)
	mux.HandleFunc("POST /api/setting/cover_to_jpg", h.setting.setCoverToJPG)
	mux.HandleFunc("DELETE /api/setting/cover_to_jpg", h.setting.deleteCoverToJPG)
	mux.HandleFunc("GET /api/setting/epub_import_strategy", h.setting.getEPUBImportStrategy)
	mux.HandleFunc("POST /api/setting/epub_import_strategy", h.setting.setEPUBImportStrategy)
	mux.HandleFunc("DELETE /api/setting/epub_import_strategy", h.setting.deleteEPUBImportStrategy)
}

// serveFallbacks mounts the two lowest-ranked patterns, so they only ever match
// what the routes above did not. Registered last for readability; the mux ranks
// by specificity rather than by registration order.
func (h *apiHandlers) serveFallbacks(mux *http.ServeMux) {
	// Unknown API paths must not fall through to the SPA index. In reader mode
	// this is also what answers the routes serveLibrary did not mount: the SPA
	// is served whole, so a page that asks for one gets a 404 rather than an
	// HTML document parsed as JSON.
	mux.HandleFunc("GET /api/{path...}", http.NotFound)

	mux.HandleFunc("GET /{path...}", h.spa.fallback)
}
