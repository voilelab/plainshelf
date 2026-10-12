package shelfzip

import (
	"archive/zip"
	"bytes"
	"errors"
	"io"
	"net/http"
	"sync"
)

// URL is where Handler serves the shelf zip: GET exports, PUT imports.
const URL = "/_demo/shelf.zip"

// Handler serves the shelf zip in front of app. An import waits for every
// request in flight and holds new ones off while it swaps the directories;
// afterwards every request answers 503, since the running server's view of the
// shelf is stale until the page reloads and the server starts again.
func Handler(libRoot string, app http.Handler) http.Handler {
	var (
		mu       sync.RWMutex
		replaced bool
	)
	stale := func(w http.ResponseWriter) {
		http.Error(w, "the shelf was replaced; reload the page", http.StatusServiceUnavailable)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET "+URL, func(w http.ResponseWriter, _ *http.Request) {
		mu.RLock()
		defer mu.RUnlock()
		if replaced {
			stale(w)
			return
		}
		var buf bytes.Buffer
		if err := Export(&buf, libRoot); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/zip")
		_, _ = w.Write(buf.Bytes())
	})
	mux.HandleFunc("PUT "+URL, func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(io.LimitReader(r.Body, maxImportBytes+1))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if int64(len(body)) > maxImportBytes {
			http.Error(w, ErrTooLarge.Error(), http.StatusRequestEntityTooLarge)
			return
		}
		mu.Lock()
		defer mu.Unlock()
		if replaced {
			stale(w)
			return
		}
		err = Import(bytes.NewReader(body), int64(len(body)), libRoot)
		switch {
		case errors.Is(err, ErrNotShelf), errors.Is(err, ErrUnsafePath), errors.Is(err, ErrTooLarge), errors.Is(err, zip.ErrFormat):
			http.Error(w, err.Error(), http.StatusBadRequest)
		case err != nil:
			http.Error(w, err.Error(), http.StatusInternalServerError)
		default:
			replaced = true
			w.WriteHeader(http.StatusNoContent)
		}
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		mu.RLock()
		defer mu.RUnlock()
		if replaced {
			stale(w)
			return
		}
		app.ServeHTTP(w, r)
	})
	return mux
}
