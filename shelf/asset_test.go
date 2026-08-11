package shelf

import (
	"errors"
	"io"
	"os"
	"path"
	"strings"
	"testing"
)

// writeAsset places a file under a source's assets/ directory the way a user
// dropping an illustration into the shelf by hand would, since nothing writes
// there through the API yet.
func writeAsset(t *testing.T, libRoot string, source *Source, name string, data []byte) string {
	t.Helper()

	assetDir := path.Join(libRoot, source.FolderPath(), SourceAssetsFolder)
	if err := os.MkdirAll(assetDir, 0755); err != nil {
		t.Fatalf("MkdirAll(%q): %v", assetDir, err)
	}

	assetPath := path.Join(assetDir, name)
	if err := os.WriteFile(assetPath, data, 0644); err != nil {
		t.Fatalf("WriteFile(%q): %v", assetPath, err)
	}
	return assetPath
}

// newBookWithSource returns a book carrying one source, plus that source.
func newBookWithSource(t *testing.T, s *Shelf, layers Layers, title, content string) (*Book, *Source) {
	t.Helper()

	book, err := s.NewBook(layers, title)
	if err != nil {
		t.Fatalf("NewBook: %v", err)
	}

	source, err := book.NewSource(strings.NewReader(content))
	if err != nil {
		t.Fatalf("NewSource: %v", err)
	}

	return book, source
}

