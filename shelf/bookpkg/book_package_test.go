package bookpkg

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/voilelab/plainshelf/internal/fsutil"
)

// newBookPackageFixture copies the committed book fixture into a temp directory
// named like a package a user would pick, so a test may add to it and any
// accidental write lands outside the repository.
func newBookPackageFixture(t *testing.T) string {
	t.Helper()

	dir := filepath.Join(t.TempDir(), "dune.bookpkg")
	if err := os.CopyFS(dir, os.DirFS("testdata/book-a82m")); err != nil {
		t.Fatalf("copying fixture: %v", err)
	}
	return dir
}

func openFixturePackage(t *testing.T, dir string) *BookPackage {
	t.Helper()

	pkg, err := OpenBookPackage(dir, nil)
	if err != nil {
		t.Fatalf("OpenBookPackage: %v", err)
	}
	t.Cleanup(func() { pkg.Close() }) //nolint:errcheck // test cleanup

	return pkg
}

// The whole point of the package reader: one folder, no shelf around it, and
// everything a reader displays comes back.
func TestOpenBookPackageReadsWithoutAShelf(t *testing.T) {
	const sourceID = "20260315-a1"

	dir := newBookPackageFixture(t)
	book := openFixturePackage(t, dir).Book()

	if book.ID() == "" {
		t.Error("expected the book to carry its persisted ID")
	}
	if book.Title() == "" {
		t.Error("expected the book to carry its title")
	}
	// A package opened outside a shelf has no layer: layer is a book's position
	// in the tree, and there is no tree here. There is nothing to assert about a
	// layer the book no longer carries — the point is that opening a bare
	// package needs no layer information at all.

	source, err := book.ResolveCurrentSource()
	if err != nil {
		t.Fatalf("ResolveCurrentSource: %v", err)
	}
	if source.ID() != sourceID {
		t.Errorf("current source = %q, want %q", source.ID(), sourceID)
	}

	content, err := source.Open()
	if err != nil {
		t.Fatalf("Source.Open: %v", err)
	}
	defer content.Close() //nolint:errcheck // read-only handle in a test

	text, err := io.ReadAll(content)
	if err != nil {
		t.Fatalf("reading source content: %v", err)
	}
	if len(text) == 0 {
		t.Error("expected the source to have content")
	}

	if _, _, err := book.OpenCover(); err != nil {
		t.Fatalf("OpenCover: %v", err)
	}

	sources, err := book.ListSource()
	if err != nil {
		t.Fatalf("ListSource: %v", err)
	}
	if len(sources) == 0 {
		t.Error("expected the package to list its sources")
	}
}

// Illustrations live inside the package, so the reader reaches them through the
// same handle rather than a shelf-relative path.
func TestOpenBookPackageReadsSourceAssets(t *testing.T) {
	const sourceID = "20260315-a1"

	dir := newBookPackageFixture(t)
	assetDir := filepath.Join(dir, SourcesFolder, sourceID, "assets")
	if err := os.MkdirAll(assetDir, 0o755); err != nil {
		t.Fatalf("creating assets dir: %v", err)
	}
	want := []byte("not really a png, but the reader only moves bytes")
	if err := os.WriteFile(filepath.Join(assetDir, "figure.png"), want, 0o644); err != nil {
		t.Fatalf("writing asset: %v", err)
	}

	book := openFixturePackage(t, dir).Book()
	source, err := book.GetSource(sourceID)
	if err != nil {
		t.Fatalf("GetSource: %v", err)
	}

	asset, err := source.OpenAsset("figure.png")
	if err != nil {
		t.Fatalf("OpenAsset: %v", err)
	}
	defer asset.File.Close() //nolint:errcheck // read-only handle in a test

	got, err := io.ReadAll(asset.File)
	if err != nil {
		t.Fatalf("reading asset: %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("asset content = %q, want %q", got, want)
	}
}

// A reader may be pointed at a book on a read-only mount, or at someone else's
// folder. Reading it must leave the directory byte-for-byte as it was: no
// app/, no lock file, no lazily upgraded book.json.
func TestOpenBookPackageWritesNothing(t *testing.T) {
	dir := newBookPackageFixture(t)
	before := treeSnapshot(t, dir)

	book := openFixturePackage(t, dir).Book()
	if _, err := book.ResolveCurrentSource(); err != nil {
		t.Fatalf("ResolveCurrentSource: %v", err)
	}
	if _, _, err := book.OpenCover(); err != nil {
		t.Fatalf("OpenCover: %v", err)
	}

	if diff := snapshotDiff(before, treeSnapshot(t, dir)); diff != "" {
		t.Errorf("opening a package changed the directory:\n%s", diff)
	}
}

// Read-only is a property of the handle, not a promise the caller keeps: every
// mutation is refused before it reaches the filesystem.
func TestOpenBookPackageRefusesWrites(t *testing.T) {
	dir := newBookPackageFixture(t)
	book := openFixturePackage(t, dir).Book()

	source, err := book.ResolveCurrentSource()
	if err != nil {
		t.Fatalf("ResolveCurrentSource: %v", err)
	}

	writes := map[string]func() error{
		"SetMeta":       func() error { return book.SetMeta(book.GetMeta()) },
		"SetCover":      func() error { return book.SetCover([]byte("data"), ".png") },
		"DeleteCover":   func() error { return book.DeleteCover() },
		"UpdateContent": func() error { return source.UpdateContent(strings.NewReader("new")) },
		"WriteAsset":    func() error { return source.WriteAsset("figure.png", []byte("data")) },
	}

	for name, write := range writes {
		if err := write(); !errors.Is(err, fsutil.ErrReadOnly) {
			t.Errorf("%s error = %v, want %v", name, err, fsutil.ErrReadOnly)
		}
	}
}

// The three ways a caller can point at the wrong thing have to be
// distinguishable: a reader says "no such folder", "that is not a book" and
// "this book is damaged" in three different places in its UI.
func TestOpenBookPackageRejectsNonPackages(t *testing.T) {
	t.Run("missing directory", func(t *testing.T) {
		_, err := OpenBookPackage(filepath.Join(t.TempDir(), "absent.bookpkg"), nil)
		if !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("error = %v, want %v", err, fs.ErrNotExist)
		}
	})

	t.Run("directory without book.json", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "not-a-book")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("creating directory: %v", err)
		}

		_, err := OpenBookPackage(dir, nil)
		if !errors.Is(err, ErrNotBookPackage) {
			t.Errorf("error = %v, want %v", err, ErrNotBookPackage)
		}
	})

	t.Run("damaged book.json", func(t *testing.T) {
		dir := newBookPackageFixture(t)
		if err := os.WriteFile(filepath.Join(dir, BookMetaFile), []byte("{not json"), 0o644); err != nil {
			t.Fatalf("damaging book.json: %v", err)
		}

		_, err := OpenBookPackage(dir, nil)
		if err == nil {
			t.Fatal("expected a damaged book.json to fail the open")
		}
		if errors.Is(err, ErrNotBookPackage) {
			t.Error("a damaged book.json must not be reported as 'not a book package'")
		}
	})
}

