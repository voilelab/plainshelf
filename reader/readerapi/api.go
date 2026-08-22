package readerapi

import (
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"path"

	"github.com/voilelab/plainshelf/shelf/bookpkg"
)

// NewHandler serves the reads the reader UI performs, and nothing else.
//
// The paths mirror the server's so the frontend's own provider works unchanged;
// what is missing is the point: no listing, no layers, no trash, no writes. A
// path outside this set is a 404 rather than a stub, so a UI that wandered
// somewhere it cannot go here fails loudly during development.
func NewHandler(lib *Library, version string) http.Handler {
	h := &handlers{lib: lib, version: version}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/mode", h.getMode)
	mux.HandleFunc("GET /api/version", h.getVersion)

	const book = "GET /api/shelves/{shelf_id}/books/{book_id}"
	mux.HandleFunc(book, h.getBook)
	mux.HandleFunc(book+"/content", h.getBookContent)
	mux.HandleFunc(book+"/cover", h.getCover)
	mux.HandleFunc(book+"/sources", h.listSources)
	mux.HandleFunc(book+"/sources/{source_id}", h.getSource)
	mux.HandleFunc(book+"/sources/{source_id}/content", h.getSourceContent)
	mux.HandleFunc(book+"/sources/{source_id}/assets/{asset_name}", h.getAsset)

	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	})

	return mux
}

type handlers struct {
	lib     *Library
	version string
}

// withBook resolves the request's book and answers the request itself when it
// cannot: no book open yet (503, the window is still asking for a folder), or
// a request for a book this reader does not hold (404).
//
// read runs while the package is held open, so a book opened in another window
// — or from the File menu — cannot close the directory handle midway through
// this response.
func (h *handlers) withBook(w http.ResponseWriter, r *http.Request, read func(*bookpkg.Book)) {
	held := h.lib.withBook(func(book *bookpkg.Book) {
		if requested := r.PathValue("book_id"); requested != book.ID() {
			http.Error(w, "book not found", http.StatusNotFound)
			return
		}
		read(book)
	})

	if !held {
		http.Error(w, "no book is open", http.StatusServiceUnavailable)
	}
}

// source resolves the source named in the path, or the current one when the
// route carries no source id.
func (h *handlers) source(w http.ResponseWriter, r *http.Request, book *bookpkg.Book) (*bookpkg.Source, bool) {
	sourceID := r.PathValue("source_id")
	if sourceID == "" {
		source, err := book.ResolveCurrentSource()
		if err != nil {
			http.Error(w, "failed to resolve the book's current source", http.StatusInternalServerError)
			return nil, false
		}
		return source, true
	}

	source, err := book.GetSource(sourceID)
	if err != nil {
		http.Error(w, "source not found", http.StatusNotFound)
		return nil, false
	}
	return source, true
}

func writeJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err := json.NewEncoder(w).Encode(value); err != nil {
		// The status is already written by now, so the response is lost either
		// way; closing the connection is what tells the client not to parse it.
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
	}
}

// GET /api/mode
//
// Read-only is not a mode the user can leave here: the reader has no write
// paths at all. The frontend reads this to hide every editing affordance.
func (h *handlers) getMode(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, map[string]bool{"read_only": true})
}

// GET /api/version
func (h *handlers) getVersion(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, map[string]string{"version": h.version})
}

// GET /api/shelves/{shelf_id}/books/{book_id}
//
// The response mirrors the server's: metadata under "meta", layers beside it.
// A package has no layers — it is not inside a shelf — so the list is empty
// rather than absent, which is what the frontend's transform expects.
func (h *handlers) getBook(w http.ResponseWriter, r *http.Request) {
	h.withBook(w, r, func(book *bookpkg.Book) {
		writeJSON(w, struct {
			Meta  *bookpkg.BookMeta `json:"meta"`
			Layer []string          `json:"layer"`
		}{Meta: book.GetMeta(), Layer: []string{}})
	})
}

// GET /api/shelves/{shelf_id}/books/{book_id}/content
func (h *handlers) getBookContent(w http.ResponseWriter, r *http.Request) {
	h.withBook(w, r, func(book *bookpkg.Book) {
		source, ok := h.source(w, r, book)
		if !ok {
			return
		}
		h.streamSource(w, source)
	})
}

