package main

import (
	"bytes"
	"io/fs"
	"net/http"
	"slices"
	"strings"

	"github.com/voilelab/plainshelf/internal/util"
)

const desktopMarker = "<script>window.__PLAINSHELF_DESKTOP__ = true;</script>"

func assetMiddleware(assets fs.FS, api func() http.Handler) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch {
			case strings.HasPrefix(r.URL.Path, "/api/") || r.URL.Path == "/api":
				api().ServeHTTP(w, r)
			case r.Method == http.MethodGet && (r.URL.Path == "/" || r.URL.Path == "/index.html"):
				serveDesktopIndex(w, assets)
			default:
				next.ServeHTTP(w, r)
			}
		})
	}
}

func serveDesktopIndex(w http.ResponseWriter, assets fs.FS) {
	page, err := desktopIndexHTML(assets)
	if err != nil {
		http.Error(w, "failed to render the app shell", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(page)
}

func desktopIndexHTML(assets fs.FS) ([]byte, error) {
	page, err := fs.ReadFile(assets, "index.html")
	if err != nil {
		return nil, util.Errorf("%w", err)
	}
	if index := bytes.Index(page, []byte("</head>")); index >= 0 {
		return slices.Concat(page[:index], []byte(desktopMarker), page[index:]), nil
	}
	return slices.Concat([]byte(desktopMarker), page), nil
}
