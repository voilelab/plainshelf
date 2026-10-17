package server

import (
	"bytes"
	"encoding/xml"
	"io/fs"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/voilelab/plainshelf/internal/fsutil"
	"github.com/voilelab/plainshelf/shelf"
)

type davTestEnv struct {
	app       *App
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
	return &davTestEnv{app: app, handler: app.DAVExportHandler(tokenFile), tokenFile: tokenFile, token: token, libRoot: libRoot}
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

func TestDAVExportShelvesHandler(t *testing.T) {
	env := newDAVTestEnv(t)
	app := newTestApp(t)
	if err := app.AddShelf(shelf.ShelfConfWithID{ID: "a", ShelfConf: shelf.ShelfConf{LibRoot: env.libRoot, ReadOnly: true}}); err != nil {
		t.Fatal(err)
	}
	if err := app.AddShelf(shelf.ShelfConfWithID{ID: "b", ShelfConf: shelf.ShelfConf{LibRoot: t.TempDir()}}); err != nil {
		t.Fatal(err)
	}
	waitForShelves(t, app)

	shared := map[string]bool{"a": true}
	h := app.DAVExportShelvesHandler(env.tokenFile, func(id string) bool { return shared[id] })
	depth := map[string]string{"Depth": "0"}
	if rec := davDo(t, h, "PROPFIND", "/dav/a/", env.token, depth); rec.Code != http.StatusMultiStatus {
		t.Errorf("PROPFIND exported shelf = %d, want 207", rec.Code)
	}
	if rec := davDo(t, h, "GET", "/dav/a/shelf.json", env.token, nil); rec.Code != http.StatusOK {
		t.Errorf("GET exported shelf file = %d, want 200", rec.Code)
	}
	for _, target := range []string{"/dav/b/", "/dav/b/shelf.json", "/dav/"} {
		if rec := davDo(t, h, "PROPFIND", target, env.token, depth); rec.Code != http.StatusNotFound {
			t.Errorf("PROPFIND %s = %d, want 404", target, rec.Code)
		}
	}
	// Still behind the token.
	if rec := davDo(t, h, "PROPFIND", "/dav/b/", "", depth); rec.Code != http.StatusUnauthorized {
		t.Errorf("PROPFIND without token = %d, want 401", rec.Code)
	}

	// Asked per request, so a changed selection applies at once.
	shared = map[string]bool{"b": true}
	if rec := davDo(t, h, "PROPFIND", "/dav/a/", env.token, depth); rec.Code != http.StatusNotFound {
		t.Errorf("PROPFIND unshared shelf = %d, want 404", rec.Code)
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
		"wildcard":      {Addr: "0.0.0.0:20001", TokenFile: "t", AllowedOrigins: []string{"*"}},
		"origin path":   {Addr: "0.0.0.0:20001", TokenFile: "t", AllowedOrigins: []string{"http://a.example/demo/"}},
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

func TestDAVExportRefusesSymlinks(t *testing.T) {
	env := newDAVTestEnv(t)
	books := filepath.Join(env.libRoot, "books")
	for link, target := range map[string]string{
		"d": "../app",
		"s": "../app/library.lock",
		"e": "/etc/passwd",
	} {
		if err := os.Symlink(target, filepath.Join(books, link)); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}
	}

	for _, target := range []string{"books/d/scan-cache.json", "books/d/tmp/upload", "books/d", "books/d/", "books/s", "books/e"} {
		for _, method := range []string{http.MethodGet, "PROPFIND"} {
			rec := env.do(t, method, "/dav/default_shelf/"+target, env.token, map[string]string{"Depth": "1"})
			if rec.Code != http.StatusNotFound {
				t.Errorf("%s %s = %d, want 404", method, target, rec.Code)
			}
		}
	}

	rec := env.do(t, "PROPFIND", "/dav/default_shelf/books/", env.token, map[string]string{"Depth": "1"})
	for _, href := range propfindHrefs(t, rec.Body.Bytes()) {
		if name := strings.TrimPrefix(href, "/dav/default_shelf/books/"); name == "d/" || name == "d" || name == "s" || name == "e" {
			t.Errorf("PROPFIND books/ lists symlink %s", href)
		}
	}
}

func TestDAVExportNotFoundIsUniform(t *testing.T) {
	env := newDAVTestEnv(t)
	// A path through a file is ENOTDIR underneath; it must read as missing.
	for _, target := range []string{"books/a.bookpkg/book.json/x", "books/nope", "app/nope", "app/library.lock"} {
		rec := env.do(t, "PROPFIND", "/dav/default_shelf/"+target, env.token, map[string]string{"Depth": "0"})
		if rec.Code != http.StatusNotFound {
			t.Errorf("PROPFIND %s = %d, want 404", target, rec.Code)
		}
	}
}

func TestDAVExportBoundsPropfindBody(t *testing.T) {
	env := newDAVTestEnv(t)
	body := `<?xml version="1.0"?><propfind xmlns="DAV:"><prop>` + strings.Repeat("<x/>", davPropfindMaxBody) + `</prop></propfind>`
	req := httptest.NewRequest("PROPFIND", "/dav/default_shelf/", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+env.token)
	req.Header.Set("Depth", "0")
	rec := httptest.NewRecorder()
	env.handler.ServeHTTP(rec, req)
	if rec.Code == http.StatusMultiStatus {
		t.Errorf("oversized PROPFIND body accepted")
	}
}

func TestRotateDAVExportTokenForConf(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	conf := `server_conf: {addr: "127.0.0.1:20000", read_timeout: 60s, write_timeout: 60s}
dav_export: {addr: "127.0.0.1:20001", read_timeout: 60s, write_timeout: 60s, token_file: ./dav.token}
app_conf: {store_path: ./store}
`
	writeTestFile(t, filepath.Join(dir, "config.yaml"), conf)
	token, tokenFile, err := RotateDAVExportTokenForConf("config.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(dir, "dav.token"); tokenFile != want {
		t.Errorf("token file = %q, want %q", tokenFile, want)
	}
	if got, _ := readDAVExportToken(tokenFile); got != token {
		t.Errorf("file holds %q, printed %q", got, token)
	}
}

func TestDAVTokenFileExposed(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("no permission bits")
	}
	tokenFile := filepath.Join(t.TempDir(), "dav.token")
	if _, err := RotateDAVExportToken(tokenFile); err != nil {
		t.Fatal(err)
	}
	if davTokenFileExposed(tokenFile) {
		t.Error("fresh token file reported exposed")
	}
	if err := os.Chmod(tokenFile, 0o644); err != nil {
		t.Fatal(err)
	}
	if !davTokenFileExposed(tokenFile) {
		t.Error("0644 token file not reported")
	}
}

// TestDAVExportSymlinkSwapRace swaps a path between a real entry and a symlink
// into app/ while it is read, the race a check-then-open walk loses.
func TestDAVExportSymlinkSwapRace(t *testing.T) {
	env := newDAVTestEnv(t)
	books := filepath.Join(env.libRoot, "books")
	secret := []byte("hidden runtime state")
	writeTestFile(t, filepath.Join(env.libRoot, "app", "secret.json"), string(secret))
	// A real file and a real directory, so the walk has something to pass.
	writeTestFile(t, filepath.Join(books, "probe"), "plain")
	writeTestFile(t, filepath.Join(books, "dd", "secret.json"), "plain")
	writeTestFile(t, filepath.Join(books, "dd", "library.lock"), "plain")
	for name, target := range map[string]string{"probe.link": "../app/secret.json", "dd.link": "../app"} {
		if err := os.Symlink(target, filepath.Join(books, name)); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}
	}

	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		// Each swap is a rename, so the path always exists as one or the other.
		swap := func(name string) {
			p := filepath.Join(books, name)
			_ = os.Rename(p, p+".real")
			_ = os.Rename(p+".link", p)
			_ = os.Rename(p, p+".link")
			_ = os.Rename(p+".real", p)
		}
		for {
			select {
			case <-stop:
				return
			default:
				swap("probe")
				swap("dd")
			}
		}
	}()
	defer func() { close(stop); <-done }()

	for i := range 2000 {
		for _, target := range []string{"books/probe", "books/dd/secret.json"} {
			rec := env.do(t, http.MethodGet, "/dav/default_shelf/"+target, env.token, nil)
			if rec.Code == http.StatusOK && bytes.Equal(rec.Body.Bytes(), secret) {
				t.Fatalf("request %d: GET %s served app/secret.json through a swapped-in symlink", i, target)
			}
		}
		rec := env.do(t, "PROPFIND", "/dav/default_shelf/books/dd/", env.token, map[string]string{"Depth": "1"})
		if rec.Code != http.StatusMultiStatus {
			continue
		}
		for _, href := range propfindHrefs(t, rec.Body.Bytes()) {
			if strings.HasSuffix(href, "/book-cache-abc.json") || strings.HasSuffix(href, "/tmp/") {
				t.Fatalf("request %d: PROPFIND books/dd/ listed app/ entry %s", i, href)
			}
		}
	}
}

