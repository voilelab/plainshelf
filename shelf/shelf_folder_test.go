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
// walk costs and what listing folders used to pay on every request.
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

	counting := &countingDirFS{FS: writeRootForTest(t, s)}
	s.dbRoot = counting
	return counting
}

func folderNames(folders []FolderPath) []string {
	names := make([]string, 0, len(folders))
	for _, folder := range folders {
		names = append(names, folder.String())
	}
	return names
}

func getFolderNames(t *testing.T, s *Shelf) []string {
	t.Helper()

	folders, err := s.GetAllFolders()
	if err != nil {
		t.Fatalf("GetAllFolders: %v", err)
	}
	return folderNames(folders)
}

// GET /folders used to walk the whole tree on every call, which is the one read
// path scan_interval did not cover.
func TestGetAllFoldersSkipsTreeWalkWithinScanInterval(t *testing.T) {
	tmpLib := path.Join(t.TempDir(), "shelf_test")
	shelf := newTestShelf(t, &ShelfConf{LibRoot: tmpLib, LockMode: "none", ScanInterval: "10m", BookCheckInterval: "10m"})

	if _, err := shelf.NewBook(FolderPath{"Fiction", "Classics"}, "Dune"); err != nil {
		t.Fatalf("NewBook: %v", err)
	}
	if err := shelf.NewFolder(FolderPath{}, "Empty"); err != nil {
		t.Fatalf("NewFolder(Empty): %v", err)
	}

	counting := countBookTreeReads(t, shelf)

	want := []string{"", "Empty", "Fiction", "Fiction/Classics"}
	for i := range 5 {
		if got := getFolderNames(t, shelf); !slices.Equal(got, want) {
			t.Fatalf("call %d: layers = %v, want %v", i, got, want)
		}
	}

	if reads := counting.reads.Load(); reads != 0 {
		t.Errorf("listing layers 5 times read %d directories under %s, want 0 within the scan interval", reads, booksFolder)
	}

	// The counter is only meaningful if it does move when a walk is due.
	shelf.markBookCacheTreeDirty()
	if got := getFolderNames(t, shelf); !slices.Equal(got, want) {
		t.Fatalf("layers after a full scan = %v, want %v", got, want)
	}
	if reads := counting.reads.Load(); reads == 0 {
		t.Error("a listing with a dirty tree read no directories, so the counter proves nothing")
	}
}

// A folder the user just created must not wait for the next scan window.
func TestNewFolderIsListedImmediatelyWithinScanInterval(t *testing.T) {
	tmpLib := path.Join(t.TempDir(), "shelf_test")
	shelf := newTestShelf(t, &ShelfConf{LibRoot: tmpLib, LockMode: "none", ScanInterval: "10m", BookCheckInterval: "10m"})

	counting := countBookTreeReads(t, shelf)

	if err := shelf.NewFolder(FolderPath{"Fiction"}, "Classics"); err != nil {
		t.Fatalf("NewFolder: %v", err)
	}

	want := []string{"", "Fiction", "Fiction/Classics"}
	if got := getFolderNames(t, shelf); !slices.Equal(got, want) {
		t.Fatalf("layers = %v, want %v", got, want)
	}

	if reads := counting.reads.Load(); reads != 0 {
		t.Errorf("creating and listing a layer read %d directories under %s, want 0", reads, booksFolder)
	}

	// Creating a book creates its folders on the way, with the same requirement.
	if _, err := shelf.NewBook(FolderPath{"Poetry"}, "Odes"); err != nil {
		t.Fatalf("NewBook: %v", err)
	}
	if got := getFolderNames(t, shelf); !slices.Contains(got, "Poetry") {
		t.Errorf("layers = %v, want the layer created by NewBook", got)
	}
}

func TestDeleteFolderIsRemovedFromListingImmediately(t *testing.T) {
	tmpLib := path.Join(t.TempDir(), "shelf_test")
	shelf := newTestShelf(t, &ShelfConf{LibRoot: tmpLib, LockMode: "none", ScanInterval: "10m", BookCheckInterval: "10m"})

	if err := shelf.NewFolder(FolderPath{}, "Temporary"); err != nil {
		t.Fatalf("NewFolder: %v", err)
	}
	if err := shelf.DeleteFolder(FolderPath{"Temporary"}); err != nil {
		t.Fatalf("DeleteFolder: %v", err)
	}

	if got := getFolderNames(t, shelf); slices.Contains(got, "Temporary") {
		t.Errorf("layers = %v, want the deleted layer gone", got)
	}
}

