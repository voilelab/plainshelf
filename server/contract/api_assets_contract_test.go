package contract_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

func TestAPISourceAssetContract(t *testing.T) {
	env := newAPITestEnv(t)
	created := importTextBook(t, env, "Illustrated", "", "art.md", "body")
	sourceID := env.currentSourceID(t, created.Meta.ID)
	assetsURL := "/api/shelves/default_shelf/books/" + created.Meta.ID + "/sources/" + sourceID + "/assets/"

	// Nothing has been placed under assets/ yet.
	rec := env.do(httptest.NewRequest(http.MethodGet, assetsURL+"img-0001.png", nil))
	assertStatus(t, rec, http.StatusNotFound)

	pngBytes := []byte("fake png bytes")
	env.writeSourceAsset(t, created.Meta.ID, sourceID, "img-0001.png", pngBytes)

	rec = env.do(httptest.NewRequest(http.MethodGet, assetsURL+"img-0001.png", nil))
	assertStatus(t, rec, http.StatusOK)
	if got := rec.Header().Get("Content-Type"); got != "image/png" {
		t.Fatalf("asset Content-Type = %q, want image/png", got)
	}
	if !bytes.Equal(rec.Body.Bytes(), pngBytes) {
		t.Fatalf("asset bytes = %q, want %q", rec.Body.Bytes(), pngBytes)
	}

	// The asset is streamed rather than buffered, so the length has to be
	// declared from the file's own size instead of the written body.
	if got := rec.Header().Get("Content-Length"); got != strconv.Itoa(len(pngBytes)) {
		t.Fatalf("asset Content-Length = %q, want %d", got, len(pngBytes))
	}

	// An asset can be replaced or removed while its URL stays the same, so the
	// client has to ask every time. This env leaves protect_read off, so the
	// response is still shared-cacheable; TestCacheVisibilityFollowsTheTokenGate
	// covers the protected case.
	if got := rec.Header().Get("Cache-Control"); got != "public, no-cache" {
		t.Fatalf("asset Cache-Control = %q, want public, no-cache", got)
	}

	// The reader fetches every illustration on a chapter, so a revalidating
	// request must be answerable without resending the bytes.
	etag := rec.Header().Get("ETag")
	if etag == "" {
		t.Fatal("asset response carries no ETag")
	}
	req := httptest.NewRequest(http.MethodGet, assetsURL+"img-0001.png", nil)
	req.Header.Set("If-None-Match", etag)
	rec = env.do(req)
	assertStatus(t, rec, http.StatusNotModified)
	if rec.Body.Len() != 0 {
		t.Fatalf("304 response body = %q, want empty", rec.Body.String())
	}

	// Each stored extension keeps its own content type.
	env.writeSourceAsset(t, created.Meta.ID, sourceID, "img-0002.webp", []byte("fake webp bytes"))
	rec = env.do(httptest.NewRequest(http.MethodGet, assetsURL+"img-0002.webp", nil))
	assertStatus(t, rec, http.StatusOK)
	if got := rec.Header().Get("Content-Type"); got != "image/webp" {
		t.Fatalf("asset Content-Type = %q, want image/webp", got)
	}

	// ServeMux has already unescaped the wildcard, so a name carrying a literal
	// percent escape must survive addressing intact. Decoding it a second time
	// would land on "chart one.png" instead, which exists here precisely so a
	// regression serves the wrong bytes rather than merely 404ing.
	env.writeSourceAsset(t, created.Meta.ID, sourceID, "chart%20one.png", []byte("percent name"))
	env.writeSourceAsset(t, created.Meta.ID, sourceID, "chart one.png", []byte("space name"))

	rec = env.do(httptest.NewRequest(http.MethodGet, assetsURL+"chart%2520one.png", nil))
	assertStatus(t, rec, http.StatusOK)
	if got := rec.Body.String(); got != "percent name" {
		t.Fatalf("asset with a percent in its name = %q, want %q", got, "percent name")
	}

	rec = env.do(httptest.NewRequest(http.MethodGet, assetsURL+"chart%20one.png", nil))
	assertStatus(t, rec, http.StatusOK)
	if got := rec.Body.String(); got != "space name" {
		t.Fatalf("asset with a space in its name = %q, want %q", got, "space name")
	}

	// A GET pattern also matches HEAD. The headers must be identical while the
	// asset itself is never read: the empty recorder body is what shows the
	// copy was skipped, since httptest does not suppress it the way net/http
	// would.
	req = httptest.NewRequest(http.MethodHead, assetsURL+"img-0001.png", nil)
	rec = env.do(req)
	assertStatus(t, rec, http.StatusOK)
	if rec.Body.Len() != 0 {
		t.Fatalf("HEAD response body = %q, want empty", rec.Body.String())
	}
	if got := rec.Header().Get("Content-Length"); got != strconv.Itoa(len(pngBytes)) {
		t.Fatalf("HEAD Content-Length = %q, want %d", got, len(pngBytes))
	}
	if got := rec.Header().Get("Content-Type"); got != "image/png" {
		t.Fatalf("HEAD Content-Type = %q, want image/png", got)
	}

	// A missing book or source is a 404, not a 500.
	rec = env.do(httptest.NewRequest(http.MethodGet,
		"/api/shelves/default_shelf/books/no-such-book/sources/"+sourceID+"/assets/img-0001.png", nil))
	assertStatus(t, rec, http.StatusNotFound)
	rec = env.do(httptest.NewRequest(http.MethodGet,
		"/api/shelves/default_shelf/books/"+created.Meta.ID+"/sources/no-such-source/assets/img-0001.png", nil))
	assertStatus(t, rec, http.StatusNotFound)

	// POST and PATCH still have no meaning on an asset; PUT and DELETE do.
	for _, method := range []string{http.MethodPost, http.MethodPatch} {
		rec = env.do(httptest.NewRequest(method, assetsURL+"img-0001.png", nil))
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("%s asset status = %d, want %d", method, rec.Code, http.StatusMethodNotAllowed)
		}
	}
}

