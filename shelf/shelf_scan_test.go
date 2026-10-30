package shelf

import (
	"os"
	"path"
	"slices"
	"strings"
	"testing"
)

func requireSymlinks(t *testing.T) {
	t.Helper()

	dir := t.TempDir()
	if err := os.Symlink("target", path.Join(dir, "link")); err != nil {
		t.Skipf("symlinks unsupported here: %v", err)
	}
}

func TestIterateShelfTreeFollowsSymlinks(t *testing.T) {
	requireSymlinks(t)

	tmpLib := path.Join(t.TempDir(), "shelf_test")
	shelf := newTestShelf(t, &ShelfConf{LibRoot: tmpLib})

	book, err := shelf.NewBook(FolderPath{"real"}, "Alpha")
	if err != nil {
		t.Fatalf("NewBook: %v", err)
	}
	pkgName := path.Base(book.PackagePath())

	booksDir := path.Join(tmpLib, booksFolder)

	if err := os.Symlink("real", path.Join(booksDir, "linked-layer")); err != nil {
		t.Fatalf("symlink layer: %v", err)
	}

	if err := os.Mkdir(path.Join(booksDir, "aliases"), 0755); err != nil {
		t.Fatalf("mkdir aliases: %v", err)
	}
	if err := os.Symlink(path.Join("..", "real", pkgName), path.Join(booksDir, "aliases", pkgName)); err != nil {
		t.Fatalf("symlink book package: %v", err)
	}

	var visited []string
	if _, err := shelf.iterateShelfTree(nil, nil, func(b *Book) bool {
		visited = append(visited, b.PackagePath())
		return true
	}); err != nil {
		t.Fatalf("iterateShelfTree: %v", err)
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

func TestShelfTreeFollowsSymlinkedFolder(t *testing.T) {
	requireSymlinks(t)

	tmpLib := path.Join(t.TempDir(), "shelf_test")
	shelf := newTestShelf(t, &ShelfConf{LibRoot: tmpLib})

	if _, err := shelf.NewBook(FolderPath{"real", "nested"}, "Alpha"); err != nil {
		t.Fatalf("NewBook: %v", err)
	}

	booksDir := path.Join(tmpLib, booksFolder)
	if err := os.Symlink("real", path.Join(booksDir, "linked-layer")); err != nil {
		t.Fatalf("symlink layer: %v", err)
	}

	var visited []string
	if _, err := shelf.iterateShelfTree(func(ls FolderPath) bool {
		visited = append(visited, strings.Join(ls, "/"))
		return true
	}, nil, nil); err != nil {
		t.Fatalf("iterateShelfTree: %v", err)
	}

	for _, want := range []string{"real", "real/nested", "linked-layer", "linked-layer/nested"} {
		if !slices.Contains(visited, want) {
			t.Errorf("layer %q was skipped by the scan; visited %v", want, visited)
		}
	}
}

func TestIterateShelfTreeIgnoresFiles(t *testing.T) {
	tmpLib := path.Join(t.TempDir(), "shelf_test")
	shelf := newTestShelf(t, &ShelfConf{LibRoot: tmpLib})

	if _, err := shelf.NewBook(FolderPath{"real"}, "Alpha"); err != nil {
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
	if _, err := shelf.iterateShelfTree(nil, nil, func(b *Book) bool {
		count++
		return true
	}); err != nil {
		t.Fatalf("iterateShelfTree: %v", err)
	}

	if count != 1 {
		t.Errorf("iterateShelfTree visited %d books, want 1", count)
	}
}
