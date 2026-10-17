package mobile

import (
	"net/http"
	"strings"
	"testing"

	"github.com/voilelab/plainshelf/internal/pcloud/pcloudtest"
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

	if err := OpenPCloudShelf("pcloud-a", "Cloud", srv.Host(), pcloudtest.Token, pcloudFolder, "pcloud:scope-a"); err != nil {
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
		if err := OpenPCloudShelf("pcloud-a", "Cloud", srv.Host(), pcloudtest.Token, pcloudFolder, "pcloud:scope-a"); err != nil {
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
	if err := OpenPCloudShelf(local, "Cloud", srv.Host(), pcloudtest.Token, pcloudFolder, "pcloud:scope-a"); err == nil {
		t.Error("a pCloud shelf replaced a local one")
	}
	if err := OpenPCloudShelf("pcloud-b", "Cloud", srv.Host(), "", pcloudFolder, "pcloud:scope-b"); err == nil {
		t.Error("an empty token was accepted")
	}
}
