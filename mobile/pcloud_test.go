package mobile

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json/v2"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/voilelab/plainshelf/internal/pcloud/pcloudtest"
	"github.com/voilelab/plainshelf/internal/taskutil"
	"github.com/voilelab/plainshelf/shelf"
)

const pcloudFolder = "/PlainShelf/Shelf"

func servePCloudShelf(t *testing.T) (*pcloudtest.Server, string) {
	t.Helper()
	libRoot := t.TempDir()
	s, err := shelf.NewShelf(&shelf.ShelfConf{LibRoot: libRoot, LockMode: "none", BookCacheWriterID: "source"})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.WaitReady(t.Context()); err != nil {
		t.Fatal(err)
	}
	book, err := s.NewBook(shelf.FolderPath{}, "Dune")
	if err != nil {
		t.Fatal(err)
	}
	source, err := book.NewSource(strings.NewReader("Dune text"))
	if err != nil {
		t.Fatal(err)
	}
	if err := source.WriteAsset("dune.png", []byte("Dune illustration")); err != nil {
		t.Fatal(err)
	}
	adult, err := s.NewBook(shelf.FolderPath{}, "Adult")
	if err != nil {
		t.Fatal(err)
	}
	meta := adult.GetMeta()
	meta.NSFW = true
	if err := adult.SetMeta(meta); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ExportBookCache(); err != nil {
		t.Fatal(err)
	}

	srv := pcloudtest.New(t, libRoot, pcloudFolder)
	pcloudHTTPClient = srv.Client()
	t.Cleanup(func() { pcloudHTTPClient = nil })
	return srv, book.ID()
}

