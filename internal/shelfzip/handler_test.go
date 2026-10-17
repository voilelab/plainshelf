package shelfzip

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func serve(h http.Handler, method, url string, body []byte) int {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(method, url, bytes.NewReader(body)))
	return rec.Code
}

func TestHandlerImportWaitsForInFlightRequestsThenGoesStale(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, map[string]string{"books/old.bookpkg/book.json": "old"})
	entered := make(chan struct{})
	release := make(chan struct{})
	app := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/slow" {
			close(entered)
			<-release
		}
		w.WriteHeader(http.StatusOK)
	})
	h := Handler(root, app)

	slow := make(chan int)
	go func() { slow <- serve(h, http.MethodGet, "/api/slow", nil) }()
	<-entered

	imported := make(chan int)
	z := zipOf(t, map[string]string{"books/new.bookpkg/book.json": "new"})
	go func() { imported <- serve(h, http.MethodPut, URL, z) }()
	select {
	case code := <-imported:
		t.Fatalf("import finished (%d) while a request was still in flight", code)
	case <-time.After(100 * time.Millisecond):
	}
	if got := readTree(t, root)["books/old.bookpkg/book.json"]; got != "old" {
		t.Fatal("the shelf changed under an in-flight request")
	}

	close(release)
	if code := <-slow; code != http.StatusOK {
		t.Fatalf("in-flight request = %d", code)
	}
	if code := <-imported; code != http.StatusNoContent {
		t.Fatalf("import = %d", code)
	}
	if got := readTree(t, root)["books/new.bookpkg/book.json"]; got != "new" {
		t.Fatal("import did not land")
	}
	for _, req := range [][2]string{{http.MethodGet, "/api/books"}, {http.MethodGet, URL}, {http.MethodPut, URL}} {
		if code := serve(h, req[0], req[1], z); code != http.StatusServiceUnavailable {
			t.Errorf("%s %s after the import = %d, want 503", req[0], req[1], code)
		}
	}
}

func TestHandlerStatusCodes(t *testing.T) {
	withLimit(t, 1<<20)
	root := t.TempDir()
	writeTree(t, root, map[string]string{"books/a.bookpkg/book.json": "a"})
	h := Handler(root, http.NotFoundHandler())
	if code := serve(h, http.MethodGet, URL, nil); code != http.StatusOK {
		t.Errorf("export = %d", code)
	}
	if code := serve(h, http.MethodPut, URL, []byte("not a zip")); code != http.StatusBadRequest {
		t.Errorf("garbage import = %d, want 400", code)
	}
	if code := serve(h, http.MethodPut, URL, zipOf(t, map[string]string{"notes.txt": "x"})); code != http.StatusBadRequest {
		t.Errorf("non-shelf import = %d, want 400", code)
	}
	if code := serve(h, http.MethodPut, URL, make([]byte, maxImportBytes+1)); code != http.StatusRequestEntityTooLarge {
		t.Errorf("oversized import = %d, want 413", code)
	}
	if code := serve(h, http.MethodGet, "/api/anything", nil); code != http.StatusNotFound {
		t.Errorf("app passthrough = %d, want the app's 404", code)
	}
}
