package shelf

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/voilelab/plainshelf/internal/fsutil"
)

func TestShelfRejectsUnsafeLayerSegments(t *testing.T) {
	tmpLib := filepath.Join(t.TempDir(), "shelf_test")
	shelf, err := OpenLocalShelf(tmpLib)
	if err != nil {
		t.Fatalf("Failed to initialize Shelf: %v", err)
	}
	defer shelf.Close()

	unsafeLayers := []Layers{
		{"..", "app", "evil"},
		{"."},
		{"safe/unsafe"},
		{"safe\\unsafe"},
		{"books.novl"},
		{""},
	}

	for _, layers := range unsafeLayers {
		if err := shelf.NewLayer(layers); err == nil {
			t.Fatalf("Expected NewLayer(%v) to reject unsafe layer", layers)
		}
		if _, err := shelf.NewBook(layers, "Unsafe Book"); err == nil {
			t.Fatalf("Expected NewBook(%v) to reject unsafe layer", layers)
		}
		if _, err := shelf.GetBooksByLayer(layers); err == nil {
			t.Fatalf("Expected GetBooksByLayer(%v) to reject unsafe layer", layers)
		}
	}

	if _, err := os.Stat(filepath.Join(tmpLib, appFolder, "evil")); !os.IsNotExist(err) {
		t.Fatalf("Unsafe layer traversal created unexpected path: %v", err)
	}
}

func TestMoveAndDeleteLayerRejectUnsafeSegments(t *testing.T) {
	tmpLib := filepath.Join(t.TempDir(), "shelf_test")
	shelf, err := OpenLocalShelf(tmpLib)
	if err != nil {
		t.Fatalf("Failed to initialize Shelf: %v", err)
	}
	defer shelf.Close()

	book, err := shelf.NewBook(Layers{"safe"}, "Safe Book")
	if err != nil {
		t.Fatalf("Failed to create book: %v", err)
	}

	if _, err := shelf.MoveBook(book.ID(), Layers{"..", "app", "evil"}); err == nil {
		t.Fatal("Expected MoveBook to reject unsafe target layer")
	}
	if err := shelf.DeleteLayer(Layers{"..", "app"}); err == nil {
		t.Fatal("Expected DeleteLayer to reject unsafe layer")
	}
}

func TestBookRejectsUnsafeSnapshotID(t *testing.T) {
	testdataRoot, err := os.OpenRoot("testdata")
	if err != nil {
		t.Fatalf("Failed to open testdata root: %v", err)
	}
	defer testdataRoot.Close()

	book, err := openBook(fsutil.NewRootFS(testdataRoot), "book-a82m")
	if err != nil {
		t.Fatalf("Failed to open book: %v", err)
	}

	for _, snapshotID := range []string{"..", "../20260315-a1", "safe/unsafe", "safe\\unsafe", ""} {
		if _, err := book.GetSnapshot(snapshotID); err == nil {
			t.Fatalf("Expected GetSnapshot(%q) to reject unsafe snapshot ID", snapshotID)
		}
	}
}
