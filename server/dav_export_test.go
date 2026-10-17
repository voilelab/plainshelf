package server

import (
	"bytes"
	"encoding/xml"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/voilelab/plainshelf/internal/fsutil"
	"github.com/voilelab/plainshelf/shelf"
)

type davTestEnv struct {
	handler   http.Handler
	tokenFile string
	token     string
	libRoot   string
}

// newDAVTestEnv exports one writable shelf, "default_shelf", with a few files
// of each kind already under app/.
func newDAVTestEnv(t *testing.T) *davTestEnv {
	t.Helper()
	libRoot := t.TempDir()
	app, err := NewApp(&AppConf{
		Shelves:   []*shelf.ShelfConfWithID{{ID: "default_shelf", ShelfConf: shelf.ShelfConf{LibRoot: libRoot}}},
		StorePath: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("NewApp: %v", err)
	}
	t.Cleanup(func() { app.Close() })
	waitForShelves(t, app)

	for name, body := range map[string]string{
		"shelf.json":                     `{"schema_version":1}`,
		"books/a.bookpkg/book.json":      `{"title":"A"}`,
		"books/f/b.bookpkg/book.json":    `{"title":"B"}`,
		"trash/books/c.bookpkg/x.txt":    "c",
		"app/book-cache-abc.json":        `{"schema_version":2}`,
		"app/book-cache-abc.json.zz.tmp": "partial",
		"app/scan-cache.json":            "{}",
		"app/library.lock":               "",
		"app/tmp/upload":                 "x",
		"stray.txt":                      "not a shelf file",
	} {
		writeTestFile(t, filepath.Join(libRoot, filepath.FromSlash(name)), body)
	}

	tokenFile := filepath.Join(t.TempDir(), "dav.token")
	if err = EnsureDAVExportToken(tokenFile); err != nil {
		t.Fatalf("EnsureDAVExportToken: %v", err)
	}
	token, err := readDAVExportToken(tokenFile)
	if err != nil {
		t.Fatal(err)
	}
	return &davTestEnv{handler: app.DAVExportHandler(tokenFile), tokenFile: tokenFile, token: token, libRoot: libRoot}
}

func writeTestFile(t *testing.T, name, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(name, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func (e *davTestEnv) do(t *testing.T, method, target, token string, header map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	return davDo(t, e.handler, method, target, token, header)
}

func davDo(t *testing.T, h http.Handler, method, target, token string, header map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, "http://lan.example:20001/", nil)
	// Set raw so a ".." segment reaches the handler as a client could send it.
	req.URL.Path = target
	req.URL.RawPath = ""
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	for k, v := range header {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// propfindHrefs returns the hrefs of a multistatus body, decoded.
func propfindHrefs(t *testing.T, body []byte) []string {
	t.Helper()
	var ms struct {
		Responses []struct {
			Href string `xml:"href"`
		} `xml:"response"`
	}
	if err := xml.Unmarshal(body, &ms); err != nil {
		t.Fatalf("decode multistatus: %v\n%s", err, body)
	}
	hrefs := make([]string, 0, len(ms.Responses))
	for _, r := range ms.Responses {
		h, err := url.PathUnescape(r.Href)
		if err != nil {
			t.Fatal(err)
		}
		hrefs = append(hrefs, h)
	}
	slices.Sort(hrefs)
	return hrefs
}

func TestDAVExportReads(t *testing.T) {
	env := newDAVTestEnv(t)

	rec := env.do(t, http.MethodOptions, "/dav/default_shelf/", env.token, nil)
	if rec.Code != http.StatusOK || rec.Header().Get("Allow") != davExportAllow || rec.Header().Get("DAV") != "1" {
		t.Errorf("OPTIONS = %d Allow=%q DAV=%q", rec.Code, rec.Header().Get("Allow"), rec.Header().Get("DAV"))
	}

	rec = env.do(t, "PROPFIND", "/dav/default_shelf/", env.token, map[string]string{"Depth": "0"})
	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("PROPFIND depth 0 = %d: %s", rec.Code, rec.Body)
	}
	if got := propfindHrefs(t, rec.Body.Bytes()); !slices.Equal(got, []string{"/dav/default_shelf/"}) {
		t.Errorf("PROPFIND depth 0 hrefs = %q", got)
	}

	rec = env.do(t, "PROPFIND", "/dav/default_shelf/", env.token, map[string]string{"Depth": "1"})
	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("PROPFIND depth 1 = %d: %s", rec.Code, rec.Body)
	}
	want := []string{
		"/dav/default_shelf/",
		"/dav/default_shelf/app/",
		"/dav/default_shelf/books/",
		"/dav/default_shelf/shelf.json",
		"/dav/default_shelf/trash/",
	}
	if got := propfindHrefs(t, rec.Body.Bytes()); !slices.Equal(got, want) {
		t.Errorf("PROPFIND depth 1 hrefs = %q, want %q", got, want)
	}

	rec = env.do(t, http.MethodGet, "/dav/default_shelf/books/f/b.bookpkg/book.json", env.token, nil)
	if rec.Code != http.StatusOK || rec.Body.String() != `{"title":"B"}` {
		t.Errorf("GET = %d %q", rec.Code, rec.Body)
	}
	rec = env.do(t, http.MethodHead, "/dav/default_shelf/books/f/b.bookpkg/book.json", env.token, nil)
	if rec.Code != http.StatusOK || rec.Body.Len() != 0 || rec.Header().Get("Content-Length") != "13" {
		t.Errorf("HEAD = %d len=%d Content-Length=%q", rec.Code, rec.Body.Len(), rec.Header().Get("Content-Length"))
	}
}

func TestDAVExportRefusesInfiniteDepth(t *testing.T) {
	env := newDAVTestEnv(t)
	for _, depth := range []string{"", "infinity"} {
		rec := env.do(t, "PROPFIND", "/dav/default_shelf/", env.token, map[string]string{"Depth": depth})
		if rec.Code != http.StatusForbidden {
			t.Errorf("PROPFIND Depth %q = %d, want 403", depth, rec.Code)
		}
	}
}

func TestDAVExportRefusesWrites(t *testing.T) {
	env := newDAVTestEnv(t)
	target := "/dav/default_shelf/books/a.bookpkg/book.json"
	header := map[string]string{"Destination": "http://lan.example:20001/dav/default_shelf/books/moved.json"}

	for _, method := range []string{http.MethodPut, http.MethodDelete, "MKCOL", "MOVE", "COPY", "PROPPATCH", "LOCK", "UNLOCK", http.MethodPost} {
		for _, token := range []string{"", "wrong", env.token} {
			rec := env.do(t, method, target, token, header)
			if rec.Code != http.StatusMethodNotAllowed {
				t.Errorf("%s (token %q) = %d, want 405", method, token, rec.Code)
			}
		}
	}
	bs, err := os.ReadFile(filepath.Join(env.libRoot, "books", "a.bookpkg", "book.json"))
	if err != nil || string(bs) != `{"title":"A"}` {
		t.Errorf("target changed: %q, %v", bs, err)
	}
}

func TestDAVExportHasNoAPI(t *testing.T) {
	env := newDAVTestEnv(t)
	for _, target := range []string{"/api/shelves", "/api/shelves/default_shelf/books", "/", "/index.html", "/dav", "/dav/", "/dav/missing/"} {
		rec := env.do(t, http.MethodGet, target, env.token, nil)
		if rec.Code != http.StatusNotFound {
			t.Errorf("GET %s = %d, want 404", target, rec.Code)
		}
	}
}

func TestDAVExportToken(t *testing.T) {
	env := newDAVTestEnv(t)
	target := "/dav/default_shelf/shelf.json"

	for _, header := range []map[string]string{nil, {"Authorization": "Bearer wrong"}, {"Authorization": "Basic " + env.token}, {"X-PlainShelf-Token": env.token}} {
		rec := env.do(t, http.MethodGet, target, "", header)
		if rec.Code != http.StatusUnauthorized || rec.Header().Get("WWW-Authenticate") == "" {
			t.Errorf("GET with %v = %d, want 401 with a challenge", header, rec.Code)
		}
	}
	if rec := env.do(t, http.MethodGet, target, env.token, nil); rec.Code != http.StatusOK {
		t.Fatalf("GET with token = %d", rec.Code)
	}

	// Persisted: a second start keeps the same token.
	if err := EnsureDAVExportToken(env.tokenFile); err != nil {
		t.Fatal(err)
	}
	if again, _ := readDAVExportToken(env.tokenFile); again != env.token {
		t.Errorf("token changed on restart: %q -> %q", env.token, again)
	}
	if info, err := os.Stat(env.tokenFile); err != nil || info.Mode().Perm()&0o077 != 0 {
		t.Errorf("token file mode = %v, %v; want no group or other access", info.Mode(), err)
	}

	// Rotation takes effect on the next request, without a restart.
	rotated, err := RotateDAVExportToken(env.tokenFile)
	if err != nil {
		t.Fatal(err)
	}
	if rotated == env.token {
		t.Fatal("rotation kept the token")
	}
	if rec := env.do(t, http.MethodGet, target, env.token, nil); rec.Code != http.StatusUnauthorized {
		t.Errorf("old token after rotation = %d, want 401", rec.Code)
	}
	if rec := env.do(t, http.MethodGet, target, rotated, nil); rec.Code != http.StatusOK {
		t.Errorf("new token after rotation = %d, want 200", rec.Code)
	}

	// A missing token file fails closed.
	if err := os.Remove(env.tokenFile); err != nil {
		t.Fatal(err)
	}
	if rec := env.do(t, http.MethodGet, target, rotated, nil); rec.Code != http.StatusUnauthorized {
		t.Errorf("GET without a token file = %d, want 401", rec.Code)
	}
}

func TestDAVExportOnlyShelfFiles(t *testing.T) {
	env := newDAVTestEnv(t)

	rec := env.do(t, "PROPFIND", "/dav/default_shelf/app/", env.token, map[string]string{"Depth": "1"})
	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("PROPFIND app/ = %d: %s", rec.Code, rec.Body)
	}
	// The shelf exports its own book cache beside the planted one.
	got := propfindHrefs(t, rec.Body.Bytes())
	if !slices.Contains(got, "/dav/default_shelf/app/book-cache-abc.json") {
		t.Errorf("PROPFIND app/ hrefs = %q, missing book-cache-abc.json", got)
	}
	for _, href := range got {
		name := strings.TrimPrefix(href, "/dav/default_shelf/app/")
		if name != "" && (!strings.HasPrefix(name, "book-cache-") || !strings.HasSuffix(name, ".json")) {
			t.Errorf("PROPFIND app/ lists %s", href)
		}
	}

	for _, name := range []string{"app/library.lock", "app/scan-cache.json", "app/book-cache-abc.json.zz.tmp", "app/tmp/upload", "app/tmp/", "stray.txt"} {
		target := "/dav/default_shelf/" + name
		if rec := env.do(t, http.MethodGet, target, env.token, nil); rec.Code != http.StatusNotFound {
			t.Errorf("GET %s = %d, want 404", name, rec.Code)
		}
		if rec := env.do(t, "PROPFIND", target, env.token, map[string]string{"Depth": "0"}); rec.Code != http.StatusNotFound {
			t.Errorf("PROPFIND %s = %d, want 404", name, rec.Code)
		}
	}

	for _, target := range []string{
		"/dav/default_shelf/books/../app/library.lock",
		"/dav/default_shelf/../../etc/passwd",
		"/dav/../dav/default_shelf/shelf.json",
		`/dav/default_shelf/books\..\app\library.lock`,
	} {
		if rec := env.do(t, http.MethodGet, target, env.token, nil); rec.Code != http.StatusBadRequest {
			t.Errorf("GET %s = %d, want 400", target, rec.Code)
		}
	}
}

func TestDAVExportSkipsVirtualShelves(t *testing.T) {
	app := newTestApp(t)
	root, err := os.OpenRoot(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { root.Close() })
	if err := app.AddVirtualShelf(shelf.ShelfConfWithID{ID: "remote"}, fsutil.NewRootFS(root), "test://remote"); err != nil {
		t.Fatal(err)
	}
	tokenFile := filepath.Join(t.TempDir(), "dav.token")
	token, err := RotateDAVExportToken(tokenFile)
	if err != nil {
		t.Fatal(err)
	}
	rec := davDo(t, app.DAVExportHandler(tokenFile), "PROPFIND", "/dav/remote/", token, map[string]string{"Depth": "0"})
	if rec.Code != http.StatusNotFound {
		t.Errorf("PROPFIND a virtual shelf = %d, want 404", rec.Code)
	}
}

// TestDAVExportConformanceBytes reads every conformance case back through the
// export, walking it with PROPFIND as a client would, and requires the same set
// of shelf files with the same bytes.
func TestDAVExportConformanceBytes(t *testing.T) {
	casesDir, err := filepath.Abs("../shelf/testdata/conformance/cases")
	if err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(casesDir)
	if err != nil {
		t.Fatal(err)
	}

	var shelves []*shelf.ShelfConfWithID
	for _, e := range entries {
		// Read-only, so the fixture tree is never written.
		shelves = append(shelves, &shelf.ShelfConfWithID{
			ID:        e.Name(),
			ShelfConf: shelf.ShelfConf{LibRoot: filepath.Join(casesDir, e.Name(), "shelf"), ReadOnly: true},
		})
	}
	app, err := NewApp(&AppConf{Shelves: shelves, StorePath: t.TempDir()})
	if err != nil {
		t.Fatalf("NewApp: %v", err)
	}
	t.Cleanup(func() { app.Close() })
	waitForShelves(t, app)

	tokenFile := filepath.Join(t.TempDir(), "dav.token")
	token, err := RotateDAVExportToken(tokenFile)
	if err != nil {
		t.Fatal(err)
	}
	h := app.DAVExportHandler(tokenFile)

	for _, s := range shelves {
		t.Run(s.ID, func(t *testing.T) {
			prefix := davExportPrefix + s.ID + "/"
			got := map[string][]byte{}
			queue := []string{prefix}
			for len(queue) > 0 {
				dir := queue[0]
				queue = queue[1:]
				rec := davDo(t, h, "PROPFIND", dir, token, map[string]string{"Depth": "1"})
				if rec.Code != http.StatusMultiStatus {
					t.Fatalf("PROPFIND %s = %d", dir, rec.Code)
				}
				for _, href := range propfindHrefs(t, rec.Body.Bytes()) {
					switch {
					case href == dir:
					case strings.HasSuffix(href, "/"):
						queue = append(queue, href)
					default:
						rec := davDo(t, h, http.MethodGet, href, token, nil)
						if rec.Code != http.StatusOK {
							t.Fatalf("GET %s = %d", href, rec.Code)
						}
						got[strings.TrimPrefix(href, prefix)] = rec.Body.Bytes()
					}
				}
			}

			want := map[string][]byte{}
			err := filepath.WalkDir(s.LibRoot, func(p string, d fs.DirEntry, err error) error {
				if err != nil || d.IsDir() {
					return err
				}
				rel, _ := filepath.Rel(s.LibRoot, p)
				rel = filepath.ToSlash(rel)
				if shelf.IsShelfFile(rel) {
					bs, err := os.ReadFile(p)
					want[rel] = bs
					return err
				}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			if len(want) == 0 {
				t.Fatal("fixture has no files")
			}
			for name, bs := range want {
				if !bytes.Equal(got[name], bs) {
					t.Errorf("%s: exported bytes differ (got %d bytes, want %d)", name, len(got[name]), len(bs))
				}
			}
			for name := range got {
				if _, ok := want[name]; !ok {
					t.Errorf("%s exported but not a fixture shelf file", name)
				}
			}
		})
	}
}

func TestDAVExportConf(t *testing.T) {
	ok := DAVExportConf{Addr: "0.0.0.0:20001", ReadTimeout: "60s", WriteTimeout: "60s", TokenFile: "dav.token"}
	if err := ok.validate("127.0.0.1:20000"); err != nil {
		t.Errorf("valid conf rejected: %v", err)
	}
	for name, conf := range map[string]DAVExportConf{
		"no addr":       {TokenFile: "t"},
		"same addr":     {Addr: "127.0.0.1:20000", TokenFile: "t"},
		"no token_file": {Addr: "0.0.0.0:20001"},
	} {
		if err := conf.validate("127.0.0.1:20000"); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

// TestShippedConfigOpensNoExport: the shipped example, unchanged, exports
// nothing.
func TestShippedConfigOpensNoExport(t *testing.T) {
	conf, err := loadAppConf("../cmd/plainshelf-srv/conf/config.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if conf.DAVExport != nil {
		t.Errorf("shipped config enables dav_export: %+v", conf.DAVExport)
	}
}

func TestIsShelfFileMatchesPathClean(t *testing.T) {
	// The FS hands IsShelfFile cleaned paths only; a hidden file must stay
	// hidden however the request spells it.
	fsys := davFS{}
	for _, name := range []string{"/app/./library.lock", "//app//library.lock", "/books/../app/library.lock", "app/tmp/../library.lock"} {
		if rel, ok := fsys.rel(name); ok {
			t.Errorf("rel(%q) = %q, exported", name, rel)
		}
	}
	if rel, ok := fsys.rel("/"); !ok || rel != "." {
		t.Errorf(`rel("/") = %q, %v`, rel, ok)
	}
}
