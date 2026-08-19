package shelf

import (
	"os"
	"path"
	"slices"
	"strings"
	"testing"
)

// requireSymlinks skips the test on platforms (or filesystems) where the test
// process cannot create symlinks, such as an unprivileged Windows account.
func requireSymlinks(t *testing.T) {
	t.Helper()

	dir := t.TempDir()
	if err := os.Symlink("target", path.Join(dir, "link")); err != nil {
		t.Skipf("symlinks unsupported here: %v", err)
	}
}

// The shelf walk decides what to descend into from the ReadDir entry instead of
// stat'ing every path, which halves the syscalls of a full scan. A symlink is
// the case that cannot be answered that way: its readdir type byte describes
// the link, not the directory behind it, so a symlinked layer and a symlinked
// book package must still be walked rather than silently skipped.
func TestIterateBooksFollowsSymlinks(t *testing.T) {
	requireSymlinks(t)

	tmpLib := path.Join(t.TempDir(), "shelf_test")
	shelf := newTestShelf(t, &ShelfConf{LibRoot: tmpLib})

	book, err := shelf.NewBook(Layers{"real"}, "Alpha")
	if err != nil {
		t.Fatalf("NewBook: %v", err)
	}
	pkgName := path.Base(book.FolderPath())

	booksDir := path.Join(tmpLib, booksFolder)

	// A layer that is a symlink to another layer.
	if err := os.Symlink("real", path.Join(booksDir, "linked-layer")); err != nil {
		t.Fatalf("symlink layer: %v", err)
	}

	// A book package that is a symlink to a book package in another layer.
	if err := os.Mkdir(path.Join(booksDir, "aliases"), 0755); err != nil {
		t.Fatalf("mkdir aliases: %v", err)
	}
	if err := os.Symlink(path.Join("..", "real", pkgName), path.Join(booksDir, "aliases", pkgName)); err != nil {
		t.Fatalf("symlink book package: %v", err)
	}

	var visited []string
	if err := shelf.iterateBooks(nil, func(b *Book) bool {
		visited = append(visited, b.FolderPath())
		return true
	}); err != nil {
		t.Fatalf("iterateBooks: %v", err)
	}

	for _, want := range []string{
		path.Join(booksFolder, "real", pkgName),
		path.Join(booksFolder, "linked-layer", pkgName),
		path.Join(booksFolder, "aliases", pkgName),
	} {
		if !slices.Contains(visited, want) {
			t.Errorf("book %q was skipped by the scan; visited %v", want, visited)
		}
	}
}

func TestIterateLayersFollowsSymlinkedLayer(t *testing.T) {
	requireSymlinks(t)

	tmpLib := path.Join(t.TempDir(), "shelf_test")
	shelf := newTestShelf(t, &ShelfConf{LibRoot: tmpLib})

	if _, err := shelf.NewBook(Layers{"real", "nested"}, "Alpha"); err != nil {
		t.Fatalf("NewBook: %v", err)
	}

	booksDir := path.Join(tmpLib, booksFolder)
	if err := os.Symlink("real", path.Join(booksDir, "linked-layer")); err != nil {
		t.Fatalf("symlink layer: %v", err)
	}

	var visited []string
	if err := shelf.iterateLayers(func(ls Layers) bool {
		visited = append(visited, strings.Join(ls, "/"))
		return true
	}); err != nil {
		t.Fatalf("iterateLayers: %v", err)
	}

	for _, want := range []string{"real", "real/nested", "linked-layer", "linked-layer/nested"} {
		if !slices.Contains(visited, want) {
			t.Errorf("layer %q was skipped by the scan; visited %v", want, visited)
		}
	}
}

// A plain file must not be descended into, and a file that merely carries the
// book extension is not a book package.
func TestIterateBooksIgnoresFiles(t *testing.T) {
	tmpLib := path.Join(t.TempDir(), "shelf_test")
	shelf := newTestShelf(t, &ShelfConf{LibRoot: tmpLib})

	if _, err := shelf.NewBook(Layers{"real"}, "Alpha"); err != nil {
		t.Fatalf("NewBook: %v", err)
	}

	booksDir := path.Join(tmpLib, booksFolder)
	if err := os.WriteFile(path.Join(booksDir, "notes.txt"), []byte("hi"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if err := os.WriteFile(path.Join(booksDir, "fake"+bookExtension), []byte("hi"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	count := 0
	if err := shelf.iterateBooks(nil, func(b *Book) bool {
		count++
		return true
	}); err != nil {
		t.Fatalf("iterateBooks: %v", err)
	}

	if count != 1 {
		t.Errorf("iterateBooks visited %d books, want 1", count)
	}
}
