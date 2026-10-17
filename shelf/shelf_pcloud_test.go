package shelf

import (
	"os"
	"path"
	"slices"
	"strings"
	"testing"

	"github.com/voilelab/plainshelf/internal/pcloud"
	"github.com/voilelab/plainshelf/internal/pcloud/pcloudtest"
)

// openPCloudShelf opens libRoot through pcloud.FS, served by a fake pCloud API.
func openPCloudShelf(t *testing.T, libRoot string, conf ShelfConf) (*Shelf, *pcloudtest.Server) {
	t.Helper()
	return openPCloudShelfOffline(t, libRoot, conf, "")
}

// openPCloudShelfOffline is openPCloudShelf keeping offline copies in offlineDir.
func openPCloudShelfOffline(t *testing.T, libRoot string, conf ShelfConf, offlineDir string) (*Shelf, *pcloudtest.Server) {
	t.Helper()

	srv := pcloudtest.New(t, libRoot, "/PlainShelf/Shelf")
	client, err := pcloud.NewClient(srv.Host(), pcloudtest.Token, srv.Client())
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	s, err := NewShelfFromFS(&conf, pcloud.NewFS(client, "/PlainShelf/Shelf"), offlineDir)
	if err != nil {
		t.Fatalf("NewShelfFromFS: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	if err := s.WaitReady(t.Context()); err != nil {
		t.Fatalf("WaitReady: %v", err)
	}
	return s, srv
}

func TestPCloudShelfRequestsDoNotGrowWithBooks(t *testing.T) {
	counts := map[int][]string{}
	for _, n := range []int{2, 12} {
		titles := make([]string, n)
		for i := range titles {
			titles[i] = "Book " + string(rune('A'+i))
		}
		s, srv := openPCloudShelf(t, seedExportedShelf(t, titles...), ShelfConf{})

		if got := listedTitles(t, s); !slices.Equal(got, titles) {
			t.Errorf("n=%d: listed %v, want %v", n, got, titles)
		}
		if _, err := s.GetAllFolders(); err != nil {
			t.Fatalf("GetAllFolders: %v", err)
		}
		counts[n] = srv.Take()
		if srv.TokenLeaked() {
			t.Errorf("n=%d: the token reached a download or a query string", n)
		}
	}

	if !slices.Equal(counts[2], counts[12]) {
		t.Errorf("requests to open and list = %v by book count, want the same for every size", counts)
	}
}

// seedBookWithText writes an exported shelf holding one book whose source reads
// "Dune text", and returns the root and the book ID.
func seedBookWithText(t *testing.T) (string, string) {
	t.Helper()
	libRoot := t.TempDir()
	local := newTestShelf(t, &ShelfConf{LibRoot: libRoot, LockMode: "none", BookCacheWriterID: testWriterID})
	book, err := local.NewBook(FolderPath{}, "Dune")
	if err != nil {
		t.Fatalf("NewBook: %v", err)
	}
	if _, err := book.NewSource(strings.NewReader("Dune text")); err != nil {
		t.Fatalf("NewSource: %v", err)
	}
	if _, err := local.ExportBookCache(); err != nil {
		t.Fatalf("ExportBookCache: %v", err)
	}
	return libRoot, book.ID()
}

func TestPCloudShelfReadsBookText(t *testing.T) {
	libRoot, bookID := seedBookWithText(t)
	s, _ := openPCloudShelf(t, libRoot, ShelfConf{})
	if text, err := readCurrentText(t, s, bookID); err != nil || text != "Dune text" {
		t.Errorf("text = %q, %v", text, err)
	}
}

func TestPCloudShelfReadsADownloadedBookOffline(t *testing.T) {
	libRoot, bookID := seedBookWithText(t)
	s, srv := openPCloudShelfOffline(t, libRoot, ShelfConf{}, t.TempDir())
	if _, err := s.DownloadBook(bookID); err != nil {
		t.Fatalf("DownloadBook: %v", err)
	}

	srv.SetOffline(true)
	if text, err := readCurrentText(t, s, bookID); err != nil || text != "Dune text" {
		t.Errorf("offline text = %q, %v", text, err)
	}
}

// A source no installation exports is walked on the first open only: the
// offline snapshot answers later ones, and a rescan still walks.
func TestPCloudShelfWithoutExportOpensFromTheOfflineSnapshot(t *testing.T) {
	libRoot := seedExportedShelf(t, "Dune", "Solaris", "Hyperion")
	if err := os.RemoveAll(path.Join(libRoot, appFolder)); err != nil {
		t.Fatalf("remove app/: %v", err)
	}
	offlineDir := t.TempDir()

	first, srv := openPCloudShelfOffline(t, libRoot, ShelfConf{}, offlineDir)
	if got := count(srv.Take(), "download"); got < 3 {
		t.Fatalf("first open made %d downloads, want a walk reading every book", got)
	}
	if err := first.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	s, srv := openPCloudShelfOffline(t, libRoot, ShelfConf{}, offlineDir)
	if got, want := listedTitles(t, s), []string{"Dune", "Hyperion", "Solaris"}; !slices.Equal(got, want) {
		t.Errorf("reopened listing = %v, want %v", got, want)
	}
	// shelf.json at most; no book is read.
	if got := count(srv.Take(), "download"); got > 1 {
		t.Errorf("reopening made %d downloads, want the snapshot to answer", got)
	}

	if _, err := s.RescanUnthrottled(); err != nil {
		t.Fatalf("Rescan: %v", err)
	}
	if got := count(srv.Take(), "download"); got < 3 {
		t.Errorf("rescan made %d downloads, want a walk", got)
	}
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
