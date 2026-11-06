package main

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/voilelab/plainshelf/shelf"
)

func writeDropFile(t *testing.T, dir, name string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("hello"), 0o600); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	return path
}

func TestSplitDroppedFilesByExtension(t *testing.T) {
	dir := t.TempDir()
	txt := writeDropFile(t, dir, "a.txt")
	md := writeDropFile(t, dir, "b.MD")
	epub := writeDropFile(t, dir, "c.epub")
	pdf := writeDropFile(t, dir, "d.pdf")
	folder := filepath.Join(dir, "folder.epub")
	if err := os.Mkdir(folder, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	missing := filepath.Join(dir, "gone.txt")

	drop := splitDroppedFiles([]string{txt, " ", md, pdf, folder, epub, missing})

	if want := []string{txt, md, epub}; !slices.Equal(drop.Paths, want) {
		t.Errorf("Paths = %v, want %v", drop.Paths, want)
	}
	if want := []string{pdf, folder, missing}; !slices.Equal(drop.Unsupported, want) {
		t.Errorf("Unsupported = %v, want %v", drop.Unsupported, want)
	}
}

func TestFilesDroppedEmitsSplit(t *testing.T) {
	dir := t.TempDir()
	txt := writeDropFile(t, dir, "a.txt")
	pdf := writeDropFile(t, dir, "b.pdf")
	native := &fakeNative{}
	app := NewDesktopApp(native)

	app.filesDropped(nil)
	app.filesDropped([]string{txt, pdf})

	if len(native.events) != 1 {
		t.Fatalf("emitted %d events, want 1 (an empty drop emits nothing)", len(native.events))
	}
	got := native.events[0]
	if got.name != filesDroppedEvent {
		t.Errorf("event name = %q, want %q", got.name, filesDroppedEvent)
	}
	drop, ok := got.data.(droppedBookFiles)
	if !ok {
		t.Fatalf("event data = %T, want droppedBookFiles", got.data)
	}
	if !slices.Equal(drop.Paths, []string{txt}) || !slices.Equal(drop.Unsupported, []string{pdf}) {
		t.Errorf("drop = %+v", drop)
	}
}

func TestImportLocalPathReadOnlyShelf(t *testing.T) {
	const shelfID = "read_only_shelf"
	desktopApp := startedDesktopAppWithConfs(t, []*shelf.ShelfConfWithID{
		{ID: shelfID, LibRoot: t.TempDir(), ReadOnly: true},
	})
	src := writeDropFile(t, t.TempDir(), "book.txt")

	result, err := desktopApp.ImportBookFromLocalPath(shelfID, src, nil)
	if err != nil {
		t.Fatalf("ImportBookFromLocalPath: %v", err)
	}
	if result.ID != "" || result.Error == "" {
		t.Fatalf("result = %+v, want a read-only error and no book", result)
	}

	shelfData, _ := desktopApp.app.ShelfManager().GetShelf(shelfID)
	books, err := shelfData.ListBooks()
	if err != nil {
		t.Fatalf("ListBooks: %v", err)
	}
	if len(books) != 0 {
		t.Fatalf("read-only shelf holds %d books after import, want 0", len(books))
	}
}
