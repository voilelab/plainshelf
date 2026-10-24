package sources_test

import (
	"archive/zip"
	"bytes"
	"io"
	"maps"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/voilelab/plainshelf/server/contract/apitest"
)

func TestAPISourceAssetContract(t *testing.T) {
	env := apitest.New(t)
	created := apitest.ImportTextBook(t, env, "Illustrated", "", "art.md", "body")
	sourceID := env.CurrentSourceID(t, created.Meta.ID)
	asset := func(name string) string { return apitest.AssetURL(created.Meta.ID, sourceID, name) }

	rec := env.Get(asset("img-0001.png"))
	apitest.AssertStatus(t, rec, http.StatusNotFound)

	pngBytes := []byte("fake png bytes")
	env.WriteSourceAsset(t, created.Meta.ID, sourceID, "img-0001.png", pngBytes)

	rec = env.Get(asset("img-0001.png"))
	apitest.AssertStatus(t, rec, http.StatusOK)
	apitest.AssertContentType(t, rec, "image/png")
	if !bytes.Equal(rec.Body.Bytes(), pngBytes) {
		t.Fatalf("asset bytes = %q, want %q", rec.Body.Bytes(), pngBytes)
	}

	if got := rec.Header().Get("Content-Length"); got != strconv.Itoa(len(pngBytes)) {
		t.Fatalf("asset Content-Length = %q, want %d", got, len(pngBytes))
	}

	if got := rec.Header().Get("Cache-Control"); got != "public, no-cache" {
		t.Fatalf("asset Cache-Control = %q, want public, no-cache", got)
	}

	etag := rec.Header().Get("ETag")
	if etag == "" {
		t.Fatal("asset response carries no ETag")
	}
	rec = env.GetIfNoneMatch(asset("img-0001.png"), etag)
	apitest.AssertStatus(t, rec, http.StatusNotModified)
	if rec.Body.Len() != 0 {
		t.Fatalf("304 response body = %q, want empty", rec.Body.String())
	}

	env.WriteSourceAsset(t, created.Meta.ID, sourceID, "img-0002.webp", []byte("fake webp bytes"))
	rec = env.Get(asset("img-0002.webp"))
	apitest.AssertStatus(t, rec, http.StatusOK)
	apitest.AssertContentType(t, rec, "image/webp")

	env.WriteSourceAsset(t, created.Meta.ID, sourceID, "chart%20one.png", []byte("percent name"))
	env.WriteSourceAsset(t, created.Meta.ID, sourceID, "chart one.png", []byte("space name"))

	for _, tc := range []struct{ name, want string }{
		{"chart%2520one.png", "percent name"},
		{"chart%20one.png", "space name"},
	} {
		rec = env.Get(asset(tc.name))
		apitest.AssertStatus(t, rec, http.StatusOK)
		if got := rec.Body.String(); got != tc.want {
			t.Fatalf("asset %q = %q, want %q", tc.name, got, tc.want)
		}
	}

	// A GET pattern also matches HEAD. The headers must be identical while the
	// asset itself is never read: the empty recorder body is what shows the
	// copy was skipped, since httptest does not suppress it the way net/http
	// would.
	rec = env.Request(http.MethodHead, asset("img-0001.png"), nil)
	apitest.AssertStatus(t, rec, http.StatusOK)
	if rec.Body.Len() != 0 {
		t.Fatalf("HEAD response body = %q, want empty", rec.Body.String())
	}
	if got := rec.Header().Get("Content-Length"); got != strconv.Itoa(len(pngBytes)) {
		t.Fatalf("HEAD Content-Length = %q, want %d", got, len(pngBytes))
	}
	apitest.AssertContentType(t, rec, "image/png")

	rec = env.Get(apitest.AssetURL("no-such-book", sourceID, "img-0001.png"))
	apitest.AssertStatus(t, rec, http.StatusNotFound)
	rec = env.Get(apitest.AssetURL(created.Meta.ID, "no-such-source", "img-0001.png"))
	apitest.AssertStatus(t, rec, http.StatusNotFound)

	for _, method := range []string{http.MethodPost, http.MethodPatch} {
		rec = env.Request(method, asset("img-0001.png"), nil)
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("%s asset status = %d, want %d", method, rec.Code, http.StatusMethodNotAllowed)
		}
	}
}

