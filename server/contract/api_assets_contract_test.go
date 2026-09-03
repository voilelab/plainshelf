package contract_test

import (
	"archive/zip"
	"bytes"
	"io"
	"maps"
	"net/http"
	"net/http/httptest"
	neturl "net/url"
	"slices"
	"strconv"
	"strings"
	"testing"
)

func TestAPISourceAssetContract(t *testing.T) {
	env := newAPITestEnv(t)
	created := importTextBook(t, env, "Illustrated", "", "art.md", "body")
	sourceID := env.currentSourceID(t, created.Meta.ID)
	asset := func(name string) string { return assetURL(created.Meta.ID, sourceID, name) }

	// Nothing has been placed under assets/ yet.
	rec := env.get(asset("img-0001.png"))
	assertStatus(t, rec, http.StatusNotFound)

	pngBytes := []byte("fake png bytes")
	env.writeSourceAsset(t, created.Meta.ID, sourceID, "img-0001.png", pngBytes)

	rec = env.get(asset("img-0001.png"))
	assertStatus(t, rec, http.StatusOK)
	assertContentType(t, rec, "image/png")
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
	rec = env.getIfNoneMatch(asset("img-0001.png"), etag)
	assertStatus(t, rec, http.StatusNotModified)
	if rec.Body.Len() != 0 {
		t.Fatalf("304 response body = %q, want empty", rec.Body.String())
	}

	// Each stored extension keeps its own content type.
	env.writeSourceAsset(t, created.Meta.ID, sourceID, "img-0002.webp", []byte("fake webp bytes"))
	rec = env.get(asset("img-0002.webp"))
	assertStatus(t, rec, http.StatusOK)
	assertContentType(t, rec, "image/webp")

	// ServeMux has already unescaped the wildcard, so a name carrying a literal
	// percent escape must survive addressing intact. Decoding it a second time
	// would land on "chart one.png" instead, which exists here precisely so a
	// regression serves the wrong bytes rather than merely 404ing.
	env.writeSourceAsset(t, created.Meta.ID, sourceID, "chart%20one.png", []byte("percent name"))
	env.writeSourceAsset(t, created.Meta.ID, sourceID, "chart one.png", []byte("space name"))

	for _, tc := range []struct{ name, want string }{
		{"chart%2520one.png", "percent name"},
		{"chart%20one.png", "space name"},
	} {
		rec = env.get(asset(tc.name))
		assertStatus(t, rec, http.StatusOK)
		if got := rec.Body.String(); got != tc.want {
			t.Fatalf("asset %q = %q, want %q", tc.name, got, tc.want)
		}
	}

	// A GET pattern also matches HEAD. The headers must be identical while the
	// asset itself is never read: the empty recorder body is what shows the
	// copy was skipped, since httptest does not suppress it the way net/http
	// would.
	rec = env.request(http.MethodHead, asset("img-0001.png"), nil)
	assertStatus(t, rec, http.StatusOK)
	if rec.Body.Len() != 0 {
		t.Fatalf("HEAD response body = %q, want empty", rec.Body.String())
	}
	if got := rec.Header().Get("Content-Length"); got != strconv.Itoa(len(pngBytes)) {
		t.Fatalf("HEAD Content-Length = %q, want %d", got, len(pngBytes))
	}
	assertContentType(t, rec, "image/png")

	// A missing book or source is a 404, not a 500.
	rec = env.get(assetURL("no-such-book", sourceID, "img-0001.png"))
	assertStatus(t, rec, http.StatusNotFound)
	rec = env.get(assetURL(created.Meta.ID, "no-such-source", "img-0001.png"))
	assertStatus(t, rec, http.StatusNotFound)

	// POST and PATCH still have no meaning on an asset; PUT and DELETE do.
	for _, method := range []string{http.MethodPost, http.MethodPatch} {
		rec = env.request(method, asset("img-0001.png"), nil)
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("%s asset status = %d, want %d", method, rec.Code, http.StatusMethodNotAllowed)
		}
	}
}

