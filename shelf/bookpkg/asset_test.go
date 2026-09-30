package bookpkg

import (
	"errors"
	"io"
	"os"
	"path"
	"strings"
	"testing"
)

// writeAsset bypasses WriteAsset, so the read path is exercised against a file
// dropped into the shelf by hand.
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

func newBookWithSource(t *testing.T, title, content string) (*Book, *Source, string) {
	t.Helper()

	book, _, libRoot := newTestBook(t, "asset-book", title)

	source, err := book.NewSource(strings.NewReader(content))
	if err != nil {
		t.Fatalf("NewSource: %v", err)
	}

	return book, source, libRoot
}

func TestSourceOpenAsset(t *testing.T) {
	_, source, libRoot := newBookWithSource(t, "Illustrated", "body")

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
	_, source, _ := newBookWithSource(t, "Written Art", "body")

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

// Writing a name the read path would refuse would create a file the server can
// never serve.
func TestSourceWriteAssetRejectsUnsafeNames(t *testing.T) {
	_, source, libRoot := newBookWithSource(t, "Unsafe Writes", "body")

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

func TestSourceDeleteAsset(t *testing.T) {
	_, source, libRoot := newBookWithSource(t, "Deletable Art", "body")
	if err := source.WriteAsset("img-0001.png", []byte("x")); err != nil {
		t.Fatalf("WriteAsset: %v", err)
	}

	if err := source.DeleteAsset("img-0001.png"); err != nil {
		t.Fatalf("DeleteAsset: %v", err)
	}
	if _, err := source.OpenAsset("img-0001.png"); !errors.Is(err, ErrAssetNotFound) {
		t.Fatalf("OpenAsset after delete: error = %v, want ErrAssetNotFound", err)
	}

	// A quiet success would hide a typo.
	if err := source.DeleteAsset("img-0001.png"); !errors.Is(err, ErrAssetNotFound) {
		t.Fatalf("second DeleteAsset: error = %v, want ErrAssetNotFound", err)
	}

	for _, name := range []string{"..", "../source.txt", "notes.txt", ""} {
		if err := source.DeleteAsset(name); !errors.Is(err, ErrInvalidAssetName) {
			t.Fatalf("DeleteAsset(%q): error = %v, want ErrInvalidAssetName", name, err)
		}
	}

	// The text is left alone; a dead link renders as its alt text.
	content, err := os.ReadFile(path.Join(libRoot, source.FolderPath(), SourceFile))
	if err != nil {
		t.Fatalf("read source.txt: %v", err)
	}
	if string(content) != "body" {
		t.Fatalf("source text changed by a delete: %q", content)
	}
}

func TestSourceOpenAssetMissing(t *testing.T) {
	_, source, libRoot := newBookWithSource(t, "No Pictures", "body")

	// Neither an absent assets/ directory nor an absent file is a server fault.
	if _, err := source.OpenAsset("img-0001.png"); !errors.Is(err, ErrAssetNotFound) {
		t.Fatalf("OpenAsset with no assets dir: error = %v, want ErrAssetNotFound", err)
	}

	writeAsset(t, libRoot, source, "img-0001.png", []byte("x"))

	if _, err := source.OpenAsset("img-0002.png"); !errors.Is(err, ErrAssetNotFound) {
		t.Fatalf("OpenAsset for absent file: error = %v, want ErrAssetNotFound", err)
	}
}

// A name resolving to a directory is a missing asset, not a server error.
func TestSourceOpenAssetRejectsDirectory(t *testing.T) {
	_, source, libRoot := newBookWithSource(t, "Directory Trap", "body")

	dirPath := path.Join(libRoot, source.FolderPath(), SourceAssetsFolder, "img-0001.png")
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		t.Fatalf("MkdirAll(%q): %v", dirPath, err)
	}

	if _, err := source.OpenAsset("img-0001.png"); !errors.Is(err, ErrAssetNotFound) {
		t.Fatalf("OpenAsset on a directory: error = %v, want ErrAssetNotFound", err)
	}
}

func TestSourceOpenAssetRejectsUnsafeNames(t *testing.T) {
	_, source, libRoot := newBookWithSource(t, "Unsafe Names", "body")

	// A real asset exists, so a rejected name cannot be mistaken for an empty
	// assets/ directory happening to answer the same way.
	writeAsset(t, libRoot, source, "img-0001.png", []byte("x"))

	// source.txt sits one level up, so the traversal cases below name a file
	// that genuinely exists: a pass proves refusal, not a missing target.
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
	_, source, libRoot := newBookWithSource(t, "Shouty Extension", "body")
	writeAsset(t, libRoot, source, "img-0001.JPG", []byte("x"))

	asset, err := source.OpenAsset("img-0001.JPG")
	if err != nil {
		t.Fatalf("OpenAsset with uppercase extension: %v", err)
	}
	asset.File.Close()
}

func TestSourceAssetETagTracksContent(t *testing.T) {
	_, source, libRoot := newBookWithSource(t, "Changing Art", "body")

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

// validateAssetName shares the ignored-name rule with layer and source-id
// validation through shelfutil, but must report it as ErrInvalidAssetName.
func TestIgnoredAssetNamesStayAssetErrors(t *testing.T) {
	err := validateAssetName("@eaDir")
	if !errors.Is(err, ErrInvalidAssetName) {
		t.Fatalf("validateAssetName(@eaDir) = %v, want ErrInvalidAssetName", err)
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

func TestSourceListAssets(t *testing.T) {
	_, source, libRoot := newBookWithSource(t, "Listable Art", "body")

	// No assets/ directory yet: an empty result, not an error.
	names, err := source.ListAssets()
	if err != nil {
		t.Fatalf("ListAssets with no assets dir: %v", err)
	}
	if len(names) != 0 {
		t.Fatalf("ListAssets with no assets dir = %v, want empty", names)
	}

	writeAsset(t, libRoot, source, "img-0002.png", []byte("two"))
	writeAsset(t, libRoot, source, "img-0001.webp", []byte("one"))

	// The list is exactly what a per-name request could open.
	writeAsset(t, libRoot, source, "notes.txt", []byte("prose"))
	writeAsset(t, libRoot, source, ".hidden.png", []byte("dot"))
	subDir := path.Join(libRoot, source.FolderPath(), SourceAssetsFolder, "nested")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("MkdirAll(%q): %v", subDir, err)
	}

	names, err = source.ListAssets()
	if err != nil {
		t.Fatalf("ListAssets: %v", err)
	}

	want := []string{"img-0001.webp", "img-0002.png"}
	if len(names) != len(want) {
		t.Fatalf("ListAssets = %v, want %v", names, want)
	}
	for i, name := range want {
		if names[i] != name {
			t.Fatalf("ListAssets = %v, want %v", names, want)
		}
	}
}

// The property that makes a separate orphan-collection pass unnecessary.
func TestDeleteSourceRemovesItsAssets(t *testing.T) {
	book, source, libRoot := newBookWithSource(t, "Doomed Art", "body")
	assetPath := writeAsset(t, libRoot, source, "img-0001.png", []byte("x"))

	if err := book.DeleteSource(source.ID()); err != nil {
		t.Fatalf("DeleteSource: %v", err)
	}

	if _, err := os.Stat(assetPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Stat(%q) after DeleteSource: err = %v, want ErrNotExist", assetPath, err)
	}
}

// An assets/ directory must not stop the book from listing its real source.
func TestAssetsDirIsNotMistakenForASource(t *testing.T) {
	book, source, libRoot := newBookWithSource(t, "One Source Only", "body")
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