func TestAPISourceAssetWriteContract(t *testing.T) {
	env := newAPITestEnv(t)
	created := importTextBook(t, env, "Editable Art", "", "art.md", "body")
	sourceID := env.currentSourceID(t, created.Meta.ID)
	assetsURL := "/api/shelves/default_shelf/books/" + created.Meta.ID + "/sources/" + sourceID + "/assets/"

	// Uploading creates the directory and the file.
	pngBytes := []byte("fake png bytes")
	rec := env.do(httptest.NewRequest(http.MethodPut, assetsURL+"img-0001.png", bytes.NewReader(pngBytes)))
	assertStatus(t, rec, http.StatusNoContent)

	rec = env.do(httptest.NewRequest(http.MethodGet, assetsURL+"img-0001.png", nil))
	assertStatus(t, rec, http.StatusOK)
	if !bytes.Equal(rec.Body.Bytes(), pngBytes) {
		t.Fatalf("stored asset = %q, want %q", rec.Body.Bytes(), pngBytes)
	}

	// Uploading again under the same name replaces it.
	replaced := []byte("replacement bytes")
	rec = env.do(httptest.NewRequest(http.MethodPut, assetsURL+"img-0001.png", bytes.NewReader(replaced)))
	assertStatus(t, rec, http.StatusNoContent)
	rec = env.do(httptest.NewRequest(http.MethodGet, assetsURL+"img-0001.png", nil))
	if !bytes.Equal(rec.Body.Bytes(), replaced) {
		t.Fatalf("replaced asset = %q, want %q", rec.Body.Bytes(), replaced)
	}

	// The name is validated on the way in exactly as it is on the way out, so
	// a file the read path could never serve cannot be written either.
	for _, assetName := range []string{"..%2fescaped.png", ".hidden.png", "notes.txt", "img-0002"} {
		rec = env.do(httptest.NewRequest(http.MethodPut, assetsURL+assetName, bytes.NewReader(pngBytes)))
		assertStatus(t, rec, http.StatusBadRequest)
	}

	// Oversized uploads are refused rather than spooled.
	rec = env.do(httptest.NewRequest(http.MethodPut, assetsURL+"img-0003.png",
		bytes.NewReader(bytes.Repeat([]byte{'x'}, (20<<20)+1))))
	assertStatus(t, rec, http.StatusRequestEntityTooLarge)

	// Deleting removes it; deleting again reports the miss rather than
	// succeeding quietly, since an asset is addressed by name.
	rec = env.do(httptest.NewRequest(http.MethodDelete, assetsURL+"img-0001.png", nil))
	assertStatus(t, rec, http.StatusNoContent)
	rec = env.do(httptest.NewRequest(http.MethodGet, assetsURL+"img-0001.png", nil))
	assertStatus(t, rec, http.StatusNotFound)
	rec = env.do(httptest.NewRequest(http.MethodDelete, assetsURL+"img-0001.png", nil))
	assertStatus(t, rec, http.StatusNotFound)
}

