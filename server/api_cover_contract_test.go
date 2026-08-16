package server

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAPICoverContract(t *testing.T) {
	env := newAPITestEnv(t)
	created := importTextBook(t, env, "Cover Me", "", "cover.txt", "body")
	url := "/api/shelves/default_shelf/books/" + created.Meta.ID + "/cover"

	rec := env.do(httptest.NewRequest(http.MethodGet, url, nil))
	assertStatus(t, rec, http.StatusNotFound)

	req := httptest.NewRequest(http.MethodPut, url, strings.NewReader("not image"))
	req.Header.Set("Content-Type", "text/plain")
	rec = env.do(req)
	assertStatus(t, rec, http.StatusBadRequest)

	req = httptest.NewRequest(http.MethodPut, url, bytes.NewReader(bytes.Repeat([]byte{'x'}, maxCoverBodySize+1)))
	req.Header.Set("Content-Type", "image/png")
	rec = env.do(req)
	assertStatus(t, rec, http.StatusRequestEntityTooLarge)

	coverBytes := []byte("fake png bytes")
	req = httptest.NewRequest(http.MethodPut, url, bytes.NewReader(coverBytes))
	req.Header.Set("Content-Type", "image/png")
	rec = env.do(req)
	assertStatus(t, rec, http.StatusNoContent)

	rec = env.do(httptest.NewRequest(http.MethodGet, url, nil))
	assertStatus(t, rec, http.StatusOK)
	if got := rec.Header().Get("Content-Type"); got != "image/png" {
		t.Fatalf("cover Content-Type = %q, want image/png", got)
	}
	if !bytes.Equal(rec.Body.Bytes(), coverBytes) {
		t.Fatalf("cover bytes = %q, want %q", rec.Body.Bytes(), coverBytes)
	}

	webpBytes := []byte("fake webp bytes")
	req = httptest.NewRequest(http.MethodPut, url, bytes.NewReader(webpBytes))
	req.Header.Set("Content-Type", "image/webp")
	rec = env.do(req)
	assertStatus(t, rec, http.StatusNoContent)

	rec = env.do(httptest.NewRequest(http.MethodGet, url, nil))
	assertStatus(t, rec, http.StatusOK)
	if got := rec.Header().Get("Content-Type"); got != "image/webp" {
		t.Fatalf("cover Content-Type = %q, want image/webp", got)
	}
	if !bytes.Equal(rec.Body.Bytes(), webpBytes) {
		t.Fatalf("cover bytes = %q, want %q", rec.Body.Bytes(), webpBytes)
	}

	rec = env.do(httptest.NewRequest(http.MethodDelete, url, nil))
	assertStatus(t, rec, http.StatusNoContent)
	rec = env.do(httptest.NewRequest(http.MethodGet, url, nil))
	assertStatus(t, rec, http.StatusNotFound)
}
