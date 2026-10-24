package shelves_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/voilelab/plainshelf/server"
	"github.com/voilelab/plainshelf/server/contract/apitest"
	"github.com/voilelab/plainshelf/shelf"
)

const remoteShelfID = "remote"

type davExporter struct {
	env       *apitest.Env
	srv       *httptest.Server
	tokenFile string

	mu       sync.Mutex
	requests []string
}

func newDAVExporter(t *testing.T) *davExporter {
	t.Helper()
	d := &davExporter{env: apitest.New(t), tokenFile: filepath.Join(t.TempDir(), "dav.token")}
	if err := server.EnsureDAVExportToken(d.tokenFile); err != nil {
		t.Fatalf("EnsureDAVExportToken: %v", err)
	}
	export := d.env.App.DAVExportHandler(d.tokenFile)
	d.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		d.mu.Lock()
		d.requests = append(d.requests, r.Method+" "+r.URL.Path)
		d.mu.Unlock()
		export.ServeHTTP(w, r)
	}))
	t.Cleanup(d.srv.Close)
	return d
}

func (d *davExporter) take() []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	out := d.requests
	d.requests = nil
	return out
}

func (d *davExporter) seed(t *testing.T, titles ...string) {
	t.Helper()
	for i, title := range titles {
		book := apitest.ImportTextBook(t, d.env, title, "", fmt.Sprintf("b%d.txt", i), title+" text")
		if i == 0 {
			rec := d.env.PutContent(apitest.BookURL(book.Meta.ID, "cover"), "image/png", strings.NewReader("png bytes"))
			apitest.AssertStatus(t, rec, http.StatusNoContent)
		}
	}
	apitest.AssertStatus(t, d.env.Post(apitest.BookCacheExportURL(), nil), http.StatusOK)
}

func (d *davExporter) source(credential string) *shelf.SourceConf {
	return &shelf.SourceConf{
		Type:       shelf.SourcePlainShelf,
		URL:        d.srv.URL + "/dav/" + apitest.DefaultShelfID + "/",
		Credential: credential,
	}
}

func withRemoteShelf(src *shelf.SourceConf) apitest.Option {
	return func(conf *server.AppConf) {
		conf.Shelves = append(conf.Shelves, &shelf.ShelfConfWithID{
			ID:     remoteShelfID,
			Source: src, ScanInterval: "1h",
		})
	}
}

func listBooks(t *testing.T, env *apitest.Env, shelfID string) map[string]string {
	t.Helper()
	titles := map[string]string{}
	for _, book := range apitest.GetJSON[[]server.Book](t, env, apitest.ShelfIDURL(shelfID, "books")) {
		titles[book.Meta.ID] = book.Meta.Title
	}
	return titles
}

func TestAPIPlainShelfSourceReadsTheExportContract(t *testing.T) {
	exporter := newDAVExporter(t)
	exporter.seed(t, "Dune", "Solaris")

	reader := apitest.New(t, withRemoteShelf(exporter.source("file:"+exporter.tokenFile)))

	local := listBooks(t, exporter.env, apitest.DefaultShelfID)
	remote := listBooks(t, reader, remoteShelfID)
	if len(local) != 2 || fmt.Sprint(local) != fmt.Sprint(remote) {
		t.Fatalf("remote books = %v, want the exporter's %v", remote, local)
	}

	for id := range local {
		for _, elem := range []string{"content", "cover"} {
			want := exporter.env.Get(apitest.ShelfIDURL(apitest.DefaultShelfID, "books", id, elem))
			got := reader.Get(apitest.ShelfIDURL(remoteShelfID, "books", id, elem))
			if got.Code != want.Code || got.Body.String() != want.Body.String() ||
				got.Header().Get("Content-Type") != want.Header().Get("Content-Type") {
				t.Errorf("%s of %s: remote %d %q %q, local %d %q %q", elem, local[id],
					got.Code, got.Header().Get("Content-Type"), got.Body.String(),
					want.Code, want.Header().Get("Content-Type"), want.Body.String())
			}
		}

		rec := reader.Patch(apitest.ShelfIDURL(remoteShelfID, "books", id), strings.NewReader(`{"title":"Renamed"}`))
		if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "SHELF_READ_ONLY") {
			t.Errorf("PATCH on the remote book = %d %s, want 409 SHELF_READ_ONLY", rec.Code, rec.Body.String())
		}
	}

	for _, req := range exporter.take() {
		method, _, _ := strings.Cut(req, " ")
		if method != "PROPFIND" && method != http.MethodGet {
			t.Errorf("the reader sent %s; only PROPFIND and GET are reads", req)
		}
	}
}