func TestPCloudBundleUsesAppTempDirectory(t *testing.T) {
	srv, bookID := servePCloudShelf(t)
	dataDir := t.TempDir()

	blocked := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(blocked, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TMPDIR", blocked)
	startT(t, dataDir)
	if err := OpenPCloudShelf("pcloud-a", "Cloud", srv.Host(), pcloudtest.Token, pcloudFolder, "pcloud:scope-a", false); err != nil {
		t.Fatal(err)
	}
	waitReady(t)
	rt, err := running()
	if err != nil {
		t.Fatal(err)
	}
	s, ok := rt.app.ShelfManager().GetShelf("pcloud-a")
	if !ok {
		t.Fatal("pCloud shelf missing")
	}
	book, err := s.GetBook(bookID)
	if err != nil {
		t.Fatal(err)
	}
	source, err := book.ResolveCurrentSource()
	if err != nil {
		t.Fatal(err)
	}
	srv.Take()
	var bundle bytes.Buffer
	zw := zip.NewWriter(&bundle)
	if err := source.WriteAssetsToZip(zw, []string{"dune.png"}); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	zr, err := zip.NewReader(bytes.NewReader(bundle.Bytes()), int64(bundle.Len()))
	if err != nil {
		t.Fatalf("invalid bundle: %v", err)
	}
	if len(zr.File) != 1 || zr.File[0].Name != "dune.png" {
		t.Fatalf("unexpected bundle entries: %v", zr.File)
	}
	file, err := zr.File[0].Open()
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	data, err := io.ReadAll(file)
	if err != nil || string(data) != "Dune illustration" {
		t.Fatalf("illustration = %q, %v", data, err)
	}
	if got := srv.Take(); !slices.Equal(got, []string{"getziplink", "download"}) {
		t.Fatalf("bundle requests = %v", got)
	}
	if want := filepath.Join(dataDir, "tmp"); os.TempDir() != want {
		t.Fatalf("temp directory = %q, want %q", os.TempDir(), want)
	}
	if entries, err := os.ReadDir(os.TempDir()); err != nil || len(entries) != 0 {
		t.Fatalf("temporary files left after bundle: %v, %v", entries, err)
	}
}

func TestPCloudShelfIsServedAndKeptOffline(t *testing.T) {
	srv, bookID := servePCloudShelf(t)
	startT(t, t.TempDir())

	if err := OpenPCloudShelf("pcloud-a", "Cloud", srv.Host(), pcloudtest.Token, pcloudFolder, "pcloud:scope-a", false); err != nil {
		t.Fatalf("OpenPCloudShelf: %v", err)
	}
	waitReady(t)
	if titles := bookTitles(t, "pcloud-a"); !strings.Contains(titles, `"Dune"`) {
		t.Fatalf("books = %s", titles)
	}
	if res := handle(t, http.MethodPut, "/api/shelves/pcloud-a/books/"+bookID+"/download", ""); res.Status != http.StatusOK {
		t.Fatalf("download: %d %s", res.Status, res.Body)
	}

	srv.SetOffline(true)
	res := handle(t, http.MethodGet, "/api/shelves/pcloud-a/books/"+bookID+"/content", "")
	if res.Status != http.StatusOK || !strings.Contains(string(res.Body), "Dune text") {
		t.Errorf("offline content: %d %s", res.Status, res.Body)
	}
	if srv.TokenLeaked() {
		t.Error("the token reached a download or a query string")
	}

	if entries := listShelves(t); len(entries) != 0 {
		t.Errorf("ListShelves = %v, want no local shelves", entries)
	}
}

func TestPCloudShelfReopensAndCloses(t *testing.T) {
	srv, _ := servePCloudShelf(t)
	startT(t, t.TempDir())

	for range 2 {
		if err := OpenPCloudShelf("pcloud-a", "Cloud", srv.Host(), pcloudtest.Token, pcloudFolder, "pcloud:scope-a", false); err != nil {
			t.Fatalf("OpenPCloudShelf: %v", err)
		}
	}
	if err := CloseVirtualShelf("pcloud-a"); err != nil {
		t.Fatalf("CloseVirtualShelf: %v", err)
	}
	if res := handle(t, http.MethodGet, "/api/shelves/pcloud-a/books", ""); res.Status != http.StatusNotFound {
		t.Errorf("closed shelf answered %d", res.Status)
	}
	if err := CloseVirtualShelf("pcloud-a"); err == nil {
		t.Error("closing twice succeeded")
	}

	local := createShelf(t, "Local")
	if err := OpenPCloudShelf(local, "Cloud", srv.Host(), pcloudtest.Token, pcloudFolder, "pcloud:scope-a", false); err == nil {
		t.Error("a pCloud shelf replaced a local one")
	}
	if err := OpenPCloudShelf("pcloud-b", "Cloud", srv.Host(), "", pcloudFolder, "pcloud:scope-b", false); err == nil {
		t.Error("an empty token was accepted")
	}
}

func TestPCloudFailedReplaceKeepsOld(t *testing.T) {
	srv, _ := servePCloudShelf(t)
	dataDir := t.TempDir()
	startT(t, dataDir)
	if err := OpenPCloudShelf("pcloud-a", "Cloud", srv.Host(), pcloudtest.Token, pcloudFolder, "pcloud:scope-a", false); err != nil {
		t.Fatalf("OpenPCloudShelf: %v", err)
	}
	waitReady(t)

	sum := sha256.Sum256([]byte("pcloud:scope-b"))
	blocked := filepath.Join(dataDir, "store", "offline", hex.EncodeToString(sum[:16]))
	if err := os.MkdirAll(filepath.Dir(blocked), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(blocked, nil, 0o600); err != nil {
		t.Fatal(err)
	}

	for name, open := range map[string]func() error{
		"empty scope": func() error {
			return OpenPCloudShelf("pcloud-a", "Cloud", srv.Host(), pcloudtest.Token, pcloudFolder, "", false)
		},
		"foreign host": func() error {
			return OpenPCloudShelf("pcloud-a", "Cloud", "example.com", pcloudtest.Token, pcloudFolder, "pcloud:scope-a", false)
		},
		"unopenable offline directory": func() error {
			return OpenPCloudShelf("pcloud-a", "Cloud", srv.Host(), pcloudtest.Token, pcloudFolder, "pcloud:scope-b", false)
		},
	} {
		if err := open(); err == nil {
			t.Errorf("%s: replacement succeeded", name)
		}
		waitReady(t)
		if titles := bookTitles(t, "pcloud-a"); !strings.Contains(titles, `"Dune"`) {
			t.Errorf("%s: after the failed replacement, books = %s", name, titles)
		}
	}
}

func TestPCloudReplaceAndCloseWaitForTasks(t *testing.T) {
	srv, _ := servePCloudShelf(t)
	startT(t, t.TempDir())
	if err := OpenPCloudShelf("pcloud-a", "Cloud", srv.Host(), pcloudtest.Token, pcloudFolder, "pcloud:scope-a", false); err != nil {
		t.Fatalf("OpenPCloudShelf: %v", err)
	}
	rt, err := running()
	if err != nil {
		t.Fatal(err)
	}
	old := taskWaitTimeout
	taskWaitTimeout = 50 * time.Millisecond
	t.Cleanup(func() { taskWaitTimeout = old })

	task := &heldTask{release: make(chan struct{})}
	release := sync.OnceFunc(func() { close(task.release) })
	t.Cleanup(release)
	if _, err := rt.app.TaskChains().Submit(&taskutil.TaskChain{Tasks: []taskutil.Task{task}}); err != nil {
		t.Fatal(err)
	}
	if err := OpenPCloudShelf("pcloud-a", "Cloud", srv.Host(), pcloudtest.Token, pcloudFolder, "pcloud:scope-a", false); !errors.Is(err, ErrBusy) {
		t.Errorf("replacing during a task: %v", err)
	}
	if err := CloseVirtualShelf("pcloud-a"); !errors.Is(err, ErrBusy) {
		t.Errorf("closing during a task: %v", err)
	}

	taskWaitTimeout = 5 * time.Second
	release()
	if err := CloseVirtualShelf("pcloud-a"); err != nil {
		t.Errorf("closing after the task: %v", err)
	}
}

func TestPCloudShelfFollowsDeviceNSFW(t *testing.T) {
	srv, _ := servePCloudShelf(t)
	startT(t, t.TempDir())
	if err := OpenPCloudShelf("pcloud-a", "Cloud", srv.Host(), pcloudtest.Token, pcloudFolder, "pcloud:scope-a", false); err != nil {
		t.Fatalf("OpenPCloudShelf: %v", err)
	}
	waitReady(t)
	if titles := bookTitles(t, "pcloud-a"); strings.Contains(titles, `"Adult"`) {
		t.Errorf("hidden on the device, books = %s", titles)
	}

	if err := SetShelfShowNSFW("pcloud-a", true); err != nil {
		t.Fatalf("SetShelfShowNSFW: %v", err)
	}
	if titles := bookTitles(t, "pcloud-a"); !strings.Contains(titles, `"Adult"`) {
		t.Errorf("shown on the device, books = %s", titles)
	}
	if res := handle(t, http.MethodGet, "/api/setting/show_nsfw", ""); !strings.Contains(string(res.Body), "false") {
		t.Errorf("the install-wide setting changed: %s", res.Body)
	}

	if err := SetShelfShowNSFW("pcloud-a", false); err != nil {
		t.Fatal(err)
	}
	out, err := ShelfNSFWMarks("pcloud-a")
	if err != nil {
		t.Fatalf("NSFWMarks: %v", err)
	}
	var marks map[string]struct {
		NSFW bool `json:"nsfw"`
	}
	if err := json.Unmarshal([]byte(out), &marks); err != nil {
		t.Fatal(err)
	}
	nsfw := 0
	for _, mark := range marks {
		if mark.NSFW {
			nsfw++
		}
	}
	if len(marks) != 2 || nsfw != 1 {
		t.Errorf("marks = %s, want both books with one marked", out)
	}

	if err := SetShelfShowNSFW("pcloud-b", true); err == nil {
		t.Error("a shelf that is not open accepted a choice")
	}
}

func TestPCloudShelfOfflineError(t *testing.T) {
	srv, _ := servePCloudShelf(t)
	srv.SetOffline(true)
	startT(t, t.TempDir())
	if err := OpenPCloudShelf("pcloud-a", "Cloud", srv.Host(), pcloudtest.Token, pcloudFolder, "pcloud:scope-a", false); err != nil {
		t.Fatalf("OpenPCloudShelf: %v", err)
	}
	rt, err := running()
	if err != nil {
		t.Fatal(err)
	}
	shelfData, ok := rt.app.ShelfManager().GetShelf("pcloud-a")
	if !ok {
		t.Fatal("pcloud-a not open")
	}
	_ = shelfData.WaitReady(t.Context())

	res := handle(t, http.MethodGet, "/api/shelves/pcloud-a/books", "")
	if res.Status != http.StatusServiceUnavailable || !strings.Contains(string(res.Body), "SOURCE_UNAVAILABLE") {
		t.Errorf("books while down: %d %s", res.Status, res.Body)
	}
}

func TestPCloudRemovalCleansOnlyItsScope(t *testing.T) {
	srv, bookID := servePCloudShelf(t)
	dataDir := t.TempDir()
	startT(t, dataDir)
	const scope = "pcloud:scope-a"
	if err := OpenPCloudShelf("cloud", "Cloud", srv.Host(), pcloudtest.Token, pcloudFolder, scope, false); err != nil {
		t.Fatal(err)
	}
	waitReady(t)
	if res := handle(t, http.MethodPut, "/api/shelves/cloud/books/"+bookID+"/download", ""); res.Status != http.StatusOK {
		t.Fatalf("download: %d %s", res.Status, res.Body)
	}
	if err := CloseVirtualShelf("cloud"); err != nil {
		t.Fatal(err)
	}
	assertOfflineScope(t, dataDir, scope, true)
	srv.SetOffline(true)
	if err := OpenPCloudShelf("cloud", "Cloud", srv.Host(), pcloudtest.Token, pcloudFolder, scope, false); err != nil {
		t.Fatal(err)
	}
	waitReady(t)
	if titles := bookTitles(t, "cloud"); !strings.Contains(titles, `"Dune"`) {
		t.Fatalf("offline listing: %s", titles)
	}
	if res := handle(t, http.MethodGet, "/api/shelves/cloud/books/"+bookID+"/content", ""); res.Status != http.StatusOK {
		t.Fatalf("offline content after reopening: %d %s", res.Status, res.Body)
	}

	other := offlineScopePath(dataDir, "other")
	if err := os.MkdirAll(other, 0700); err != nil {
		t.Fatal(err)
	}
	if err := RemoveVirtualShelfOffline(scope); err != nil {
		t.Fatal(err)
	}
	assertOfflineScope(t, dataDir, scope, false)
	assertOfflineScope(t, dataDir, "other", true)
	if err := RemoveVirtualShelfOffline(scope); err != nil {
		t.Fatalf("retry: %v", err)
	}
	if err := RemoveVirtualShelfOffline(""); err == nil {
		t.Fatal("empty scope accepted")
	}
	assertOfflineScope(t, dataDir, "other", true)
}

func offlineScopePath(dataDir, scope string) string {
	sum := sha256.Sum256([]byte(scope))
	return filepath.Join(dataDir, "store", "offline", hex.EncodeToString(sum[:16]))
}

func assertOfflineScope(t *testing.T, dataDir, scope string, want bool) {
	t.Helper()
	_, err := os.Stat(offlineScopePath(dataDir, scope))
	if want && err != nil {
		t.Fatalf("scope %q missing: %v", scope, err)
	}
	if !want && !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("scope %q still exists: %v", scope, err)
	}
}
