package shelf

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/voilelab/plainshelf/internal/fsutil"
	"github.com/voilelab/plainshelf/shelf/bookpkg"
)

func TestShelfRejectsUnsafeFolderSegments(t *testing.T) {
	tmpLib := filepath.Join(t.TempDir(), "shelf_test")
	shelf := newTestShelf(t, &ShelfConf{LibRoot: tmpLib})

	unsafeFolders := []FolderPath{
		{"..", "app", "evil"},
		{"."},
		{"safe/unsafe"},
		{"safe\\unsafe"},
		{"books.bookpkg"},
		{""},
	}

	for _, folders := range unsafeFolders {
		if err := shelf.NewFolder(folders[:len(folders)-1], folders[len(folders)-1]); err == nil {
			t.Fatalf("Expected NewFolder(%v) to reject unsafe folder", folders)
		}
		if _, err := shelf.NewBook(folders, "Unsafe Book"); err == nil {
			t.Fatalf("Expected NewBook(%v) to reject unsafe layer", folders)
		}
		if _, err := shelf.GetBooksByFolder(folders); err == nil {
			t.Fatalf("Expected GetBooksByFolder(%v) to reject unsafe layer", folders)
		}
	}

	if _, err := os.Stat(filepath.Join(tmpLib, appFolder, "evil")); !os.IsNotExist(err) {
		t.Fatalf("Unsafe layer traversal created unexpected path: %v", err)
	}
}

func TestMoveAndDeleteFolderRejectUnsafeSegments(t *testing.T) {
	tmpLib := filepath.Join(t.TempDir(), "shelf_test")
	shelf := newTestShelf(t, &ShelfConf{LibRoot: tmpLib})

	book, err := shelf.NewBook(FolderPath{"safe"}, "Safe Book")
	if err != nil {
		t.Fatalf("Failed to create book: %v", err)
	}

	if _, err := shelf.MoveBook(book.ID(), FolderPath{"..", "app", "evil"}); err == nil {
		t.Fatal("Expected MoveBook to reject unsafe target layer")
	}
	if err := shelf.DeleteFolder(FolderPath{"..", "app"}); err == nil {
		t.Fatal("Expected DeleteFolder to reject unsafe layer")
	}
	if err := shelf.RenameFolder(FolderPath{"safe"}, ".."); err == nil {
		t.Fatal("Expected RenameFolder to reject unsafe target name")
	}
	if err := shelf.MoveFolder(FolderPath{"safe"}, FolderPath{"..", "app"}); err == nil {
		t.Fatal("Expected MoveFolder to reject unsafe target layer")
	}
}

func TestBookRejectsUnsafeSourceID(t *testing.T) {
	testdataRoot, err := os.OpenRoot("testdata")
	if err != nil {
		t.Fatalf("Failed to open testdata root: %v", err)
	}
	defer testdataRoot.Close()

	book, err := bookpkg.Open(fsutil.NewRootFS(testdataRoot), newLoggerForTest(), "book-a82m")
	if err != nil {
		t.Fatalf("Failed to open book: %v", err)
	}

	for _, sourceID := range []string{"..", "../20260315-a1", "safe/unsafe", "safe\\unsafe", ""} {
		if _, err := book.GetSource(sourceID); err == nil {
			t.Fatalf("Expected GetSource(%q) to reject unsafe source ID", sourceID)
		}
	}
}