// GET /api/shelves/{shelf_id}/books/{book_id}/sources
func (h *handlers) listSources(w http.ResponseWriter, r *http.Request) {
	h.withBook(w, r, func(book *bookpkg.Book) {
		h.writeSourceList(w, book)
	})
}

func (h *handlers) writeSourceList(w http.ResponseWriter, book *bookpkg.Book) {
	sources, err := book.ListSource()
	if err != nil {
		http.Error(w, "failed to list the book's sources", http.StatusInternalServerError)
		return
	}

	metas := make([]*bookpkg.SourceMeta, 0, len(sources))
	for _, source := range sources {
		metas = append(metas, source.GetMeta())
	}
	writeJSON(w, metas)
}

// GET /api/shelves/{shelf_id}/books/{book_id}/sources/{source_id}
func (h *handlers) getSource(w http.ResponseWriter, r *http.Request) {
	h.withBook(w, r, func(book *bookpkg.Book) {
		source, ok := h.source(w, r, book)
		if !ok {
			return
		}
		writeJSON(w, source.GetMeta())
	})
}

// GET /api/shelves/{shelf_id}/books/{book_id}/sources/{source_id}/content
func (h *handlers) getSourceContent(w http.ResponseWriter, r *http.Request) {
	h.withBook(w, r, func(book *bookpkg.Book) {
		source, ok := h.source(w, r, book)
		if !ok {
			return
		}
		h.streamSource(w, source)
	})
}

func (h *handlers) streamSource(w http.ResponseWriter, source *bookpkg.Source) {
	content, err := source.Open()
	if err != nil {
		http.Error(w, "failed to open the source", http.StatusInternalServerError)
		return
	}
	defer content.Close() //nolint:errcheck // read-only handle

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	if _, err := io.Copy(w, content); err != nil {
		// Same as writeJSON: the status is out, so this only records why the
		// body is short.
		return
	}
}

// GET /api/shelves/{shelf_id}/books/{book_id}/cover
func (h *handlers) getCover(w http.ResponseWriter, r *http.Request) {
	h.withBook(w, r, func(book *bookpkg.Book) {
		h.writeCover(w, book)
	})
}

func (h *handlers) writeCover(w http.ResponseWriter, book *bookpkg.Book) {
	cover, ext, err := book.OpenCover()
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			http.Error(w, "cover not found", http.StatusNotFound)
			return
		}
		http.Error(w, "failed to open the cover", http.StatusInternalServerError)
		return
	}
	if len(cover) == 0 {
		http.Error(w, "cover not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", contentTypeForExt(ext))
	w.Write(cover) //nolint:errcheck // the status is already written
}

// GET /api/shelves/{shelf_id}/books/{book_id}/sources/{source_id}/assets/{asset_name}
func (h *handlers) getAsset(w http.ResponseWriter, r *http.Request) {
	h.withBook(w, r, func(book *bookpkg.Book) {
		source, ok := h.source(w, r, book)
		if !ok {
			return
		}
		h.writeAsset(w, source, r.PathValue("asset_name"))
	})
}

func (h *handlers) writeAsset(w http.ResponseWriter, source *bookpkg.Source, name string) {
	// Whatever the name is, it is resolved by the shelf's own asset rules
	// (assets/ only, no traversal, supported extensions) rather than by string
	// handling here.
	asset, err := source.OpenAsset(name)
	if err != nil {
		http.Error(w, "asset not found", http.StatusNotFound)
		return
	}
	defer asset.File.Close() //nolint:errcheck // read-only handle

	w.Header().Set("Content-Type", contentTypeForExt(asset.Ext))
	if _, err := io.Copy(w, asset.File); err != nil {
		return
	}
}

// contentTypeForExt keeps the reader from guessing: an illustration the browser
// cannot type is served as bytes rather than as something it will try to parse.
func contentTypeForExt(ext string) string {
	if ext == "" {
		return "application/octet-stream"
	}
	if !path.IsAbs(ext) && ext[0] != '.' {
		ext = "." + ext
	}
	if mimeType := mime.TypeByExtension(ext); mimeType != "" {
		return mimeType
	}
	return "application/octet-stream"
}