// A replaced illustration keeps its URL - the reader derives it from the file
// name in the text, and nothing records a version to bust a cache with - so a
// client that may reuse the old bytes would show the wrong picture.
func TestAssetRevalidationSurvivesAReplacement(t *testing.T) {
	env := newAPITestEnv(t)
	created := importTextBook(t, env, "Replaced Art", "", "art.md", "body")
	sourceID := env.currentSourceID(t, created.Meta.ID)
	url := "/api/shelves/default_shelf/books/" + created.Meta.ID +
		"/sources/" + sourceID + "/assets/img-0001.png"

	rec := env.do(httptest.NewRequest(http.MethodPut, url, strings.NewReader("first bytes")))
	assertStatus(t, rec, http.StatusNoContent)

	rec = env.do(httptest.NewRequest(http.MethodGet, url, nil))
	assertStatus(t, rec, http.StatusOK)
	firstETag := rec.Header().Get("ETag")
	if got := rec.Header().Get("Cache-Control"); !strings.Contains(got, "no-cache") {
		t.Fatalf("asset Cache-Control = %q, want it to force revalidation", got)
	}

	// A cover may be cached for a day because its URL gains a cache-busting key
	// when it changes; an asset URL never changes, hence the difference.
	coverReq := httptest.NewRequest(http.MethodPut,
		"/api/shelves/default_shelf/books/"+created.Meta.ID+"/cover", strings.NewReader("cover"))
	coverReq.Header.Set("Content-Type", "image/png")
	rec = env.do(coverReq)
	assertStatus(t, rec, http.StatusNoContent)
	rec = env.do(httptest.NewRequest(http.MethodGet,
		"/api/shelves/default_shelf/books/"+created.Meta.ID+"/cover", nil))
	if got := rec.Header().Get("Cache-Control"); !strings.Contains(got, "max-age=86400") {
		t.Fatalf("cover Cache-Control = %q, want it to stay cacheable", got)
	}

	// Replacing changes the validator, so a client holding the old one is told
	// to take the new bytes rather than being answered 304.
	rec = env.do(httptest.NewRequest(http.MethodPut, url, strings.NewReader("second bytes, longer")))
	assertStatus(t, rec, http.StatusNoContent)

	req := httptest.NewRequest(http.MethodGet, url, nil)
	req.Header.Set("If-None-Match", firstETag)
	rec = env.do(req)
	assertStatus(t, rec, http.StatusOK)
	if got := rec.Body.String(); got != "second bytes, longer" {
		t.Fatalf("revalidated asset = %q, want the replacement", got)
	}

	// And once it is deleted, the same conditional request reports the miss
	// rather than confirming a copy that is no longer there.
	rec = env.do(httptest.NewRequest(http.MethodDelete, url, nil))
	assertStatus(t, rec, http.StatusNoContent)

	req = httptest.NewRequest(http.MethodGet, url, nil)
	req.Header.Set("If-None-Match", firstETag)
	rec = env.do(req)
	assertStatus(t, rec, http.StatusNotFound)
}

// Writing an asset is a mutating request like any other, so both gates that
// decide what may write have to cover it.
func TestAPISourceAssetWritesAreGated(t *testing.T) {
	env := newAPITestEnv(t)
	created := importTextBook(t, env, "Gated Art", "", "art.md", "body")
	sourceID := env.currentSourceID(t, created.Meta.ID)
	assetsURL := "/api/shelves/default_shelf/books/" + created.Meta.ID + "/sources/" + sourceID + "/assets/"

	// doRaw omits the token do() would attach.
	for _, method := range []string{http.MethodPut, http.MethodDelete} {
		rec := env.doRaw(httptest.NewRequest(method, assetsURL+"img-0001.png", bytes.NewReader([]byte("x"))))
		assertStatus(t, rec, http.StatusUnauthorized)
	}

	env.app.Conf().ReadOnly = true
	for _, method := range []string{http.MethodPut, http.MethodDelete} {
		rec := env.do(httptest.NewRequest(method, assetsURL+"img-0001.png", bytes.NewReader([]byte("x"))))
		assertStatus(t, rec, http.StatusForbidden)
	}
	env.app.Conf().ReadOnly = false

	// A read is unaffected by either gate in this configuration.
	rec := env.doRaw(httptest.NewRequest(http.MethodGet, assetsURL+"img-0001.png", nil))
	assertStatus(t, rec, http.StatusNotFound)
}

