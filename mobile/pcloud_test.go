package mobile

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json/v2"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/voilelab/plainshelf/internal/pcloud/pcloudtest"
	"github.com/voilelab/plainshelf/internal/taskutil"
	"github.com/voilelab/plainshelf/shelf"
)

const pcloudFolder = "/PlainShelf/Shelf"

// servePCloudShelf serves a shelf holding one book with text over a fake
// pCloud API, and returns the server and the book's ID.
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
	if _, err := book.NewSource(strings.NewReader("Dune text")); err != nil {
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
	// Not one of the shelves the app keeps.
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

func TestPCloudShelfFailedReplacementKeepsTheOldOne(t *testing.T) {
	srv, _ := servePCloudShelf(t)
	dataDir := t.TempDir()
	startT(t, dataDir)
	if err := OpenPCloudShelf("pcloud-a", "Cloud", srv.Host(), pcloudtest.Token, pcloudFolder, "pcloud:scope-a", false); err != nil {
		t.Fatalf("OpenPCloudShelf: %v", err)
	}
	waitReady(t)

	// A file where scope-b's offline directory goes makes opening it fail
	// after the old shelf is closed.
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

func TestPCloudShelfReplaceAndCloseWaitForTasks(t *testing.T) {
	srv, _ := servePCloudShelf(t)
	startT(t, t.TempDir())
	if err := OpenPCloudShelf("pcloud-a", "Cloud", srv.Host(), pcloudtest.Token, pcloudFolder, "pcloud:scope-a", false); err != nil {
		t.Fatalf("OpenPCloudShelf: %v", err)
	}
	rt, err := running()
	if err != nil {
		t.Fatal(err)
	}
	old := quiesceTimeout
	quiesceTimeout = 50 * time.Millisecond
	t.Cleanup(func() { quiesceTimeout = old })

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

	quiesceTimeout = 5 * time.Second
	release()
	if err := CloseVirtualShelf("pcloud-a"); err != nil {
		t.Errorf("closing after the task: %v", err)
	}
}

// The device's choice decides what the pCloud shelf serves, whatever the
// install-wide setting the local shelves follow says.
func TestPCloudShelfFollowsTheDevicesAdultContentChoice(t *testing.T) {
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

	// The marks cover the books the device hides, too.
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
