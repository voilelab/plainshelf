package davfs

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/voilelab/plainshelf/internal/fsutil"
	"golang.org/x/net/webdav"
)

type fakeDAV struct {
	*httptest.Server
	dir string

	mu       sync.Mutex
	token    string
	requests []string
	auths    []string
}

func newFakeDAV(t *testing.T, prefix string) *fakeDAV {
	t.Helper()
	f := &fakeDAV{dir: t.TempDir(), token: "s3cret"}
	h := &webdav.Handler{Prefix: prefix, FileSystem: webdav.Dir(f.dir), LockSystem: webdav.NewMemLS()}
	f.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.requests = append(f.requests, r.Method+" "+r.URL.EscapedPath())
		f.auths = append(f.auths, r.Header.Get("Authorization"))
		want := "Bearer " + f.token
		f.mu.Unlock()
		if r.Header.Get("Authorization") != want {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		h.ServeHTTP(w, r)
	}))
	t.Cleanup(f.Close)
	return f
}

func (f *fakeDAV) write(t *testing.T, name, content string) {
	t.Helper()
	p := filepath.Join(f.dir, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func (f *fakeDAV) take() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := f.requests
	f.requests = nil
	return out
}

func newTestFS(t *testing.T, rawURL string, secret Secret) *FS {
	t.Helper()
	f, err := New(rawURL, Options{Secret: secret})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	f.retryDelay = time.Millisecond
	return f
}

func staticSecret(s string) Secret {
	return func(context.Context) (string, error) { return s, nil }
}

func TestTargetPreservesBaseEscapingAndEscapesEachFileSegment(t *testing.T) {
	for _, tc := range []struct {
		name    string
		base    string
		file    string
		dir     bool
		path    string
		escaped string
	}{
		{"root", "/dav/shelf", ".", true, "/dav/shelf/", "/dav/shelf/"},
		{"directory", "/dav/shelf", "books/F", true, "/dav/shelf/books/F/", "/dav/shelf/books/F/"},
		{"escaped base slash", "/dav%2fshelf", "book.json", false, "/dav/shelf/book.json", "/dav%2fshelf/book.json"},
		{"literal percent escape in name", "/dav/shelf", "books/%2F/book.json", false, "/dav/shelf/books/%2F/book.json", "/dav/shelf/books/%252F/book.json"},
		{"reserved characters", "/dav/shelf", "Dune #1/50%?.md", false, "/dav/shelf/Dune #1/50%?.md", "/dav/shelf/Dune%20%231/50%25%3F.md"},
		{"unicode", "/dav/shelf", "書/一.md", false, "/dav/shelf/書/一.md", "/dav/shelf/%E6%9B%B8/%E4%B8%80.md"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newTestFS(t, "https://example.com"+tc.base, nil)
			got := f.target(tc.file, tc.dir)
			if got.Path != tc.path || got.EscapedPath() != tc.escaped {
				t.Fatalf("target = path %q, escaped %q; want %q, %q", got.Path, got.EscapedPath(), tc.path, tc.escaped)
			}
			if got.String() != "https://example.com"+tc.escaped {
				t.Fatalf("URL = %q, want %q", got.String(), "https://example.com"+tc.escaped)
			}
		})
	}
}

func TestReadDirStatAndOpen(t *testing.T) {
	srv := newFakeDAV(t, "/dav/shelf")
	srv.write(t, "books/Dune #1/book.json", `{"title":"Dune"}`)
	srv.write(t, "books/50% off/book.json", `{}`)
	srv.write(t, "shelf.json", `{}`)

	f := newTestFS(t, srv.URL+"/dav/shelf", staticSecret("s3cret"))

	entries, err := f.ReadDir("books")
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
		if !e.IsDir() {
			t.Errorf("%q is not reported as a directory", e.Name())
		}
	}
	if want := []string{"50% off", "Dune #1"}; !slices.Equal(names, want) {
		t.Errorf("ReadDir(books) = %v, want %v", names, want)
	}

	info, err := f.Stat("books/Dune #1/book.json")
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if info.IsDir() || info.Size() != int64(len(`{"title":"Dune"}`)) || info.Name() != "book.json" || info.ModTime().IsZero() {
		t.Errorf("Stat = dir %v size %d name %q mtime %v", info.IsDir(), info.Size(), info.Name(), info.ModTime())
	}

	root, err := f.Stat(".")
	if err != nil || !root.IsDir() {
		t.Fatalf("Stat(.) = %v, %v; want a directory", root, err)
	}

	file, err := f.Open("books/Dune #1/book.json")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer file.Close()
	got, err := io.ReadAll(file)
	if err != nil || string(got) != `{"title":"Dune"}` {
		t.Errorf("read %q, %v", got, err)
	}

	for _, req := range srv.take() {
		method, _, _ := strings.Cut(req, " ")
		if method != "PROPFIND" && method != http.MethodGet {
			t.Errorf("sent %s; only PROPFIND and GET are reads", req)
		}
	}
}