func TestSourceOpenAsset(t *testing.T) {
	libRoot := path.Join(t.TempDir(), "shelf_test")
	s := newTestShelf(t, &ShelfConf{LibRoot: libRoot})

	_, source := newBookWithSource(t, s, nil, "Illustrated", "body")

	want := []byte("fake png bytes")
	writeAsset(t, libRoot, source, "img-0001.png", want)

	asset, err := source.OpenAsset("img-0001.png")
	if err != nil {
		t.Fatalf("OpenAsset: %v", err)
	}
	defer asset.File.Close()

	got, err := io.ReadAll(asset.File)
	if err != nil {
		t.Fatalf("read asset: %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("asset bytes = %q, want %q", got, want)
	}
	if asset.Ext != ".png" {
		t.Errorf("asset ext = %q, want .png", asset.Ext)
	}
	if asset.Info.Size() != int64(len(want)) {
		t.Errorf("asset size = %d, want %d", asset.Info.Size(), len(want))
	}
}

func TestSourceWriteAsset(t *testing.T) {
	libRoot := path.Join(t.TempDir(), "shelf_test")
	s := newTestShelf(t, &ShelfConf{LibRoot: libRoot})

	_, source := newBookWithSource(t, s, nil, "Written Art", "body")

	// The assets directory does not exist yet; writing has to create it.
	want := []byte("fake png bytes")
	if err := source.WriteAsset("img-0001.png", want); err != nil {
		t.Fatalf("WriteAsset: %v", err)
	}

	asset, err := source.OpenAsset("img-0001.png")
	if err != nil {
		t.Fatalf("OpenAsset after WriteAsset: %v", err)
	}
	defer asset.File.Close()

	got, err := io.ReadAll(asset.File)
	if err != nil {
		t.Fatalf("read asset: %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("asset bytes = %q, want %q", got, want)
	}

	// Rewriting replaces the contents rather than appending to them.
	if err := source.WriteAsset("img-0001.png", []byte("replaced")); err != nil {
		t.Fatalf("WriteAsset second time: %v", err)
	}
	again, err := source.OpenAsset("img-0001.png")
	if err != nil {
		t.Fatalf("OpenAsset after rewrite: %v", err)
	}
	defer again.File.Close()
	rewritten, err := io.ReadAll(again.File)
	if err != nil {
		t.Fatalf("read rewritten asset: %v", err)
	}
	if string(rewritten) != "replaced" {
		t.Errorf("rewritten asset = %q, want %q", rewritten, "replaced")
	}
}

// A name the read path would refuse must never reach the filesystem: writing it
// would create a file the server can never serve.
func TestSourceWriteAssetRejectsUnsafeNames(t *testing.T) {
	libRoot := path.Join(t.TempDir(), "shelf_test")
	s := newTestShelf(t, &ShelfConf{LibRoot: libRoot})

	_, source := newBookWithSource(t, s, nil, "Unsafe Writes", "body")

	for _, name := range []string{"", "..", "../escaped.png", "sub/img.png", ".hidden.png", "notes.txt"} {
		if err := source.WriteAsset(name, []byte("x")); !errors.Is(err, ErrInvalidAssetName) {
			t.Fatalf("WriteAsset(%q): error = %v, want ErrInvalidAssetName", name, err)
		}
	}

	// Nothing was created on the way to being refused.
	if _, err := os.Stat(path.Join(libRoot, source.FolderPath(), SourceAssetsFolder)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("assets dir exists after only refused writes: err = %v", err)
	}
	if _, err := os.Stat(path.Join(libRoot, source.FolderPath(), "escaped.png")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("a refused name escaped the assets dir: err = %v", err)
	}
}

func TestSourceOpenAssetMissing(t *testing.T) {
	libRoot := path.Join(t.TempDir(), "shelf_test")
	s := newTestShelf(t, &ShelfConf{LibRoot: libRoot})

	_, source := newBookWithSource(t, s, nil, "No Pictures", "body")

	// Neither an absent assets/ directory nor an absent file inside one is a
	// server fault; both must be reported as a missing asset.
	if _, err := source.OpenAsset("img-0001.png"); !errors.Is(err, ErrAssetNotFound) {
		t.Fatalf("OpenAsset with no assets dir: error = %v, want ErrAssetNotFound", err)
	}

	writeAsset(t, libRoot, source, "img-0001.png", []byte("x"))

	if _, err := source.OpenAsset("img-0002.png"); !errors.Is(err, ErrAssetNotFound) {
		t.Fatalf("OpenAsset for absent file: error = %v, want ErrAssetNotFound", err)
	}
}

// A name that resolves to a directory must read as a missing asset rather than
// surfacing the read failure as a server error.
func TestSourceOpenAssetRejectsDirectory(t *testing.T) {
	libRoot := path.Join(t.TempDir(), "shelf_test")
	s := newTestShelf(t, &ShelfConf{LibRoot: libRoot})

	_, source := newBookWithSource(t, s, nil, "Directory Trap", "body")

	dirPath := path.Join(libRoot, source.FolderPath(), SourceAssetsFolder, "img-0001.png")
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		t.Fatalf("MkdirAll(%q): %v", dirPath, err)
	}

	if _, err := source.OpenAsset("img-0001.png"); !errors.Is(err, ErrAssetNotFound) {
		t.Fatalf("OpenAsset on a directory: error = %v, want ErrAssetNotFound", err)
	}
}

func TestSourceOpenAssetRejectsUnsafeNames(t *testing.T) {
	libRoot := path.Join(t.TempDir(), "shelf_test")
	s := newTestShelf(t, &ShelfConf{LibRoot: libRoot})

	_, source := newBookWithSource(t, s, nil, "Unsafe Names", "body")

	// A real asset exists, so a rejected name cannot be mistaken for an empty
	// assets/ directory happening to answer the same way.
	writeAsset(t, libRoot, source, "img-0001.png", []byte("x"))

	// The text file sits beside assets/, one level up: the traversal cases
	// below name a file that genuinely exists, so a passing test proves the
	// name was refused rather than merely missing its target.
	cases := map[string]string{
		"empty":              "",
		"dot":                ".",
		"parent":             "..",
		"traversal":          "../source.txt",
		"nested":             "sub/img-0001.png",
		"backslash":          `..\source.txt`,
		"absolute":           "/etc/hostname",
		"hidden":             ".hidden.png",
		"unsupported ext":    "source.txt",
		"no ext":             "img-0001",
		"uppercase traverse": "../SOURCE.TXT",
	}

	for name, assetName := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := source.OpenAsset(assetName); !errors.Is(err, ErrInvalidAssetName) {
				t.Fatalf("OpenAsset(%q): error = %v, want ErrInvalidAssetName", assetName, err)
			}
			if etag := source.AssetETag(assetName); etag != "" {
				t.Fatalf("AssetETag(%q) = %q, want empty", assetName, etag)
			}
		})
	}
}