func TestAPISourceAssetWriteContract(t *testing.T) {
	env := apitest.New(t)
	created := apitest.ImportTextBook(t, env, "Editable Art", "", "art.md", "body")
	sourceID := env.CurrentSourceID(t, created.Meta.ID)
	asset := func(name string) string { return apitest.AssetURL(created.Meta.ID, sourceID, name) }

	pngBytes := []byte("fake png bytes")
	rec := env.Put(asset("img-0001.png"), bytes.NewReader(pngBytes))
	apitest.AssertStatus(t, rec, http.StatusNoContent)

	rec = env.Get(asset("img-0001.png"))
	apitest.AssertStatus(t, rec, http.StatusOK)
	if !bytes.Equal(rec.Body.Bytes(), pngBytes) {
		t.Fatalf("stored asset = %q, want %q", rec.Body.Bytes(), pngBytes)
	}

	replaced := []byte("replacement bytes")
	rec = env.Put(asset("img-0001.png"), bytes.NewReader(replaced))
	apitest.AssertStatus(t, rec, http.StatusNoContent)
	rec = env.Get(asset("img-0001.png"))
	if !bytes.Equal(rec.Body.Bytes(), replaced) {
		t.Fatalf("replaced asset = %q, want %q", rec.Body.Bytes(), replaced)
	}

	for _, assetName := range []string{"..%2fescaped.png", ".hidden.png", "notes.txt", "img-0002"} {
		rec = env.Put(asset(assetName), bytes.NewReader(pngBytes))
		apitest.AssertStatus(t, rec, http.StatusBadRequest)
	}

	rec = env.Put(asset("img-0003.png"), bytes.NewReader(bytes.Repeat([]byte{'x'}, apitest.MaxBinaryUploadSize+1)))
	apitest.AssertStatus(t, rec, http.StatusRequestEntityTooLarge)

	rec = env.Delete(asset("img-0001.png"))
	apitest.AssertStatus(t, rec, http.StatusNoContent)
	rec = env.Get(asset("img-0001.png"))
	apitest.AssertStatus(t, rec, http.StatusNotFound)
	rec = env.Delete(asset("img-0001.png"))
	apitest.AssertStatus(t, rec, http.StatusNotFound)
}

func TestAssetRevalidationSurvivesAReplacement(t *testing.T) {
	env := apitest.New(t)
	created := apitest.ImportTextBook(t, env, "Replaced Art", "", "art.md", "body")
	sourceID := env.CurrentSourceID(t, created.Meta.ID)
	url := apitest.AssetURL(created.Meta.ID, sourceID, "img-0001.png")

	rec := env.Put(url, strings.NewReader("first bytes"))
	apitest.AssertStatus(t, rec, http.StatusNoContent)

	rec = env.Get(url)
	apitest.AssertStatus(t, rec, http.StatusOK)
	firstETag := rec.Header().Get("ETag")
	if got := rec.Header().Get("Cache-Control"); !strings.Contains(got, "no-cache") {
		t.Fatalf("asset Cache-Control = %q, want it to force revalidation", got)
	}

	coverURL := apitest.BookURL(created.Meta.ID, "cover")
	rec = env.PutContent(coverURL, "image/png", strings.NewReader("cover"))
	apitest.AssertStatus(t, rec, http.StatusNoContent)
	rec = env.Get(coverURL)
	if got := rec.Header().Get("Cache-Control"); !strings.Contains(got, "max-age=86400") {
		t.Fatalf("cover Cache-Control = %q, want it to stay cacheable", got)
	}

	rec = env.Put(url, strings.NewReader("second bytes, longer"))
	apitest.AssertStatus(t, rec, http.StatusNoContent)

	rec = env.GetIfNoneMatch(url, firstETag)
	apitest.AssertStatus(t, rec, http.StatusOK)
	if got := rec.Body.String(); got != "second bytes, longer" {
		t.Fatalf("revalidated asset = %q, want the replacement", got)
	}

	rec = env.Delete(url)
	apitest.AssertStatus(t, rec, http.StatusNoContent)

	rec = env.GetIfNoneMatch(url, firstETag)
	apitest.AssertStatus(t, rec, http.StatusNotFound)
}

func TestAPISourceAssetRejectsUnsafeNames(t *testing.T) {
	env := apitest.New(t)
	created := apitest.ImportTextBook(t, env, "Unsafe Assets", "", "art.md", "secret body")
	sourceID := env.CurrentSourceID(t, created.Meta.ID)
	asset := func(name string) string { return apitest.AssetURL(created.Meta.ID, sourceID, name) }

	env.WriteSourceAsset(t, created.Meta.ID, sourceID, "img-0001.png", []byte("fake png bytes"))

	env.WriteSourceAsset(t, created.Meta.ID, sourceID, " lead.png", []byte("space prefixed"))
	env.WriteSourceAsset(t, created.Meta.ID, sourceID, "lead.png", []byte("plain"))

	for _, tc := range []struct{ name, want string }{
		{"%20lead.png", "space prefixed"},
		{"lead.png", "plain"},
	} {
		rec := env.Get(asset(tc.name))
		apitest.AssertStatus(t, rec, http.StatusOK)
		if got := rec.Body.String(); got != tc.want {
			t.Fatalf("asset %q = %q, want %q", tc.name, got, tc.want)
		}
	}

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
			rec := env.Get(asset(assetName))
			apitest.AssertStatus(t, rec, http.StatusBadRequest)
			if strings.Contains(rec.Body.String(), "secret body") {
				t.Fatalf("response leaked file contents: %s", rec.Body.String())
			}
		})
	}
}

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
		rc.Close()
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

