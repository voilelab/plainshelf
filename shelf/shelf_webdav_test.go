package shelf

import (
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/voilelab/plainshelf/internal/davfs"
	"golang.org/x/net/webdav"
)

// davRecorder is a general WebDAV server over a shelf directory that records
// every request it is sent.
type davRecorder struct {
	mu       sync.Mutex
	requests []string
}

func (d *davRecorder) take() []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	out := d.requests
	d.requests = nil
	return out
}

// openWebDAVShelf opens libRoot through davfs.FS, served by x/net/webdav.
func openWebDAVShelf(t *testing.T, libRoot string) *Shelf {
	t.Helper()
	s, _ := openWebDAVShelfRecorded(t, libRoot)
	return s
}

func openWebDAVShelfRecorded(t *testing.T, libRoot string) (*Shelf, *davRecorder) {
	t.Helper()

	rec := &davRecorder{}
	h := &webdav.Handler{Prefix: "/dav", FileSystem: webdav.Dir(libRoot), LockSystem: webdav.NewMemLS()}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec.mu.Lock()
		rec.requests = append(rec.requests, r.Method+" "+r.URL.Path)
		rec.mu.Unlock()
		h.ServeHTTP(w, r)
	}))
	t.Cleanup(srv.Close)

	root, err := davfs.New(srv.URL+"/dav/", davfs.Options{})
	if err != nil {
		t.Fatalf("davfs.New: %v", err)
	}
	s, err := NewShelfFromFS(&ShelfConf{}, root, "")
	if err != nil {
		t.Fatalf("NewShelfFromFS: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	if err := s.WaitReady(t.Context()); err != nil {
		t.Fatalf("WaitReady: %v", err)
	}
	return s, rec
}

func TestWebDAVShelfRequestsDoNotGrowWithBooks(t *testing.T) {
	counts := map[int]int{}
	for _, n := range []int{2, 12} {
		titles := make([]string, n)
		for i := range titles {
			titles[i] = "Book " + string(rune('A'+i))
		}
		s, rec := openWebDAVShelfRecorded(t, seedExportedShelf(t, titles...))

		if got := listedTitles(t, s); !slices.Equal(got, titles) {
			t.Errorf("n=%d: listed %v, want %v", n, got, titles)
		}
		if _, err := s.GetAllFolders(); err != nil {
			t.Fatalf("GetAllFolders: %v", err)
		}
		requests := rec.take()
		for _, req := range requests {
			method, _, _ := strings.Cut(req, " ")
			if method != "PROPFIND" && method != http.MethodGet {
				t.Errorf("n=%d: sent %s; only PROPFIND and GET are reads", n, req)
			}
			if strings.HasPrefix(strings.TrimPrefix(req, method+" "), "/dav/books/") {
				t.Errorf("n=%d: opening and listing reached into books/: %s", n, req)
			}
		}
		counts[n] = len(requests)
	}
	if counts[2] != counts[12] {
		t.Errorf("requests to open and list = %v by book count, want the same for every size", counts)
	}
}

func TestResolveSource(t *testing.T) {
	valid := []ShelfConf{
		{LibRoot: "/srv/books"},
		{Source: &SourceConf{Type: SourceLocal, Path: "/srv/books"}},
		{Source: &SourceConf{Type: SourcePlainShelf, URL: "http://192.168.1.20:20001/dav/default_shelf/", Credential: "file:/etc/token"}},
		{Source: &SourceConf{Type: SourceWebDAV, URL: "https://nas.example/dav/books/"}},
		{Source: &SourceConf{Type: SourceWebDAV, URL: "https://nas.example/dav/books/", Username: "reader", Credential: "env:PW"}},
		{Source: &SourceConf{Type: SourcePCloud, Host: "eapi.pcloud.com", Path: "/PlainShelf", Credential: "host:cloud"}},
	}
	for _, conf := range valid {
		if _, err := conf.ResolveSource(); err != nil {
			t.Errorf("ResolveSource(%+v / %+v) = %v", conf, conf.Source, err)
		}
	}

	invalid := map[string]ShelfConf{
		"both":             {LibRoot: "/srv", Source: &SourceConf{Type: SourceLocal, Path: "/srv"}},
		"neither":          {},
		"unknown type":     {Source: &SourceConf{Type: "ftp", URL: "https://x/"}},
		"foreign field":    {Source: &SourceConf{Type: SourceLocal, Path: "/srv", URL: "https://x/"}},
		"missing url":      {Source: &SourceConf{Type: SourceWebDAV}},
		"bare token":       {Source: &SourceConf{Type: SourcePlainShelf, URL: "http://x/dav/s/", Credential: "abc123"}},
		"empty reference":  {Source: &SourceConf{Type: SourcePlainShelf, URL: "http://x/dav/s/", Credential: "env:"}},
		"plainshelf no cr": {Source: &SourceConf{Type: SourcePlainShelf, URL: "http://x/dav/s/"}},
		"user no secret":   {Source: &SourceConf{Type: SourceWebDAV, URL: "http://x/", Username: "u"}},
		"secret in url":    {Source: &SourceConf{Type: SourceWebDAV, URL: "http://u:pw@x/"}},
		"token in query":   {Source: &SourceConf{Type: SourceWebDAV, URL: "http://x/?token=t"}},
	}
	for name, conf := range invalid {
		if _, err := conf.ResolveSource(); err == nil {
			t.Errorf("%s: ResolveSource accepted %+v", name, conf.Source)
		}
	}
}

func TestSourceScope(t *testing.T) {
	for _, tc := range []struct {
		src  SourceConf
		want string
	}{
		{SourceConf{Type: SourceLocal, Path: "/srv"}, ""},
		{SourceConf{Type: SourceWebDAV, URL: "HTTPS://NAS.example/dav/books"}, "https://nas.example/dav/books/"},
		{SourceConf{Type: SourceWebDAV, URL: "https://nas.example/dav/books/", Username: "reader", Credential: "env:X"}, "https://reader@nas.example/dav/books/"},
		{SourceConf{Type: SourcePlainShelf, URL: "http://10.0.0.2:20001/dav/s", Credential: "env:X"}, "http://10.0.0.2:20001/dav/s/"},
		{SourceConf{Type: SourcePCloud, Host: "eapi.pcloud.com", UserID: "42", Path: "/PlainShelf"}, "pcloud://eapi.pcloud.com/42/PlainShelf"},
	} {
		if got := tc.src.Scope(); got != tc.want {
			t.Errorf("Scope(%+v) = %q, want %q", tc.src, got, tc.want)
		}
	}
}

func TestNewShelfOpensLocalSource(t *testing.T) {
	libRoot := seedExportedShelf(t, "Dune")
	s := newTestShelf(t, &ShelfConf{Source: &SourceConf{Type: SourceLocal, Path: libRoot}, LockMode: "none"})
	if got := listedTitles(t, s); !slices.Equal(got, []string{"Dune"}) {
		t.Errorf("listed %v, want [Dune]", got)
	}

	if _, err := NewShelf(&ShelfConf{Source: &SourceConf{Type: SourceWebDAV, URL: "https://nas.example/"}}); err == nil {
		t.Error("NewShelf opened a virtual source it has no ReadFS for")
	}
}
