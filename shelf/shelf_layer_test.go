package shelf

import (
	"io/fs"
	"path"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/voilelab/plainshelf/internal/fsutil"
)

// countingDirFS counts directory reads under books/, which is what a full tree
// walk costs and what listing layers used to pay on every request.
type countingDirFS struct {
	fsutil.FS
	reads atomic.Int64
}

func (f *countingDirFS) ReadDir(name string) ([]fs.DirEntry, error) {
	if name == booksFolder || strings.HasPrefix(name, booksFolder+"/") {
		f.reads.Add(1)
	}
	return f.FS.ReadDir(name)
}

// countBookTreeReads swaps a counting filesystem into a shelf whose initial scan
// has already finished, so only I/O caused by the test itself is counted.
//
// Safe despite the background refresh goroutines: the swap happens before the
// call that could start one, and starting a goroutine orders the write ahead of
// anything it reads. The shelf must therefore have no book cache writer ID, or
// the export started by the initial scan would still be running.
func countBookTreeReads(t *testing.T, s *Shelf) *countingDirFS {
	t.Helper()

	if s.bookCacheWriterID != "" {
		t.Fatal("countBookTreeReads needs a shelf without a book cache writer ID")
	}

	counting := &countingDirFS{FS: s.dbRoot}
	s.dbRoot = counting
	return counting
}

func layerNames(layers []Layers) []string {
	names := make([]string, 0, len(layers))
	for _, layer := range layers {
		names = append(names, layer.String())
	}
	return names
}

func getLayerNames(t *testing.T, s *Shelf) []string {
	t.Helper()

	layers, err := s.GetAllLayers()
	if err != nil {
		t.Fatalf("GetAllLayers: %v", err)
	}
	return layerNames(layers)
}

// GET /layers used to walk the whole tree on every call, which is the one read
// path scan_interval did not cover.
func TestGetAllLayersSkipsTreeWalkWithinScanInterval(t *testing.T) {
	tmpLib := path.Join(t.TempDir(), "shelf_test")
	shelf := newTestShelf(t, &ShelfConf{LibRoot: tmpLib, LockMode: "none", ScanInterval: "10m", BookCheckInterval: "10m"})

	if _, err := shelf.NewBook(Layers{"Fiction", "Classics"}, "Dune"); err != nil {
		t.Fatalf("NewBook: %v", err)
	}
	if err := shelf.NewLayer(Layers{"Empty"}); err != nil {
		t.Fatalf("NewLayer(Empty): %v", err)
	}

	counting := countBookTreeReads(t, shelf)

	want := []string{"", "Empty", "Fiction", "Fiction/Classics"}
	for i := range 5 {
		if got := getLayerNames(t, shelf); !slices.Equal(got, want) {
			t.Fatalf("call %d: layers = %v, want %v", i, got, want)
		}
	}

	if reads := counting.reads.Load(); reads != 0 {
		t.Errorf("listing layers 5 times read %d directories under %s, want 0 within the scan interval", reads, booksFolder)
	}

	// The counter is only meaningful if it does move when a walk is due.
	shelf.markBookCacheTreeDirty()
	if got := getLayerNames(t, shelf); !slices.Equal(got, want) {
		t.Fatalf("layers after a full scan = %v, want %v", got, want)
	}
	if reads := counting.reads.Load(); reads == 0 {
		t.Error("a listing with a dirty tree read no directories, so the counter proves nothing")
	}
}

// A layer the user just created must not wait for the next scan window.
func TestNewLayerIsListedImmediatelyWithinScanInterval(t *testing.T) {
	tmpLib := path.Join(t.TempDir(), "shelf_test")
	shelf := newTestShelf(t, &ShelfConf{LibRoot: tmpLib, LockMode: "none", ScanInterval: "10m", BookCheckInterval: "10m"})

	counting := countBookTreeReads(t, shelf)

	if err := shelf.NewLayer(Layers{"Fiction", "Classics"}); err != nil {
		t.Fatalf("NewLayer: %v", err)
	}

	want := []string{"", "Fiction", "Fiction/Classics"}
	if got := getLayerNames(t, shelf); !slices.Equal(got, want) {
		t.Fatalf("layers = %v, want %v", got, want)
	}

	if reads := counting.reads.Load(); reads != 0 {
		t.Errorf("creating and listing a layer read %d directories under %s, want 0", reads, booksFolder)
	}

	// Creating a book creates its layers on the way, with the same requirement.
	if _, err := shelf.NewBook(Layers{"Poetry"}, "Odes"); err != nil {
		t.Fatalf("NewBook: %v", err)
	}
	if got := getLayerNames(t, shelf); !slices.Contains(got, "Poetry") {
		t.Errorf("layers = %v, want the layer created by NewBook", got)
	}
}