func TestAPISourceAssetWriteContract(t *testing.T) {
	env := newAPITestEnv(t)
	created := importTextBook(t, env, "Editable Art", "", "art.md", "body")
	sourceID := env.currentSourceID(t, created.Meta.ID)
	asset := func(name string) string { return assetURL(created.Meta.ID, sourceID, name) }

	// Uploading creates the directory and the file.
	pngBytes := []byte("fake png bytes")
	rec := env.put(asset("img-0001.png"), bytes.NewReader(pngBytes))
	assertStatus(t, rec, http.StatusNoContent)

	rec = env.get(asset("img-0001.png"))
	assertStatus(t, rec, http.StatusOK)
	if !bytes.Equal(rec.Body.Bytes(), pngBytes) {
		t.Fatalf("stored asset = %q, want %q", rec.Body.Bytes(), pngBytes)
	}

	// Uploading again under the same name replaces it.
	replaced := []byte("replacement bytes")
	rec = env.put(asset("img-0001.png"), bytes.NewReader(replaced))
	assertStatus(t, rec, http.StatusNoContent)
	rec = env.get(asset("img-0001.png"))
	if !bytes.Equal(rec.Body.Bytes(), replaced) {
		t.Fatalf("replaced asset = %q, want %q", rec.Body.Bytes(), replaced)
	}

	// The name is validated on the way in exactly as it is on the way out, so
	// a file the read path could never serve cannot be written either.
	for _, assetName := range []string{"..%2fescaped.png", ".hidden.png", "notes.txt", "img-0002"} {
		rec = env.put(asset(assetName), bytes.NewReader(pngBytes))
		assertStatus(t, rec, http.StatusBadRequest)
	}

	// Oversized uploads are refused rather than spooled.
	rec = env.put(asset("img-0003.png"), bytes.NewReader(bytes.Repeat([]byte{'x'}, maxBinaryUploadSize+1)))
	assertStatus(t, rec, http.StatusRequestEntityTooLarge)

	// Deleting removes it; deleting again reports the miss rather than
	// succeeding quietly, since an asset is addressed by name.
	rec = env.delete(asset("img-0001.png"))
	assertStatus(t, rec, http.StatusNoContent)
	rec = env.get(asset("img-0001.png"))
	assertStatus(t, rec, http.StatusNotFound)
	rec = env.delete(asset("img-0001.png"))
	assertStatus(t, rec, http.StatusNotFound)
}

// A replaced illustration keeps its URL - the reader derives it from the file
// name in the text, and nothing records a version to bust a cache with - so a
// client that may reuse the old bytes would show the wrong picture.
func TestAssetRevalidationSurvivesAReplacement(t *testing.T) {
	env := newAPITestEnv(t)
	created := importTextBook(t, env, "Replaced Art", "", "art.md", "body")
	sourceID := env.currentSourceID(t, created.Meta.ID)
	url := assetURL(created.Meta.ID, sourceID, "img-0001.png")

	rec := env.put(url, strings.NewReader("first bytes"))
	assertStatus(t, rec, http.StatusNoContent)

	rec = env.get(url)
	assertStatus(t, rec, http.StatusOK)
	firstETag := rec.Header().Get("ETag")
	if got := rec.Header().Get("Cache-Control"); !strings.Contains(got, "no-cache") {
		t.Fatalf("asset Cache-Control = %q, want it to force revalidation", got)
	}

	// A cover may be cached for a day because its URL gains a cache-busting key
	// when it changes; an asset URL never changes, hence the difference.
	coverURL := bookURL(created.Meta.ID, "cover")
	rec = env.putContent(coverURL, "image/png", strings.NewReader("cover"))
	assertStatus(t, rec, http.StatusNoContent)
	rec = env.get(coverURL)
	if got := rec.Header().Get("Cache-Control"); !strings.Contains(got, "max-age=86400") {
		t.Fatalf("cover Cache-Control = %q, want it to stay cacheable", got)
	}

	// Replacing changes the validator, so a client holding the old one is told
	// to take the new bytes rather than being answered 304.
	rec = env.put(url, strings.NewReader("second bytes, longer"))
	assertStatus(t, rec, http.StatusNoContent)

	rec = env.getIfNoneMatch(url, firstETag)
	assertStatus(t, rec, http.StatusOK)
	if got := rec.Body.String(); got != "second bytes, longer" {
		t.Fatalf("revalidated asset = %q, want the replacement", got)
	}

	// And once it is deleted, the same conditional request reports the miss
	// rather than confirming a copy that is no longer there.
	rec = env.delete(url)
	assertStatus(t, rec, http.StatusNoContent)

	rec = env.getIfNoneMatch(url, firstETag)
	assertStatus(t, rec, http.StatusNotFound)
}