// An empty folder holds no book, so a listing rebuilt from cached books would
// lose it. The scan has to record folders in their own right.
func TestGetAllFoldersKeepsEmptyFolderAcrossFullScan(t *testing.T) {
	tmpLib := path.Join(t.TempDir(), "shelf_test")
	shelf := newTestShelf(t, &ShelfConf{LibRoot: tmpLib, LockMode: "none", ScanInterval: "10m", BookCheckInterval: "10m"})

	if err := shelf.NewFolder(FolderPath{"Empty"}, "Nested"); err != nil {
		t.Fatalf("NewFolder: %v", err)
	}

	// Force the next listing through a full scan rather than the incremental
	// record NewFolder left behind.
	shelf.markBookCacheTreeDirty()

	want := []string{"", "Empty", "Empty/Nested"}
	if got := getFolderNames(t, shelf); !slices.Equal(got, want) {
		t.Fatalf("layers after a full scan = %v, want %v", got, want)
	}
}

func TestGetAllFoldersAfterRenameAndMove(t *testing.T) {
	tmpLib := path.Join(t.TempDir(), "shelf_test")
	shelf := newTestShelf(t, &ShelfConf{LibRoot: tmpLib, LockMode: "none", ScanInterval: "10m", BookCheckInterval: "10m"})

	if _, err := shelf.NewBook(FolderPath{"alpha", "beta"}, "Move Me"); err != nil {
		t.Fatalf("NewBook: %v", err)
	}
	if err := shelf.NewFolder(FolderPath{}, "gamma"); err != nil {
		t.Fatalf("NewFolder(gamma): %v", err)
	}

	if err := shelf.RenameFolder(FolderPath{"alpha", "beta"}, "delta"); err != nil {
		t.Fatalf("RenameFolder: %v", err)
	}
	want := []string{"", "alpha", "alpha/delta", "gamma"}
	if got := getFolderNames(t, shelf); !slices.Equal(got, want) {
		t.Fatalf("layers after rename = %v, want %v", got, want)
	}

	if err := shelf.MoveFolder(FolderPath{"alpha", "delta"}, FolderPath{"gamma"}); err != nil {
		t.Fatalf("MoveFolder: %v", err)
	}
	want = []string{"", "alpha", "gamma", "gamma/delta"}
	if got := getFolderNames(t, shelf); !slices.Equal(got, want) {
		t.Fatalf("layers after move = %v, want %v", got, want)
	}
}

// A book restored from trash may need a folder that was deleted meanwhile.
func TestRestoredBookFolderIsListedImmediately(t *testing.T) {
	tmpLib := path.Join(t.TempDir(), "shelf_test")
	shelf := newTestShelf(t, &ShelfConf{LibRoot: tmpLib, LockMode: "none", ScanInterval: "10m", BookCheckInterval: "10m"})

	book, err := shelf.NewBook(FolderPath{"Archive"}, "Recoverable")
	if err != nil {
		t.Fatalf("NewBook: %v", err)
	}
	if err := shelf.MoveBookToTrash(book.ID()); err != nil {
		t.Fatalf("MoveBookToTrash: %v", err)
	}
	if err := shelf.DeleteFolder(FolderPath{"Archive"}); err != nil {
		t.Fatalf("DeleteFolder: %v", err)
	}
	if got := getFolderNames(t, shelf); slices.Contains(got, "Archive") {
		t.Fatalf("layers = %v, want the deleted layer gone", got)
	}

	if err := shelf.RestoreTrashedBook(book.ID()); err != nil {
		t.Fatalf("RestoreTrashedBook: %v", err)
	}
	if got := getFolderNames(t, shelf); !slices.Contains(got, "Archive") {
		t.Errorf("layers = %v, want the restored book's layer back", got)
	}
}