func TestDeleteLayerIsRemovedFromListingImmediately(t *testing.T) {
	tmpLib := path.Join(t.TempDir(), "shelf_test")
	shelf := newTestShelf(t, &ShelfConf{LibRoot: tmpLib, LockMode: "none", ScanInterval: "10m", BookCheckInterval: "10m"})

	if err := shelf.NewLayer(Layers{"Temporary"}); err != nil {
		t.Fatalf("NewLayer: %v", err)
	}
	if err := shelf.DeleteLayer(Layers{"Temporary"}); err != nil {
		t.Fatalf("DeleteLayer: %v", err)
	}

	if got := getLayerNames(t, shelf); slices.Contains(got, "Temporary") {
		t.Errorf("layers = %v, want the deleted layer gone", got)
	}
}

// An empty layer holds no book, so a listing rebuilt from cached books would
// lose it. The scan has to record layers in their own right.
func TestGetAllLayersKeepsEmptyLayerAcrossFullScan(t *testing.T) {
	tmpLib := path.Join(t.TempDir(), "shelf_test")
	shelf := newTestShelf(t, &ShelfConf{LibRoot: tmpLib, LockMode: "none", ScanInterval: "10m", BookCheckInterval: "10m"})

	if err := shelf.NewLayer(Layers{"Empty", "Nested"}); err != nil {
		t.Fatalf("NewLayer: %v", err)
	}

	// Force the next listing through a full scan rather than the incremental
	// record NewLayer left behind.
	shelf.markBookCacheTreeDirty()

	want := []string{"", "Empty", "Empty/Nested"}
	if got := getLayerNames(t, shelf); !slices.Equal(got, want) {
		t.Fatalf("layers after a full scan = %v, want %v", got, want)
	}
}

func TestGetAllLayersAfterRenameAndMove(t *testing.T) {
	tmpLib := path.Join(t.TempDir(), "shelf_test")
	shelf := newTestShelf(t, &ShelfConf{LibRoot: tmpLib, LockMode: "none", ScanInterval: "10m", BookCheckInterval: "10m"})

	if _, err := shelf.NewBook(Layers{"alpha", "beta"}, "Move Me"); err != nil {
		t.Fatalf("NewBook: %v", err)
	}
	if err := shelf.NewLayer(Layers{"gamma"}); err != nil {
		t.Fatalf("NewLayer(gamma): %v", err)
	}

	if err := shelf.RenameLayer(Layers{"alpha", "beta"}, Layers{"alpha", "delta"}); err != nil {
		t.Fatalf("RenameLayer: %v", err)
	}
	want := []string{"", "alpha", "alpha/delta", "gamma"}
	if got := getLayerNames(t, shelf); !slices.Equal(got, want) {
		t.Fatalf("layers after rename = %v, want %v", got, want)
	}

	if err := shelf.MoveLayer(Layers{"alpha", "delta"}, Layers{"gamma"}); err != nil {
		t.Fatalf("MoveLayer: %v", err)
	}
	want = []string{"", "alpha", "gamma", "gamma/delta"}
	if got := getLayerNames(t, shelf); !slices.Equal(got, want) {
		t.Fatalf("layers after move = %v, want %v", got, want)
	}
}

// A book restored from trash may need a layer that was deleted meanwhile.
func TestRestoredBookLayerIsListedImmediately(t *testing.T) {
	tmpLib := path.Join(t.TempDir(), "shelf_test")
	shelf := newTestShelf(t, &ShelfConf{LibRoot: tmpLib, LockMode: "none", ScanInterval: "10m", BookCheckInterval: "10m"})

	book, err := shelf.NewBook(Layers{"Archive"}, "Recoverable")
	if err != nil {
		t.Fatalf("NewBook: %v", err)
	}
	if err := shelf.MoveBookToTrash(book.ID()); err != nil {
		t.Fatalf("MoveBookToTrash: %v", err)
	}
	if err := shelf.DeleteLayer(Layers{"Archive"}); err != nil {
		t.Fatalf("DeleteLayer: %v", err)
	}
	if got := getLayerNames(t, shelf); slices.Contains(got, "Archive") {
		t.Fatalf("layers = %v, want the deleted layer gone", got)
	}

	if err := shelf.RestoreTrashedBook(book.ID()); err != nil {
		t.Fatalf("RestoreTrashedBook: %v", err)
	}
	if got := getLayerNames(t, shelf); !slices.Contains(got, "Archive") {
		t.Errorf("layers = %v, want the restored book's layer back", got)
	}
}
