package readerapi

import (
	"encoding/json/v2"
	"errors"
	"io"
	"io/fs"
	"mime"
	"net/http"

	"github.com/voilelab/plainshelf/internal/jsonopt"
	"github.com/voilelab/plainshelf/shelf/bookpkg"
)

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

func (h *handlers) writeJSON(w http.ResponseWriter, value any) {
	body, err := json.Marshal(value, jsonopt.API())
	if err != nil {
		h.lib.warn("failed to encode response", "error", err)
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if _, err := w.Write(body); err != nil {
		h.lib.warn("failed to write response", "error", err)
	}
}

func (h *handlers) getMode(w http.ResponseWriter, _ *http.Request) {
	h.writeJSON(w, map[string]bool{"read_only": true})
}

func (h *handlers) getVersion(w http.ResponseWriter, _ *http.Request) {
	h.writeJSON(w, map[string]string{"version": h.version})
}

func (h *handlers) getBook(w http.ResponseWriter, r *http.Request) {
	h.withBook(w, r, func(book *bookpkg.Book) {
		h.writeJSON(w, struct {
			Meta   *bookpkg.BookMeta `json:"meta"`
			Folder []string          `json:"folder"`
		}{Meta: book.GetMeta(), Folder: []string{}})
	})
}

func (h *handlers) getBookContent(w http.ResponseWriter, r *http.Request) {
	h.withBook(w, r, func(book *bookpkg.Book) {
		source, ok := h.source(w, r, book)
		if !ok {
			return
		}
		h.streamSource(w, book, source)
	})
}

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
	h.writeJSON(w, metas)
}

func (h *handlers) getSource(w http.ResponseWriter, r *http.Request) {
	h.withBook(w, r, func(book *bookpkg.Book) {
		source, ok := h.source(w, r, book)
		if !ok {
			return
		}
		h.writeJSON(w, source.GetMeta())
	})
}

func (h *handlers) getSourceContent(w http.ResponseWriter, r *http.Request) {
	h.withBook(w, r, func(book *bookpkg.Book) {
		source, ok := h.source(w, r, book)
		if !ok {
			return
		}
		h.streamSource(w, book, source)
	})
}

func (h *handlers) streamSource(w http.ResponseWriter, book *bookpkg.Book, source *bookpkg.Source) {
	content, err := source.Open()
	if err != nil {
		http.Error(w, "failed to open the source", http.StatusInternalServerError)
		return
	}
	defer content.Close()

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	if _, err := io.Copy(w, content); err != nil {
		h.lib.warn("source stream ended early", "book_id", book.ID(), "source_id", source.ID(), "error", err)
	}
}

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
	w.Write(cover)
}

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

	asset, err := source.OpenAsset(name)
	if err != nil {
		http.Error(w, "asset not found", http.StatusNotFound)
		return
	}
	defer asset.File.Close()

	w.Header().Set("Content-Type", contentTypeForExt(asset.Ext))
	if _, err := io.Copy(w, asset.File); err != nil {
		return
	}
}

func contentTypeForExt(ext string) string {
	if mimeType := mime.TypeByExtension(ext); mimeType != "" {
		return mimeType
	}
	return "application/octet-stream"
}