// The asset route reaches the filesystem by name, so it gets its own traversal
// cases rather than trusting the shelf-level test alone.
func TestAPISourceAssetRejectsUnsafeNames(t *testing.T) {
	env := newAPITestEnv(t)
	created := importTextBook(t, env, "Unsafe Assets", "", "art.md", "secret body")
	sourceID := env.currentSourceID(t, created.Meta.ID)
	asset := func(name string) string { return assetURL(created.Meta.ID, sourceID, name) }

	env.writeSourceAsset(t, created.Meta.ID, sourceID, "img-0001.png", []byte("fake png bytes"))

	// A name is served exactly as addressed. Trimming it would make " lead.png"
	// unreachable and, with both files present, quietly answer with the other
	// one - the same failure the double-decode used to cause.
	env.writeSourceAsset(t, created.Meta.ID, sourceID, " lead.png", []byte("space prefixed"))
	env.writeSourceAsset(t, created.Meta.ID, sourceID, "lead.png", []byte("plain"))

	for _, tc := range []struct{ name, want string }{
		{"%20lead.png", "space prefixed"},
		{"lead.png", "plain"},
	} {
		rec := env.get(asset(tc.name))
		assertStatus(t, rec, http.StatusOK)
		if got := rec.Body.String(); got != tc.want {
			t.Fatalf("asset %q = %q, want %q", tc.name, got, tc.want)
		}
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
			rec := env.get(asset(assetName))
			assertStatus(t, rec, http.StatusBadRequest)
			if strings.Contains(rec.Body.String(), "secret body") {
				t.Fatalf("response leaked file contents: %s", rec.Body.String())
			}
		})
	}
}

// assetsBundleURL addresses a source's batch assets.zip endpoint, naming the
// files to pack with repeated `name` query parameters. With no names it packs
// the whole assets/ directory.
func assetsBundleURL(bookID, sourceID string, names ...string) string {
	base := sourceURL(bookID, sourceID, "assets.zip")
	if len(names) == 0 {
		return base
	}
	query := neturl.Values{}
	for _, name := range names {
		query.Add("name", name)
	}
	return base + "?" + query.Encode()
}

// readZipBundle decodes a returned assets.zip into a name→bytes map, failing the
// test if the archive is malformed or carries a duplicate entry.
func readZipBundle(t *testing.T, body []byte) map[string][]byte {
	t.Helper()

	reader, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		t.Fatalf("open zip bundle: %v", err)
	}

	entries := make(map[string][]byte, len(reader.File))
	for _, file := range reader.File {
		rc, err := file.Open()
		if err != nil {
			t.Fatalf("open zip entry %q: %v", file.Name, err)
		}
		data, err := io.ReadAll(rc)
		rc.Close() //nolint:errcheck // read already succeeded; nothing to salvage
		if err != nil {
			t.Fatalf("read zip entry %q: %v", file.Name, err)
		}
		if _, dup := entries[file.Name]; dup {
			t.Fatalf("zip bundle has a duplicate entry %q", file.Name)
		}
		entries[file.Name] = data
	}
	return entries
}

func bundleEntryNames(entries map[string][]byte) []string {
	return slices.Sorted(maps.Keys(entries))
}

// The batch endpoint packs a source's illustrations into one zip so a download
// client pays a single round trip instead of one per figure.
func TestAPISourceAssetsBundleContract(t *testing.T) {
	env := newAPITestEnv(t)
	created := importTextBook(t, env, "Bundled Art", "", "art.md", "body")
	sourceID := env.currentSourceID(t, created.Meta.ID)

	png := []byte("fake png bytes")
	webp := []byte("fake webp bytes")
	env.writeSourceAsset(t, created.Meta.ID, sourceID, "img-0001.png", png)
	env.writeSourceAsset(t, created.Meta.ID, sourceID, "img-0002.webp", webp)

	// A named subset is packed; a name with no file behind it is skipped rather
	// than failing the request; and the entry names are flat - no assets/ prefix
	// and no separator - so the client writes each straight back under its name.
	rec := env.get(assetsBundleURL(created.Meta.ID, sourceID, "img-0001.png", "img-0002.webp", "missing.png"))
	assertStatus(t, rec, http.StatusOK)
	assertContentType(t, rec, "application/zip")

	entries := readZipBundle(t, rec.Body.Bytes())
	if got := bundleEntryNames(entries); len(got) != 2 {
		t.Fatalf("bundle entries = %v, want img-0001.png and img-0002.webp", got)
	}
	if !bytes.Equal(entries["img-0001.png"], png) {
		t.Fatalf("bundle img-0001.png = %q, want %q", entries["img-0001.png"], png)
	}
	if !bytes.Equal(entries["img-0002.webp"], webp) {
		t.Fatalf("bundle img-0002.webp = %q, want %q", entries["img-0002.webp"], webp)
	}

	// With no names given the whole assets/ directory is packed, so a client that
	// wants everything need not enumerate it first.
	rec = env.get(assetsBundleURL(created.Meta.ID, sourceID))
	assertStatus(t, rec, http.StatusOK)
	entries = readZipBundle(t, rec.Body.Bytes())
	if got := bundleEntryNames(entries); len(got) != 2 || entries["img-0001.png"] == nil || entries["img-0002.webp"] == nil {
		t.Fatalf("whole-directory bundle = %v, want both assets", got)
	}

	// Only GET (and the HEAD it implies) has meaning here; the archive is a read.
	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete} {
		rec = env.request(method, assetsBundleURL(created.Meta.ID, sourceID), nil)
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("%s assets.zip status = %d, want %d", method, rec.Code, http.StatusMethodNotAllowed)
		}
	}

	// A missing book or source is a 404, not a 500 or an empty archive.
	rec = env.get(assetsBundleURL("no-such-book", sourceID, "img-0001.png"))
	assertStatus(t, rec, http.StatusNotFound)
	rec = env.get(assetsBundleURL(created.Meta.ID, "no-such-source", "img-0001.png"))
	assertStatus(t, rec, http.StatusNotFound)
}