func TestSourceAssetExtensionsAreCaseInsensitive(t *testing.T) {
	libRoot := path.Join(t.TempDir(), "shelf_test")
	s := newTestShelf(t, &ShelfConf{LibRoot: libRoot})

	_, source := newBookWithSource(t, s, nil, "Shouty Extension", "body")
	writeAsset(t, libRoot, source, "img-0001.JPG", []byte("x"))

	asset, err := source.OpenAsset("img-0001.JPG")
	if err != nil {
		t.Fatalf("OpenAsset with uppercase extension: %v", err)
	}
	asset.File.Close()
}

func TestSourceAssetETagTracksContent(t *testing.T) {
	libRoot := path.Join(t.TempDir(), "shelf_test")
	s := newTestShelf(t, &ShelfConf{LibRoot: libRoot})

	_, source := newBookWithSource(t, s, nil, "Changing Art", "body")

	if etag := source.AssetETag("img-0001.png"); etag != "" {
		t.Fatalf("AssetETag for a missing asset = %q, want empty", etag)
	}

	writeAsset(t, libRoot, source, "img-0001.png", []byte("first"))
	first := source.AssetETag("img-0001.png")
	if first == "" {
		t.Fatal("AssetETag for an existing asset is empty")
	}

	writeAsset(t, libRoot, source, "img-0001.png", []byte("second is longer"))
	if second := source.AssetETag("img-0001.png"); second == first {
		t.Fatalf("AssetETag unchanged after rewriting the asset: %q", second)
	}
}

func TestIsSupportedImageExt(t *testing.T) {
	supported := []string{".jpg", ".jpeg", ".png", ".webp", ".gif", ".PNG", ".JpEg"}
	for _, ext := range supported {
		if !IsSupportedImageExt(ext) {
			t.Errorf("IsSupportedImageExt(%q) = false, want true", ext)
		}
	}

	unsupported := []string{"", ".txt", ".svg", ".bmp", "png", ".png.txt"}
	for _, ext := range unsupported {
		if IsSupportedImageExt(ext) {
			t.Errorf("IsSupportedImageExt(%q) = true, want false", ext)
		}
	}
}

// Assets live inside the book package, so every operation that relocates the
// package must carry them along without the book's identity changing.
func TestSourceAssetsSurviveMoveAndTrashRestore(t *testing.T) {
	libRoot := path.Join(t.TempDir(), "shelf_test")
	s := newTestShelf(t, &ShelfConf{LibRoot: libRoot})

	book, source := newBookWithSource(t, s, Layers{"before"}, "Travelling Art", "body")
	bookID := book.ID()
	sourceID := source.ID()
	want := []byte("fake png bytes")
	writeAsset(t, libRoot, source, "img-0001.png", want)

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

// Deleting a source takes its assets with it. This is the property that makes
// a separate orphan-collection pass unnecessary.
func TestDeleteSourceRemovesItsAssets(t *testing.T) {
	libRoot := path.Join(t.TempDir(), "shelf_test")
	s := newTestShelf(t, &ShelfConf{LibRoot: libRoot})

	book, source := newBookWithSource(t, s, nil, "Doomed Art", "body")
	assetPath := writeAsset(t, libRoot, source, "img-0001.png", []byte("x"))

	if err := book.DeleteSource(source.ID()); err != nil {
		t.Fatalf("DeleteSource: %v", err)
	}

	if _, err := os.Stat(assetPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Stat(%q) after DeleteSource: err = %v, want ErrNotExist", assetPath, err)
	}
}

// An assets/ directory must be inert to the code that walks the shelf: it is
// not a source, and it must not stop the book from being found.
func TestAssetsDirIsNotMistakenForASource(t *testing.T) {
	libRoot := path.Join(t.TempDir(), "shelf_test")
	s := newTestShelf(t, &ShelfConf{LibRoot: libRoot})

	book, source := newBookWithSource(t, s, nil, "One Source Only", "body")
	writeAsset(t, libRoot, source, "img-0001.png", []byte("x"))

	sources, err := book.ListSource()
	if err != nil {
		t.Fatalf("ListSource: %v", err)
	}
	if len(sources) != 1 {
		t.Fatalf("ListSource returned %d sources, want 1", len(sources))
	}
	if sources[0].ID() != source.ID() {
		t.Fatalf("ListSource returned source %q, want %q", sources[0].ID(), source.ID())
	}
}