func TestAPIPlainShelfSourceRequestCount(t *testing.T) {
	counts := map[int][]string{}
	for _, n := range []int{2, 12} {
		exporter := newDAVExporter(t)
		titles := make([]string, n)
		for i := range titles {
			titles[i] = fmt.Sprintf("Book %02d", i)
		}
		exporter.seed(t, titles...)
		exporter.take()

		reader := apitest.New(t, withRemoteShelf(exporter.source("file:"+exporter.tokenFile)))
		if got := listBooks(t, reader, remoteShelfID); len(got) != n {
			t.Fatalf("n=%d: listed %d books", n, len(got))
		}
		counts[n] = exporter.take()
	}
	if len(counts[2]) != len(counts[12]) {
		t.Errorf("requests by book count: 2 → %v\n12 → %v", counts[2], counts[12])
	}
}

func TestAPIPlainShelfSourceErrorsContract(t *testing.T) {
	exporter := newDAVExporter(t)
	exporter.seed(t, "Dune")

	wrongToken := filepath.Join(t.TempDir(), "wrong.token")
	if err := os.WriteFile(wrongToken, []byte("not-the-token\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	refused := newFailingRemote(t, exporter.source("file:"+wrongToken))
	rec := refused.Get(apitest.ShelfIDURL(remoteShelfID, "books"))
	apitest.AssertErrorEnvelope(t, rec, http.StatusServiceUnavailable, "SOURCE_UNAUTHORIZED", "the shelf source refused the credential")

	down := httptest.NewServer(http.NotFoundHandler())
	src := &shelf.SourceConf{Type: shelf.SourceWebDAV, URL: down.URL + "/dav/"}
	down.Close()
	offline := newFailingRemote(t, src)
	rec = offline.Get(apitest.ShelfIDURL(remoteShelfID, "books"))
	apitest.AssertErrorEnvelope(t, rec, http.StatusServiceUnavailable, "SOURCE_UNAVAILABLE", "the shelf source is unavailable")

	unshared := exporter.source("file:" + exporter.tokenFile)
	unshared.URL = exporter.srv.URL + "/dav/not-shared/"
	missing := newFailingRemote(t, unshared)
	rec = missing.Get(apitest.ShelfIDURL(remoteShelfID, "books"))
	apitest.AssertErrorEnvelope(t, rec, http.StatusServiceUnavailable, "NOT_A_SHELF", "the shelf source is not a shelf: it has no books/ directory")
}

func newFailingRemote(t *testing.T, src *shelf.SourceConf) *apitest.Env {
	t.Helper()
	env := apitest.NewUnstarted(t, withRemoteShelf(src))
	data, ok := env.App.ShelfManager().GetShelf(remoteShelfID)
	if !ok {
		t.Fatal("remote shelf missing")
	}
	if err := data.WaitReady(t.Context()); err == nil {
		t.Fatal("remote shelf opened although its source fails")
	}
	return env
}

func TestAPISourceValidationAtStartup(t *testing.T) {
	t.Setenv("PLAINSHELF_TEST_UNSET", "")
	for name, src := range map[string]*shelf.SourceConf{
		"pcloud":            {Type: shelf.SourcePCloud, Host: "eapi.pcloud.com", Path: "/PlainShelf", Credential: "env:PLAINSHELF_TEST_UNSET"},
		"unresolvable":      {Type: shelf.SourceWebDAV, URL: "http://127.0.0.1:1/", Username: "u", Credential: "env:PLAINSHELF_TEST_UNSET"},
		"host scheme":       {Type: shelf.SourceWebDAV, URL: "http://127.0.0.1:1/", Username: "u", Credential: "host:x"},
		"bare token":        {Type: shelf.SourcePlainShelf, URL: "http://127.0.0.1:1/", Credential: "abc"},
		"foreign field":     {Type: shelf.SourceWebDAV, URL: "http://127.0.0.1:1/", Host: "x"},
		"credential in url": {Type: shelf.SourceWebDAV, URL: "http://u:pw@127.0.0.1:1/"},
	} {
		conf := apitest.AppConf(t, withRemoteShelf(src))
		app, err := server.NewApp(conf)
		if err == nil {
			app.Close()
			t.Errorf("%s: NewApp accepted %+v", name, src)
			continue
		}
		if !strings.Contains(err.Error(), remoteShelfID) {
			t.Errorf("%s: error %q does not name the shelf", name, err)
		}
	}

	conf := apitest.AppConf(t)
	conf.Shelves[0].Source = &shelf.SourceConf{Type: shelf.SourceLocal, Path: t.TempDir()}
	if app, err := server.NewApp(conf); err == nil {
		app.Close()
		t.Error("NewApp accepted lib_root and source together")
	}
}

func TestAPILocalSourceOpensLikeLibRootContract(t *testing.T) {
	libRoot := t.TempDir()
	env := apitest.New(t, func(conf *server.AppConf) {
		conf.Shelves[0].LibRoot = ""
		conf.Shelves[0].Source = &shelf.SourceConf{Type: shelf.SourceLocal, Path: libRoot}
	})
	book := apitest.ImportTextBook(t, env, "Dune", "", "dune.txt", "text")
	if ids := apitest.ListedBookIDs(t, env); !slices.Equal(ids, []string{book.Meta.ID}) {
		t.Errorf("listed %v, want [%s]", ids, book.Meta.ID)
	}
	if _, err := os.Stat(filepath.Join(libRoot, "books")); err != nil {
		t.Errorf("the book was not written under the source path: %v", err)
	}
}

func TestCheckSourceContract(t *testing.T) {
	exporter := newDAVExporter(t)
	exporter.seed(t, "Dune", "Solaris")
	token, err := os.ReadFile(exporter.tokenFile)
	if err != nil {
		t.Fatal(err)
	}
	url := exporter.source("").URL

	closed := httptest.NewServer(http.NotFoundHandler())
	closedURL := closed.URL + "/dav/x/"
	closed.Close()

	cases := []struct {
		name, url, secret string
		want              server.SourceCheck
	}{
		{"shelf", url, strings.TrimSpace(string(token)), server.SourceCheck{Books: 2}},
		{"wrong token", url, "wrong", server.SourceCheck{Code: server.SourceCheckUnauthorized}},
		{"unreachable", closedURL, "token", server.SourceCheck{Code: server.SourceCheckUnavailable}},
		{"not a shelf", exporter.srv.URL + "/dav/" + apitest.DefaultShelfID + "/books/", strings.TrimSpace(string(token)),
			server.SourceCheck{Code: server.SourceCheckNotAShelf}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := server.CheckSource(shelf.SourcePlainShelf, tc.url, "", tc.secret)
			if err != nil {
				t.Fatalf("CheckSource: %v", err)
			}
			if got != tc.want {
				t.Fatalf("CheckSource = %+v, want %+v", got, tc.want)
			}
		})
	}

	for _, typ := range []string{shelf.SourceLocal, shelf.SourcePCloud, "ftp"} {
		if _, err := server.CheckSource(typ, url, "", "token"); err == nil {
			t.Errorf("CheckSource(%q) succeeded, want an error", typ)
		}
	}

	if _, err := server.CheckSource(shelf.SourceWebDAV, strings.Replace(url, "://", "://user:pw@", 1), "", ""); err == nil {
		t.Error("CheckSource accepted a URL carrying credentials")
	}
}
