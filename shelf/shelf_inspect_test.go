package shelf

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/voilelab/plainshelf/internal/fsutil"
)

func rootFS(t *testing.T, dir string) fsutil.ReadFS {
	t.Helper()
	rt, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { rt.Close() })
	return fsutil.NewRootFS(rt)
}

func TestCountBookPackagesSkipsIgnoredDirsAndDoesNotDescendIntoBooks(t *testing.T) {
	dir := t.TempDir()
	for _, d := range []string{
		"books/a.bookpkg/nested.bookpkg",
		"books/Fiction/b.bookpkg",
		"books/Fiction/Empty",
		"books/@eaDir/c.bookpkg",
		"books/Hidden/d.bookpkg",
	} {
		if err := os.MkdirAll(filepath.Join(dir, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "books", "e.bookpkg"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, shelfConfigFile), []byte(`{"scan":{"ignored_dirs":[{"name":"@eaDir"},{"name":"Hidden"}]}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	n, err := CountBookPackages(rootFS(t, dir))
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Errorf("count = %d, want 2", n)
	}
}

func TestCountBookPackagesWithoutBooksIsNotAShelf(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, booksFolder), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := CountBookPackages(rootFS(t, dir)); !errors.Is(err, ErrNotAShelf) {
		t.Errorf("books/ as a file: err = %v, want ErrNotAShelf", err)
	}
	if _, err := CountBookPackages(rootFS(t, t.TempDir())); !errors.Is(err, ErrNotAShelf) {
		t.Errorf("empty folder: err = %v, want ErrNotAShelf", err)
	}
}

func TestShelfRootNames(t *testing.T) {
	if !IsShelfRoot([]string{"app", "books"}) || IsShelfRoot([]string{"Books", "trash"}) {
		t.Error("IsShelfRoot keys on a books/ subdirectory")
	}
	if !IsBookPackage("dune.bookpkg") || IsBookPackage("Fiction") {
		t.Error("IsBookPackage keys on the .bookpkg suffix")
	}
}
