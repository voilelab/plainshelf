package shelf

import (
	"io"
	"os"
	"path"
	"strings"
	"testing"
)

// writeShelfAsset plants a file under a source's assets/ directory by hand, the
// way a user dropping an illustration into the shelf would, so the read path is
// exercised against a file it did not write itself.
func writeShelfAsset(t *testing.T, libRoot string, source *Source, name string, data []byte) {
	t.Helper()

	assetDir := path.Join(libRoot, source.FolderPath(), SourceAssetsFolder)
	if err := os.MkdirAll(assetDir, 0755); err != nil {
		t.Fatalf("MkdirAll(%q): %v", assetDir, err)
	}
	if err := os.WriteFile(path.Join(assetDir, name), data, 0644); err != nil {
		t.Fatalf("WriteFile(%q): %v", name, err)
	}
}

// Assets live inside the book package, so every shelf operation that relocates
// the package must carry them along without the book's identity changing. The
// book-package-level asset behaviour is covered in shelf/bookpkg; this test pins
// the shelf move/trash/restore path on top of it.
func TestSourceAssetsSurviveMoveAndTrashRestore(t *testing.T) {
	libRoot := path.Join(t.TempDir(), "shelf_test")
	s := newTestShelf(t, &ShelfConf{LibRoot: libRoot})

	book, err := s.NewBook(Layers{"before"}, "Travelling Art")
	if err != nil {
		t.Fatalf("NewBook: %v", err)
	}
	source, err := book.NewSource(strings.NewReader("body"))
	if err != nil {
		t.Fatalf("NewSource: %v", err)
	}
	bookID := book.ID()
	sourceID := source.ID()
	want := []byte("fake png bytes")
	writeShelfAsset(t, libRoot, source, "img-0001.png", want)

	assertAssetReadable := func(t *testing.T, stage string) {
		t.Helper()

		moved, err := s.GetBook(bookID)
		if err != nil {
			t.Fatalf("GetBook after %s: %v", stage, err)
		}
		movedSource, err := moved.GetSource(sourceID)
		if err != nil {
			t.Fatalf("GetSource after %s: %v", stage, err)
		}
		asset, err := movedSource.OpenAsset("img-0001.png")
		if err != nil {
			t.Fatalf("OpenAsset after %s: %v", stage, err)
		}
		defer asset.File.Close()

		got, err := io.ReadAll(asset.File)
		if err != nil {
			t.Fatalf("read asset after %s: %v", stage, err)
		}
		if string(got) != string(want) {
			t.Fatalf("asset bytes after %s = %q, want %q", stage, got, want)
		}
	}

	if _, err := s.MoveBook(bookID, Layers{"after", "nested"}); err != nil {
		t.Fatalf("MoveBook: %v", err)
	}
	assertAssetReadable(t, "move")

	if err := s.MoveBookToTrash(bookID); err != nil {
		t.Fatalf("MoveBookToTrash: %v", err)
	}
	if err := s.RestoreTrashedBook(bookID); err != nil {
		t.Fatalf("RestoreTrashedBook: %v", err)
	}
	assertAssetReadable(t, "trash and restore")
}