func TestSeekUsesRange(t *testing.T) {
	srv := newFakeDAV(t, "")
	srv.write(t, "a.txt", "0123456789")
	f := newTestFS(t, srv.URL, staticSecret("s3cret"))

	file, err := f.Open("a.txt")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer file.Close()
	seeker := file.(io.ReadSeeker)
	if _, err := seeker.Seek(6, io.SeekStart); err != nil {
		t.Fatalf("Seek: %v", err)
	}
	got, err := io.ReadAll(seeker)
	if err != nil || string(got) != "6789" {
		t.Errorf("after seek read %q, %v; want 6789", got, err)
	}
	if end, err := seeker.Seek(-3, io.SeekEnd); err != nil || end != 7 {
		t.Fatalf("Seek end = %d, %v", end, err)
	}
	got, _ = io.ReadAll(seeker)
	if string(got) != "789" {
		t.Errorf("after seek from end read %q, want 789", got)
	}
}

func TestSeekOnServerIgnoringRange(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "0123456789")
	}))
	defer srv.Close()
	f := newTestFS(t, srv.URL, nil)

	file, err := f.Open("a.txt")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer file.Close()
	seeker := file.(io.ReadSeeker)
	if _, err := seeker.Seek(4, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	got, _ := io.ReadAll(seeker)
	if string(got) != "456789" {
		t.Errorf("read %q, want 456789", got)
	}
}

func TestNotFoundAndUnauthorized(t *testing.T) {
	srv := newFakeDAV(t, "")
	srv.write(t, "a.txt", "x")

	f := newTestFS(t, srv.URL, staticSecret("s3cret"))
	for _, call := range []func() error{
		func() error { _, err := f.Stat("missing"); return err },
		func() error { _, err := f.ReadDir("missing"); return err },
		func() error { _, err := f.Open("missing"); return err },
	} {
		err := call()
		if !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("error = %v, want fs.ErrNotExist", err)
		}
		if errors.Is(err, fsutil.ErrSourceUnavailable) {
			t.Errorf("a missing file reads as an unavailable source: %v", err)
		}
	}

	wrong := newTestFS(t, srv.URL, staticSecret("wrong"))
	_, err := wrong.Stat("a.txt")
	if !errors.Is(err, fsutil.ErrSourceUnauthorized) || !errors.Is(err, fsutil.ErrSourceUnavailable) {
		t.Errorf("wrong token error = %v, want ErrSourceUnauthorized and ErrSourceUnavailable", err)
	}
	if strings.Contains(err.Error(), "wrong") {
		t.Errorf("error %q quotes the credential", err)
	}

	srv.Close()
	_, err = f.Stat("a.txt")
	if !errors.Is(err, fsutil.ErrSourceUnavailable) || errors.Is(err, fsutil.ErrSourceUnauthorized) {
		t.Errorf("unreachable server error = %v, want ErrSourceUnavailable only", err)
	}
}

func TestRotatedCredentialIsResolvedAgain(t *testing.T) {
	srv := newFakeDAV(t, "")
	srv.write(t, "a.txt", "x")

	var current atomic.Value
	current.Store("s3cret")
	var asked atomic.Int32
	f := newTestFS(t, srv.URL, func(context.Context) (string, error) {
		asked.Add(1)
		return current.Load().(string), nil
	})
	if _, err := f.Stat("a.txt"); err != nil {
		t.Fatalf("Stat: %v", err)
	}

	srv.mu.Lock()
	srv.token = "rotated"
	srv.mu.Unlock()
	current.Store("rotated")
	if _, err := f.Stat("a.txt"); err != nil {
		t.Fatalf("Stat after rotation: %v", err)
	}
	if got := asked.Load(); got != 2 {
		t.Errorf("secret asked %d times, want once at start and once after the 401", got)
	}
}

func TestBasicAuthWithUsername(t *testing.T) {
	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		got = user + ":" + pass
		if !ok {
			got = r.Header.Get("Authorization")
		}
		_, _ = io.WriteString(w, "x")
	}))
	defer srv.Close()

	f, err := New(srv.URL, Options{Username: "reader", Secret: staticSecret("pw")})
	if err != nil {
		t.Fatal(err)
	}
	file, err := f.Open("a.txt")
	if err != nil {
		t.Fatal(err)
	}
	file.Close()
	if got != "reader:pw" {
		t.Errorf("server saw %q, want HTTP Basic reader:pw", got)
	}
}

