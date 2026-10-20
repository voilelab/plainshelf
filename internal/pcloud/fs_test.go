package pcloud

import (
	"archive/zip"
	"bytes"
	"errors"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/voilelab/plainshelf/internal/fsutil"
	"github.com/voilelab/plainshelf/internal/pcloud/pcloudtest"
)

const mount = "/Books/Shelf"

func seedTree(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		full := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func newTestFS(t *testing.T, srv *pcloudtest.Server, token string) *FS {
	t.Helper()
	client, err := NewClient("https://"+srv.Host()+"/", token, srv.Client())
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	client.retryDelay = time.Millisecond
	return NewFS(client, mount)
}

func count(requests []string, method string) int {
	n := 0
	for _, r := range requests {
		if r == method {
			n++
		}
	}
	return n
}

var sampleTree = map[string]string{
	"shelf.json":                          `{}`,
	"app/book-cache-a.json":               `{"x":1}`,
	"books/One.bookpkg/book.json":         `{"title":"One"}`,
	"books/F/Two.bookpkg/book.json":       `{"title":"Two"}`,
	"books/F/Two.bookpkg/sources/s1/a.md": "text",
}

func TestFSPassesFstest(t *testing.T) {
	srv := pcloudtest.New(t, seedTree(t, sampleTree), mount)
	f := newTestFS(t, srv, pcloudtest.Token)
	if err := fstest.TestFS(f, "shelf.json", "app/book-cache-a.json", "books/F/Two.bookpkg/sources/s1/a.md"); err != nil {
		t.Fatal(err)
	}
	if srv.TokenLeaked() {
		t.Error("the token reached a download or a query string")
	}
}

func TestFSWalkCostsOneListingForAnyTreeSize(t *testing.T) {
	srv := pcloudtest.New(t, seedTree(t, sampleTree), mount)
	f := newTestFS(t, srv, pcloudtest.Token)

	var files []string
	err := fs.WalkDir(f, ".", func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			files = append(files, p)
		}
		return err
	})
	if err != nil {
		t.Fatalf("WalkDir: %v", err)
	}
	if len(files) != len(sampleTree) {
		t.Errorf("walked %v, want %d files", files, len(sampleTree))
	}
	if got := count(srv.Take(), "listfolder"); got != 3 {
		t.Errorf("listfolder calls = %d, want 3", got)
	}
}

func TestFSReadsFileContent(t *testing.T) {
	srv := pcloudtest.New(t, seedTree(t, sampleTree), mount)
	f := newTestFS(t, srv, pcloudtest.Token)

	data, err := fs.ReadFile(f, "books/F/Two.bookpkg/sources/s1/a.md")
	if err != nil || string(data) != "text" {
		t.Fatalf("ReadFile = %q, %v", data, err)
	}
	info, err := f.Stat("books/F/Two.bookpkg/sources/s1/a.md")
	if err != nil || info.Size() != 4 || info.IsDir() || info.ModTime().IsZero() {
		t.Errorf("Stat = %+v, %v", info, err)
	}
	requests := srv.Take()
	if count(requests, "getfilelink") != 1 || count(requests, "download") != 1 {
		t.Errorf("requests = %v, want one getfilelink and one download", requests)
	}
}

func TestFSFallsBackWhenRecursiveListingIsRefused(t *testing.T) {
	srv := pcloudtest.New(t, seedTree(t, sampleTree), mount)
	srv.RefuseRecursive()
	f := newTestFS(t, srv, pcloudtest.Token)

	entries, err := f.ReadDir("books/F/Two.bookpkg/sources/s1")
	if err != nil || len(entries) != 1 || entries[0].Name() != "a.md" {
		t.Fatalf("ReadDir = %v, %v", entries, err)
	}
	requests := srv.Take()
	if count(requests, "listfolder") < 5 {
		t.Errorf("requests = %v, want the refused recursive call followed by a walk", requests)
	}
}

func TestFSMissingNameIsNotExist(t *testing.T) {
	srv := pcloudtest.New(t, seedTree(t, sampleTree), mount)
	f := newTestFS(t, srv, pcloudtest.Token)

	for _, name := range []string{"books/Nope.bookpkg", "shelf.json/x"} {
		if _, err := f.Stat(name); !errors.Is(err, fs.ErrNotExist) || errors.Is(err, fsutil.ErrSourceUnavailable) {
			t.Errorf("Stat(%q) = %v, want only fs.ErrNotExist", name, err)
		}
	}
}