// A book written by a newer build still opens: refusing it would leave the
// reader with nothing to show, and the package is never written back anyway.
func TestOpenBookPackageOpensFutureSchema(t *testing.T) {
	dir := newBookPackageFixture(t)
	future, err := os.ReadFile("testdata/schema/v2-future/book.json")
	if err != nil {
		t.Fatalf("reading future fixture: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, BookMetaFile), future, 0o644); err != nil {
		t.Fatalf("writing future book.json: %v", err)
	}

	book := openFixturePackage(t, dir).Book()
	if book.ID() == "" {
		t.Error("expected a future-schema book to still read its metadata")
	}
}

// Close releases the directory handle, and stays safe to call twice — a caller
// that closes in a defer and again on an error path is doing the right thing.
func TestBookPackageCloseIsIdempotent(t *testing.T) {
	pkg, err := OpenBookPackage(newBookPackageFixture(t), nil)
	if err != nil {
		t.Fatalf("OpenBookPackage: %v", err)
	}

	if err := pkg.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	if err := pkg.Close(); err != nil {
		t.Errorf("second Close: %v", err)
	}
}

// A package whose book.json is newer than this build is refused twice over,
// and the schema guard is the one that answers: it deliberately runs before
// the write handle is narrowed, so that a refused write cannot truncate a
// cover or delete a file on its way to failing.
//
// This is Book's behaviour on any read-only shelf, not something the package
// reader chooses — the point of pinning it here is that "every mutation is
// refused" is the guarantee, and fsutil.ErrReadOnly is only how most of them
// say so.
func TestOpenBookPackageRefusesWritesToAFutureSchemaBook(t *testing.T) {
	dir := newBookPackageFixture(t)
	future, err := os.ReadFile("testdata/schema/v2-future/book.json")
	if err != nil {
		t.Fatalf("reading future fixture: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, BookMetaFile), future, 0o644); err != nil {
		t.Fatalf("writing future book.json: %v", err)
	}

	before := treeSnapshot(t, dir)
	book := openFixturePackage(t, dir).Book()

	// DeleteCover is absent deliberately: this fixture records no cover, so it
	// has nothing to delete and returns nil without reaching a guard.
	writes := map[string]func() error{
		"SetMeta":  func() error { return book.SetMeta(book.GetMeta()) },
		"SetCover": func() error { return book.SetCover([]byte("data"), ".png") },
	}
	for name, write := range writes {
		err := write()
		if err == nil {
			t.Errorf("%s: expected a refusal", name)
			continue
		}
		if !errors.Is(err, ErrUnsupportedBookSchemaVersion) && !errors.Is(err, fsutil.ErrReadOnly) {
			t.Errorf("%s error = %v, want the schema guard or %v", name, err, fsutil.ErrReadOnly)
		}
	}

	if diff := snapshotDiff(before, treeSnapshot(t, dir)); diff != "" {
		t.Errorf("a refused write changed the package:\n%s", diff)
	}
}