func TestAPISourceAssetsBundleContract(t *testing.T) {
	env := apitest.New(t)
	created := apitest.ImportTextBook(t, env, "Bundled Art", "", "art.md", "body")
	sourceID := env.CurrentSourceID(t, created.Meta.ID)

	png := []byte("fake png bytes")
	webp := []byte("fake webp bytes")
	env.WriteSourceAsset(t, created.Meta.ID, sourceID, "img-0001.png", png)
	env.WriteSourceAsset(t, created.Meta.ID, sourceID, "img-0002.webp", webp)

	rec := env.Get(apitest.AssetsBundleURL(created.Meta.ID, sourceID, "img-0001.png", "img-0002.webp", "missing.png"))
	apitest.AssertStatus(t, rec, http.StatusOK)
	apitest.AssertContentType(t, rec, "application/zip")

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

	rec = env.Get(apitest.AssetsBundleURL(created.Meta.ID, sourceID))
	apitest.AssertStatus(t, rec, http.StatusOK)
	entries = readZipBundle(t, rec.Body.Bytes())
	if got := bundleEntryNames(entries); len(got) != 2 || entries["img-0001.png"] == nil || entries["img-0002.webp"] == nil {
		t.Fatalf("whole-directory bundle = %v, want both assets", got)
	}

	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete} {
		rec = env.Request(method, apitest.AssetsBundleURL(created.Meta.ID, sourceID), nil)
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("%s assets.zip status = %d, want %d", method, rec.Code, http.StatusMethodNotAllowed)
		}
	}

	rec = env.Get(apitest.AssetsBundleURL("no-such-book", sourceID, "img-0001.png"))
	apitest.AssertStatus(t, rec, http.StatusNotFound)
	rec = env.Get(apitest.AssetsBundleURL(created.Meta.ID, "no-such-source", "img-0001.png"))
	apitest.AssertStatus(t, rec, http.StatusNotFound)
}

func TestAPISourceAssetsBundleFollowsTheTokenGate(t *testing.T) {
	security := apitest.LocalTokenSecurity()
	security.ProtectRead = true
	env := apitest.New(t, apitest.WithSecurity(security))
	created := apitest.ImportTextBook(t, env, "Protected Bundle", "", "art.md", "body")

	sourceID := created.Meta.CurrentSource
	env.WriteSourceAsset(t, created.Meta.ID, sourceID, "img-0001.png", []byte("bytes"))
	url := apitest.AssetsBundleURL(created.Meta.ID, sourceID, "img-0001.png")

	rec := env.DoRaw(httptest.NewRequest(http.MethodGet, url, nil))
	apitest.AssertStatus(t, rec, http.StatusUnauthorized)

	req := httptest.NewRequest(http.MethodGet, url, nil)
	req.Header.Set("Authorization", "Bearer "+env.App.SecurityToken())
	rec = env.DoRaw(req)
	apitest.AssertStatus(t, rec, http.StatusOK)
	if _, ok := readZipBundle(t, rec.Body.Bytes())["img-0001.png"]; !ok {
		t.Fatal("authorized bundle is missing its asset")
	}
}

func TestAPISourceAssetsBundleRejectsUnsafeNames(t *testing.T) {
	env := apitest.New(t)
	created := apitest.ImportTextBook(t, env, "Unsafe Bundle", "", "art.md", "secret body")
	sourceID := env.CurrentSourceID(t, created.Meta.ID)
	env.WriteSourceAsset(t, created.Meta.ID, sourceID, "img-0001.png", []byte("fake png bytes"))

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
			rec := env.Get(apitest.AssetsBundleURL(created.Meta.ID, sourceID, "img-0001.png", name))
			apitest.AssertStatus(t, rec, http.StatusBadRequest)
			if strings.Contains(rec.Body.String(), "secret body") {
				t.Fatalf("response leaked file contents: %s", rec.Body.String())
			}
			if strings.Contains(rec.Body.String(), "fake png bytes") {
				t.Fatalf("response leaked a partial archive: %s", rec.Body.String())
			}
		})
	}
}

func TestIfNoneMatchHandlesListsAndWildcard(t *testing.T) {
	env := apitest.New(t)
	created := apitest.ImportTextBook(t, env, "Revalidate", "", "art.md", "body")
	sourceID := env.CurrentSourceID(t, created.Meta.ID)
	url := apitest.AssetURL(created.Meta.ID, sourceID, "img-0001.png")

	env.WriteSourceAsset(t, created.Meta.ID, sourceID, "img-0001.png", []byte("fake png bytes"))

	rec := env.Get(url)
	apitest.AssertStatus(t, rec, http.StatusOK)
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
			apitest.AssertStatus(t, env.GetIfNoneMatch(url, header), http.StatusNotModified)
		})
	}

	misses := map[string]string{
		"unrelated tag":  `W/"nothing-like-it"`,
		"unrelated list": `W/"a", W/"b"`,
		"empty":          "",
	}
	for name, header := range misses {
		t.Run(name, func(t *testing.T) {
			apitest.AssertStatus(t, env.GetIfNoneMatch(url, header), http.StatusOK)
		})
	}
}