// The batch read shares getAsset's security boundary: under protect_read it is
// refused without the token and served with it.
func TestAPISourceAssetsBundleFollowsTheTokenGate(t *testing.T) {
	security := localTokenSecurity()
	security.ProtectRead = true
	env := newAPITestEnv(t, withSecurity(security))
	created := importTextBook(t, env, "Protected Bundle", "", "art.md", "body")
	// Under protect_read even reading the source list needs the token, so take
	// the source id from the import result rather than a tokenless GET.
	sourceID := created.Meta.CurrentSource
	env.writeSourceAsset(t, created.Meta.ID, sourceID, "img-0001.png", []byte("bytes"))
	url := assetsBundleURL(created.Meta.ID, sourceID, "img-0001.png")

	rec := env.doRaw(httptest.NewRequest(http.MethodGet, url, nil))
	assertStatus(t, rec, http.StatusUnauthorized)

	req := httptest.NewRequest(http.MethodGet, url, nil)
	req.Header.Set("Authorization", "Bearer "+env.app.SecurityToken())
	rec = env.doRaw(req)
	assertStatus(t, rec, http.StatusOK)
	if _, ok := readZipBundle(t, rec.Body.Bytes())["img-0001.png"]; !ok {
		t.Fatal("authorized bundle is missing its asset")
	}
}

// Each requested name is validated up front, the way getAsset validates its
// path segment, so one unsafe name is a 400 for the whole request rather than a
// truncated archive - and nothing on the shelf leaks on the way to that 400.
func TestAPISourceAssetsBundleRejectsUnsafeNames(t *testing.T) {
	env := newAPITestEnv(t)
	created := importTextBook(t, env, "Unsafe Bundle", "", "art.md", "secret body")
	sourceID := env.currentSourceID(t, created.Meta.ID)
	env.writeSourceAsset(t, created.Meta.ID, sourceID, "img-0001.png", []byte("fake png bytes"))

	// source.txt, book.json and friends really do sit above assets/, so a 400
	// proves the name was refused rather than merely missing its target. The
	// valid img-0001.png alongside each unsafe name must not smuggle a 200 or a
	// partial archive carrying its bytes.
	for _, name := range []string{
		"../source.txt",
		"../../book.json",
		"sub/img-0001.png",
		`..\source.txt`,
		"/etc/hostname",
		".hidden.png",
		"source.txt",
		"img-0001",
	} {
		t.Run(name, func(t *testing.T) {
			rec := env.get(assetsBundleURL(created.Meta.ID, sourceID, "img-0001.png", name))
			assertStatus(t, rec, http.StatusBadRequest)
			if strings.Contains(rec.Body.String(), "secret body") {
				t.Fatalf("response leaked file contents: %s", rec.Body.String())
			}
			if strings.Contains(rec.Body.String(), "fake png bytes") {
				t.Fatalf("response leaked a partial archive: %s", rec.Body.String())
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
	url := assetURL(created.Meta.ID, sourceID, "img-0001.png")

	env.writeSourceAsset(t, created.Meta.ID, sourceID, "img-0001.png", []byte("fake png bytes"))

	rec := env.get(url)
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
			assertStatus(t, env.getIfNoneMatch(url, header), http.StatusNotModified)
		})
	}

	misses := map[string]string{
		"unrelated tag":  `W/"nothing-like-it"`,
		"unrelated list": `W/"a", W/"b"`,
		"empty":          "",
	}
	for name, header := range misses {
		t.Run(name, func(t *testing.T) {
			assertStatus(t, env.getIfNoneMatch(url, header), http.StatusOK)
		})
	}
}