// TestMainServiceReleasesAPIWhenExportFails: the API is already listening when
// the export starts, so an export that fails must not leave it bound.
func TestMainServiceReleasesAPIWhenExportFails(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	apiAddr := ln.Addr().String()
	ln.Close()

	dir := t.TempDir()
	conf := `logger: {log_file: {type: none}}
server_conf: {addr: "` + apiAddr + `", read_timeout: 60s, write_timeout: 60s}
dav_export: {addr: "not-an-address", read_timeout: 60s, write_timeout: 60s, token_file: ` + filepath.Join(dir, "dav.token") + `}
app_conf:
  logger: {log_file: {type: none}}
  store_path: ` + filepath.Join(dir, "store") + `
`
	confPath := filepath.Join(dir, "config.yaml")
	writeTestFile(t, confPath, conf)

	if err := MainService(confPath); err == nil {
		t.Fatal("MainService started with an unbindable dav_export.addr")
	}
	again, err := net.Listen("tcp", apiAddr)
	if err != nil {
		t.Fatalf("API address still bound after a failed start: %v", err)
	}
	again.Close()
}

// withDAVOrigins is env's handler with CORS for origins.
func withDAVOrigins(t *testing.T, env *davTestEnv, origins ...string) http.Handler {
	t.Helper()
	h, err := env.app.davExportCORSHandler(env.tokenFile, origins)
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func TestDAVExportCORS(t *testing.T) {
	env := newDAVTestEnv(t)
	h := withDAVOrigins(t, env, "http://Demo.example:8080")
	target := "/dav/default_shelf/"

	rec := davDo(t, h, "PROPFIND", target, env.token, map[string]string{"Origin": "http://demo.example:8080", "Depth": "1"})
	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("allowed PROPFIND = %d", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "http://demo.example:8080" {
		t.Errorf("allowed origin header = %q", got)
	}

	for _, origin := range []string{"http://evil.example", "https://demo.example:8080", "null", ""} {
		rec = davDo(t, h, "PROPFIND", target, env.token, map[string]string{"Origin": origin, "Depth": "1"})
		if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
			t.Errorf("origin %q got Allow-Origin %q", origin, got)
		}
	}

	// Unconfigured: no CORS header of any kind, preflight included.
	header := map[string]string{"Origin": "http://demo.example:8080", "Access-Control-Request-Method": "PROPFIND", "Depth": "1"}
	for _, method := range []string{"PROPFIND", http.MethodOptions} {
		rec = env.do(t, method, target, env.token, header)
		for k := range rec.Header() {
			if strings.HasPrefix(k, "Access-Control-") || k == "Vary" {
				t.Errorf("unconfigured %s sent %s", method, k)
			}
		}
	}
	if rec = env.do(t, http.MethodOptions, target, "", header); rec.Code != http.StatusUnauthorized {
		t.Errorf("unconfigured tokenless preflight = %d, want 401", rec.Code)
	}
}

func TestDAVExportCORSPreflight(t *testing.T) {
	env := newDAVTestEnv(t)
	h := withDAVOrigins(t, env, "http://demo.example")
	preflight := map[string]string{
		"Origin":                         "http://demo.example",
		"Access-Control-Request-Method":  "PROPFIND",
		"Access-Control-Request-Headers": "authorization, depth",
	}

	// No token: answered, with nothing from the shelf in it.
	for _, target := range []string{"/dav/default_shelf/", "/dav/default_shelf/books/a.bookpkg/book.json", "/dav/missing/", "/dav/default_shelf/app/scan-cache.json"} {
		rec := davDo(t, h, http.MethodOptions, target, "", preflight)
		if rec.Code != http.StatusNoContent || rec.Body.Len() != 0 {
			t.Errorf("preflight %s = %d %q, want empty 204", target, rec.Code, rec.Body.String())
		}
		hdr := rec.Header()
		if got := hdr.Get("Access-Control-Allow-Origin"); got != "http://demo.example" {
			t.Errorf("preflight %s Allow-Origin = %q", target, got)
		}
		if hdr.Get("DAV") != "" || hdr.Get("Allow") != "" {
			t.Errorf("preflight %s answered as the shelf's OPTIONS", target)
		}
		methods := hdr.Get("Access-Control-Allow-Methods")
		for _, m := range []string{"GET", "PROPFIND"} {
			if !strings.Contains(methods, m) {
				t.Errorf("Allow-Methods %q lacks %s", methods, m)
			}
		}
		for _, m := range []string{"PUT", "DELETE", "MKCOL", "MOVE", "PROPPATCH"} {
			if strings.Contains(methods, m) {
				t.Errorf("Allow-Methods %q offers %s", methods, m)
			}
		}
		allowHeaders := strings.ToLower(hdr.Get("Access-Control-Allow-Headers"))
		for _, name := range []string{"authorization", "depth"} {
			if !strings.Contains(allowHeaders, name) {
				t.Errorf("Allow-Headers %q lacks %s", allowHeaders, name)
			}
		}
		if hdr.Get("Access-Control-Allow-Credentials") != "" {
			t.Error("preflight allows credentials")
		}
	}

	// A disallowed origin's preflight is the plain OPTIONS it was before: 401.
	preflight["Origin"] = "http://evil.example"
	rec := davDo(t, h, http.MethodOptions, "/dav/default_shelf/", "", preflight)
	if rec.Code != http.StatusUnauthorized || rec.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Errorf("disallowed preflight = %d, Allow-Origin %q", rec.Code, rec.Header().Get("Access-Control-Allow-Origin"))
	}

	// An allowed origin still needs the token, and still cannot write.
	origin := map[string]string{"Origin": "http://demo.example", "Depth": "1"}
	if rec = davDo(t, h, "PROPFIND", "/dav/default_shelf/", "", origin); rec.Code != http.StatusUnauthorized {
		t.Errorf("tokenless PROPFIND = %d, want 401", rec.Code)
	}
	for _, method := range []string{http.MethodPut, http.MethodDelete, "MKCOL", "PROPPATCH"} {
		if rec = davDo(t, h, method, "/dav/default_shelf/books/a.bookpkg/book.json", env.token, origin); rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s from allowed origin = %d, want 405", method, rec.Code)
		}
	}
}
