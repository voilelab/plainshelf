package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func serveAsset(t *testing.T, path string) *httptest.ResponseRecorder {
	t.Helper()
	assets := fstest.MapFS{
		"index.html": {Data: []byte("<html><head><title>x</title></head><body></body></html>")},
	}
	api := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("api")) })
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("asset")) })
	handler := assetMiddleware(assets, func() http.Handler { return api })(next)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

func TestIndexCarriesDesktopMarker(t *testing.T) {
	for _, path := range []string{"/", "/index.html"} {
		rec := serveAsset(t, path)
		body := rec.Body.String()
		marker := strings.Index(body, "window.__PLAINSHELF_DESKTOP__")
		if marker < 0 || marker > strings.Index(body, "</head>") {
			t.Fatalf("%s: marker missing from <head>: %s", path, body)
		}
		if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/html") {
			t.Fatalf("%s: Content-Type = %q, want text/html for Wails runtime injection", path, got)
		}
	}
}

func TestAssetMiddlewareRoutes(t *testing.T) {
	for path, want := range map[string]string{
		"/api/books":     "api",
		"/api":           "api",
		"/assets/app.js": "asset",
	} {
		if got := serveAsset(t, path).Body.String(); got != want {
			t.Fatalf("%s served %q, want %q", path, got, want)
		}
	}
}