func TestFSMissingMountNamesTheFoldersThere(t *testing.T) {
	srv := pcloudtest.New(t, seedTree(t, sampleTree), mount)
	client, err := NewClient(srv.Host(), pcloudtest.Token, srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	_, err = NewFS(client, "/Books/Shelff").Stat(".")
	if !errors.Is(err, fs.ErrNotExist) || !strings.Contains(err.Error(), `in "/Books". It contains "Shelf".`) {
		t.Errorf("err = %v, want a not-exist naming the folders in /Books", err)
	}
}

func TestFSRelistsAStaleFolder(t *testing.T) {
	dir := seedTree(t, sampleTree)
	srv := pcloudtest.New(t, dir, mount)
	f := newTestFS(t, srv, pcloudtest.Token)
	now := time.Now()
	f.now = func() time.Time { return now }

	if _, err := f.ReadDir("app"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "app", "book-cache-b.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	srv.Take()

	entries, _ := f.ReadDir("app")
	if len(entries) != 1 || len(srv.Take()) != 0 {
		t.Fatalf("fresh ReadDir = %v, want the cached listing with no request", entries)
	}

	now = now.Add(f.ListingMaxAge + time.Second)
	entries, err := f.ReadDir("app")
	if err != nil || len(entries) != 2 {
		t.Fatalf("stale ReadDir = %v, %v, want the new file", entries, err)
	}
	if got := srv.Take(); !slices.Equal(got, []string{"listfolder"}) {
		t.Errorf("stale ReadDir requests = %v, want one non-recursive listing", got)
	}
	if _, err := f.Stat("books/F/Two.bookpkg/book.json"); err != nil || len(srv.Take()) != 0 {
		t.Errorf("Stat after relisting = %v, want it answered from the tree", err)
	}
}

func TestFSRetriesTransientFailures(t *testing.T) {
	srv := pcloudtest.New(t, seedTree(t, sampleTree), mount)
	f := newTestFS(t, srv, pcloudtest.Token)

	srv.FailNext(http.StatusServiceUnavailable, http.StatusTooManyRequests)
	if _, err := f.Stat("shelf.json"); err != nil {
		t.Fatalf("Stat after two transient failures: %v", err)
	}

	srv.FailNext(http.StatusBadGateway, http.StatusBadGateway, http.StatusBadGateway)
	_, err := f.Open("shelf.json")
	if !errors.Is(err, fsutil.ErrSourceUnavailable) {
		t.Errorf("Open after three failures = %v, want ErrSourceUnavailable", err)
	}

	srv.Take()
	srv.FailNext(http.StatusForbidden)
	if _, err := f.Open("shelf.json"); !errors.Is(err, fsutil.ErrSourceUnavailable) {
		t.Errorf("Open on 403 = %v, want ErrSourceUnavailable", err)
	}
	if got := srv.Take(); len(got) != 1 {
		t.Errorf("a 403 was sent %d times, want no retry", len(got))
	}
}

func TestFSRefusedTokenIsUnavailableAndNotRetried(t *testing.T) {
	srv := pcloudtest.New(t, seedTree(t, sampleTree), mount)
	f := newTestFS(t, srv, "wrong-token")

	_, err := f.Stat("shelf.json")
	if !errors.Is(err, fsutil.ErrSourceUnavailable) || !strings.Contains(err.Error(), "2094") {
		t.Errorf("err = %v, want ErrSourceUnavailable carrying result 2094", err)
	}
	if strings.Contains(err.Error(), "wrong-token") {
		t.Errorf("error %q quotes the token", err)
	}
	if got := srv.Take(); len(got) != 1 {
		t.Errorf("requests = %v, want one", got)
	}
}

func TestFSOfflineIsSourceUnavailable(t *testing.T) {
	srv := pcloudtest.New(t, seedTree(t, sampleTree), mount)
	f := newTestFS(t, srv, pcloudtest.Token)
	if _, err := f.Stat("shelf.json"); err != nil {
		t.Fatal(err)
	}

	srv.SetOffline(true)
	_, err := f.Open("shelf.json")
	if !errors.Is(err, fsutil.ErrSourceUnavailable) || errors.Is(err, fs.ErrNotExist) {
		t.Errorf("Open offline = %v, want ErrSourceUnavailable", err)
	}
	if strings.Contains(err.Error(), "/dl/") {
		t.Errorf("error %q quotes the download link", err)
	}
}

func TestFSOpenZipCostsTwoRequests(t *testing.T) {
	srv := pcloudtest.New(t, seedTree(t, sampleTree), mount)
	f := newTestFS(t, srv, pcloudtest.Token)
	if _, err := f.Stat("."); err != nil {
		t.Fatal(err)
	}
	srv.Take()

	rc, err := f.OpenZip("books/One.bookpkg/book.json", "books/F/Two.bookpkg/sources/s1/a.md")
	if err != nil {
		t.Fatalf("OpenZip: %v", err)
	}
	data, err := io.ReadAll(rc)
	rc.Close()
	if err != nil {
		t.Fatal(err)
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, entry := range zr.File {
		names = append(names, entry.Name)
	}
	if !slices.Equal(names, []string{"book.json", "a.md"}) {
		t.Errorf("zip holds %v", names)
	}
	if got := srv.Take(); !slices.Equal(got, []string{"getziplink", "download"}) {
		t.Errorf("requests = %v", got)
	}
}

func TestNewClientAndHosts(t *testing.T) {
	for _, host := range []string{" ", "example.com", "api.pcloud.com.example.com"} {
		if _, err := NewClient(host, "t", nil); err == nil {
			t.Errorf("host %q accepted", host)
		}
	}
	if _, err := NewClient("https://eapi.pcloud.com/", "t", nil); err != nil {
		t.Errorf("EU host with scheme refused: %v", err)
	}
	if _, err := NewClient(HostEU, " ", nil); err == nil {
		t.Error("empty token accepted")
	}
	for id, want := range map[int]string{1: HostUS, 2: HostEU} {
		if got, err := HostForLocation(id); err != nil || got != want {
			t.Errorf("HostForLocation(%d) = %q, %v", id, got, err)
		}
	}
	if _, err := HostForLocation(3); err == nil {
		t.Error("HostForLocation(3) succeeded")
	}
}

var _ fs.ReadDirFS = (*FS)(nil)
