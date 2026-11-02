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
		"index.html":    {Data: []byte("<html><head><title>x</title></head><body></body></html>")},
		"assets/app.js": {Data: []byte("app")},
		"favicon.ico":   {Data: []byte("icon")},
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

func TestIndexOmitsRuntimeWailsInjects(t *testing.T) {
	for _, path := range []string{"/", "/index.html"} {
		if body := serveAsset(t, path).Body.String(); strings.Contains(body, "/wails/") {
			t.Fatalf("%s: Wails injects its runtime there itself, got %s", path, body)
		}
	}
}

func TestSPARouteCarriesMarkerAndRuntime(t *testing.T) {
	for _, path := range []string{"/books", "/settings", "/reader/0123abcd"} {
		rec := serveAsset(t, path)
		body := rec.Body.String()
		head := body[:strings.Index(body, "</head>")]
		for _, want := range []string{
			"window.__PLAINSHELF_DESKTOP__",
			`<script src="/wails/ipc.js"></script>`,
			`<script src="/wails/runtime.js"></script>`,
		} {
			if !strings.Contains(head, want) {
				t.Fatalf("%s: %q missing from <head>: %s", path, want, body)
			}
		}
		if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/html") {
			t.Fatalf("%s: Content-Type = %q, want text/html", path, got)
		}
	}
}

func TestAssetMiddlewareRoutes(t *testing.T) {
	for path, want := range map[string]string{
		"/api/books":        "api",
		"/api":              "api",
		"/assets/app.js":    "asset",
		"/favicon.ico":      "asset",
		"/assets/gone.js":   "asset",
		"/wails/runtime.js": "asset",
		"/wails/ipc.js":     "asset",
		"/wails/runtime":    "asset",
	} {
		if got := serveAsset(t, path).Body.String(); got != want {
			t.Fatalf("%s served %q, want %q", path, got, want)
		}
	}
}