// The asset route reaches the filesystem by name, so it gets its own traversal
// cases rather than trusting the shelf-level test alone.
func TestAPISourceAssetRejectsUnsafeNames(t *testing.T) {
	env := newAPITestEnv(t)
	created := importTextBook(t, env, "Unsafe Assets", "", "art.md", "secret body")
	sourceID := env.currentSourceID(t, created.Meta.ID)
	assetsURL := "/api/shelves/default_shelf/books/" + created.Meta.ID + "/sources/" + sourceID + "/assets/"

	env.writeSourceAsset(t, created.Meta.ID, sourceID, "img-0001.png", []byte("fake png bytes"))

	// A name is served exactly as addressed. Trimming it would make " lead.png"
	// unreachable and, with both files present, quietly answer with the other
	// one - the same failure the double-decode used to cause.
	env.writeSourceAsset(t, created.Meta.ID, sourceID, " lead.png", []byte("space prefixed"))
	env.writeSourceAsset(t, created.Meta.ID, sourceID, "lead.png", []byte("plain"))

	rec := env.do(httptest.NewRequest(http.MethodGet, assetsURL+"%20lead.png", nil))
	assertStatus(t, rec, http.StatusOK)
	if got := rec.Body.String(); got != "space prefixed" {
		t.Fatalf("asset with a leading space = %q, want %q", got, "space prefixed")
	}

	rec = env.do(httptest.NewRequest(http.MethodGet, assetsURL+"lead.png", nil))
	assertStatus(t, rec, http.StatusOK)
	if got := rec.Body.String(); got != "plain" {
		t.Fatalf("asset without a leading space = %q, want %q", got, "plain")
	}

	// An encoded separator survives routing: the mux hands these to the handler
	// as one path value that decodes to "../source.txt". Since source.txt really
	// does sit one level above assets/, they name a file that exists, so 400 is
	// evidence the name was refused rather than merely unresolvable.
	for _, assetName := range []string{
		"..%2fsource.txt",
		"..%2Fsource.txt",
		"%2e%2e%2fsource.txt",
		"%2e%2e%2f%2e%2e%2fbook.json",
		"..%5csource.txt",
		"%2fetc%2fhostname",
		".hidden.png",
		"source.txt",
		"img-0001",
	} {
		t.Run(assetName, func(t *testing.T) {
			rec := env.do(httptest.NewRequest(http.MethodGet, assetsURL+assetName, nil))
			assertStatus(t, rec, http.StatusBadRequest)
			if strings.Contains(rec.Body.String(), "secret body") {
				t.Fatalf("response leaked file contents: %s", rec.Body.String())
			}
		})
	}
}

// If-None-Match is a list, may be "*", and is compared weakly for GET and HEAD.
// A missed match costs a full body, which for an asset has no size bound.
func TestIfNoneMatchHandlesListsAndWildcard(t *testing.T) {
	env := newAPITestEnv(t)
	created := importTextBook(t, env, "Revalidate", "", "art.md", "body")
	sourceID := env.currentSourceID(t, created.Meta.ID)
	url := "/api/shelves/default_shelf/books/" + created.Meta.ID +
		"/sources/" + sourceID + "/assets/img-0001.png"

	env.writeSourceAsset(t, created.Meta.ID, sourceID, "img-0001.png", []byte("fake png bytes"))

	rec := env.do(httptest.NewRequest(http.MethodGet, url, nil))
	assertStatus(t, rec, http.StatusOK)
	etag := rec.Header().Get("ETag")
	if etag == "" {
		t.Fatal("no ETag to revalidate against")
	}

	revalidates := map[string]string{
		"exact":             etag,
		"list":              `W/"other-1", ` + etag,
		"list with spacing": etag + ` , W/"other-2"`,
		"wildcard":          "*",
		"strong spelling":   strings.TrimPrefix(etag, "W/"),
	}
	for name, header := range revalidates {
		t.Run(name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, url, nil)
			req.Header.Set("If-None-Match", header)
			rec := env.do(req)
			assertStatus(t, rec, http.StatusNotModified)
		})
	}

	misses := map[string]string{
		"unrelated tag":  `W/"nothing-like-it"`,
		"unrelated list": `W/"a", W/"b"`,
		"empty":          "",
	}
	for name, header := range misses {
		t.Run(name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, url, nil)
			if header != "" {
				req.Header.Set("If-None-Match", header)
			}
			rec := env.do(req)
			assertStatus(t, rec, http.StatusOK)
		})
	}
}