func TestTransientFailuresAreRetried(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if calls.Add(1) < 3 {
			http.Error(w, "busy", http.StatusServiceUnavailable)
			return
		}
		_, _ = io.WriteString(w, "x")
	}))
	defer srv.Close()
	f := newTestFS(t, srv.URL, nil)

	file, err := f.Open("a.txt")
	if err != nil {
		t.Fatalf("Open after two 503s: %v", err)
	}
	file.Close()

	calls.Store(-10)
	if _, err := f.Open("a.txt"); !errors.Is(err, fsutil.ErrSourceUnavailable) {
		t.Errorf("Open after repeated 503s = %v, want ErrSourceUnavailable", err)
	}
	if got := calls.Load(); got != -7 {
		t.Errorf("attempts = %d, want %d", got+10, maxAttempts)
	}
}

func TestNonDAVReplyIsNotAnEmptyFolder(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "<html>captive portal</html>")
	}))
	defer srv.Close()
	f := newTestFS(t, srv.URL, nil)

	if _, err := f.ReadDir("."); !errors.Is(err, fsutil.ErrSourceUnavailable) {
		t.Errorf("ReadDir on a non-WebDAV reply = %v, want ErrSourceUnavailable", err)
	}
}

func TestIsReadOnly(t *testing.T) {
	f := newTestFS(t, "http://example.invalid/", nil)
	if _, err := fsutil.Writable(f); !errors.Is(err, fsutil.ErrReadOnly) {
		t.Errorf("Writable = %v, want ErrReadOnly", err)
	}
}

func TestParseURL(t *testing.T) {
	for raw, want := range map[string]string{
		"HTTP://NAS.example/dav/books": "http://nas.example/dav/books/",
		"https://nas.example":          "https://nas.example/",
	} {
		u, err := ParseURL(raw)
		if err != nil || u.String() != want {
			t.Errorf("ParseURL(%q) = %v, %v; want %s", raw, u, err, want)
		}
	}
	for _, raw := range []string{
		"ftp://nas.example/",
		"https://user:pw@nas.example/",
		"https://nas.example/?token=x",
		"https://nas.example/#x",
		"/relative",
	} {
		if _, err := ParseURL(raw); err == nil {
			t.Errorf("ParseURL(%q) accepted", raw)
		}
	}
}

func TestCollectionRedirectKeepsPropfind(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "books", "a"), 0o755); err != nil {
		t.Fatal(err)
	}
	h := &webdav.Handler{FileSystem: webdav.Dir(dir), LockSystem: webdav.NewMemLS()}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/books" {
			http.Redirect(w, r, "/books/", http.StatusMovedPermanently)
			return
		}
		h.ServeHTTP(w, r)
	}))
	defer srv.Close()
	f := newTestFS(t, srv.URL, nil)

	info, err := f.Stat("books")
	if err != nil || !info.IsDir() || info.Name() != "books" {
		t.Fatalf("Stat(books) through a redirect = %v, %v; want a directory", info, err)
	}
}

func TestRedirectToAnotherOriginIsRefused(t *testing.T) {
	var leaked atomic.Bool
	other := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			leaked.Store(true)
		}
	}))
	defer other.Close()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, other.URL+r.URL.Path, http.StatusTemporaryRedirect)
	}))
	defer srv.Close()
	f := newTestFS(t, srv.URL, staticSecret("s3cret"))

	if _, err := f.Open("a.txt"); err == nil {
		t.Error("Open followed a redirect to another origin")
	}
	if leaked.Load() {
		t.Error("the credential reached another origin")
	}
}

func TestChunkedGetStillReportsSize(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("0123456789"), 0o644); err != nil {
		t.Fatal(err)
	}
	h := &webdav.Handler{FileSystem: webdav.Dir(dir), LockSystem: webdav.NewMemLS()}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.Header.Get("Range") == "" {
			w.WriteHeader(http.StatusOK)
			w.(http.Flusher).Flush()
			_, _ = io.WriteString(w, "0123456789")
			return
		}
		h.ServeHTTP(w, r)
	}))
	defer srv.Close()
	f := newTestFS(t, srv.URL, nil)

	file, err := f.Open("a.txt")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || info.Size() != 10 {
		t.Fatalf("Stat on a chunked GET = %v, %v; want size 10", info, err)
	}
	if end, err := file.(io.Seeker).Seek(-2, io.SeekEnd); err != nil || end != 8 {
		t.Errorf("Seek end = %d, %v; want 8", end, err)
	}
}
