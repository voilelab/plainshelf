package main

import (
	"os"
	"testing"

	"github.com/voilelab/plainshelf/server"
	"github.com/voilelab/plainshelf/shelf"
)

func TestBookOpenDialogOptions(t *testing.T) {
	options := bookOpenDialogOptions()
	if len(options.Filters) != 1 {
		t.Fatalf("expected exactly one file filter, got %d", len(options.Filters))
	}

	filter := options.Filters[0]
	if filter.Pattern != "*.txt" {
		t.Fatalf("expected txt-only filter pattern, got %q", filter.Pattern)
	}
}

func TestNormalizeSelectedLocalPaths(t *testing.T) {
	paths := normalizeSelectedLocalPaths([]string{"", "  ", " /tmp/book-1.txt ", "/tmp/book-2.txt"})
	if len(paths) != 2 {
		t.Fatalf("expected two valid paths, got %d", len(paths))
	}
	if paths[0] != "/tmp/book-1.txt" {
		t.Fatalf("unexpected first path: %q", paths[0])
	}
	if paths[1] != "/tmp/book-2.txt" {
		t.Fatalf("unexpected second path: %q", paths[1])
	}
}

func TestNormalizeLayerParts(t *testing.T) {
	parts := normalizeLayerParts([]string{"", "  ", " fiction ", " sci-fi "})
	if len(parts) != 2 {
		t.Fatalf("expected two valid layer parts, got %d", len(parts))
	}
	if parts[0] != "fiction" {
		t.Fatalf("unexpected first part: %q", parts[0])
	}
	if parts[1] != "sci-fi" {
		t.Fatalf("unexpected second part: %q", parts[1])
	}
}

func TestShelfDirectoryDialogOptions(t *testing.T) {
	options := shelfDirectoryDialogOptions()
	if options.Title == "" {
		t.Fatal("expected shelf directory dialog title")
	}
}

func TestGenerateDesktopShelfID(t *testing.T) {
	existing := map[string]struct{}{"books": {}, "books-2": {}}
	got := generateDesktopShelfID("Books!", "/tmp/fallback", existing)
	if got != "books-3" {
		t.Fatalf("generateDesktopShelfID = %q, want books-3", got)
	}

	got = generateDesktopShelfID("繁體書架", "/tmp/My Library", map[string]struct{}{})
	if got != "my-library" {
		t.Fatalf("generateDesktopShelfID fallback = %q, want my-library", got)
	}
}

func TestNormalizeDesktopShelfDirectory(t *testing.T) {
	dir := t.TempDir()
	got, err := normalizeDesktopShelfDirectory("  " + dir + "  ")
	if err != nil {
		t.Fatalf("normalizeDesktopShelfDirectory: %v", err)
	}
	if got != dir {
		t.Fatalf("normalizeDesktopShelfDirectory = %q, want %q", got, dir)
	}

	file := dir + "/book.txt"
	if err := os.WriteFile(file, []byte("book"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if _, err := normalizeDesktopShelfDirectory(file); err == nil {
		t.Fatal("expected file path to be rejected")
	}
}

func TestSaveAndLoadDesktopShelves(t *testing.T) {
	dataRoot := t.TempDir()
	libRoot := t.TempDir()
	configPath := desktopShelfConfigPath(dataRoot)

	input := []*server.ShelfConfWithID{
		{
			ID:   "books",
			Name: "Books",
			ShelfConf: shelf.ShelfConf{
				LibRoot: libRoot,
			},
		},
	}
	if err := saveDesktopShelves(configPath, input); err != nil {
		t.Fatalf("saveDesktopShelves: %v", err)
	}

	loaded, err := loadDesktopShelves(dataRoot)
	if err != nil {
		t.Fatalf("loadDesktopShelves: %v", err)
	}
	if len(loaded) != 1 {
		t.Fatalf("expected one loaded shelf, got %d", len(loaded))
	}
	if loaded[0].ID != "books" || loaded[0].Name != "Books" || loaded[0].LibRoot != libRoot {
		t.Fatalf("unexpected loaded shelf: %#v", loaded[0])
	}
}

func TestDesktopAppAddShelfPersistsAndRejectsDuplicateDirectory(t *testing.T) {
	dataRoot := t.TempDir()
	shelfConfs := defaultDesktopShelves(dataRoot)
	app, err := server.NewApp(&server.AppConf{
		Shelves:          shelfConfs,
		StorePath:        t.TempDir(),
		ReadHistoryLimit: 10,
		Security: &server.SecurityConf{
			Mode: server.SecurityModeNone,
		},
	})
	if err != nil {
		t.Fatalf("NewApp: %v", err)
	}
	defer app.Close()

	desktopApp := &DesktopApp{
		app:             app,
		dataRoot:        dataRoot,
		shelfConfigPath: desktopShelfConfigPath(dataRoot),
		shelfConfs:      shelfConfs,
	}
	libRoot := t.TempDir()

	created, err := desktopApp.AddShelf("My Books", libRoot)
	if err != nil {
		t.Fatalf("AddShelf: %v", err)
	}
	if created.ID != "my-books" || created.Name != "My Books" || created.LibRoot != libRoot {
		t.Fatalf("unexpected created shelf: %#v", created)
	}
	if _, ok := app.GetShelf("my-books"); !ok {
		t.Fatal("new shelf was not registered in the running server app")
	}

	loaded, err := loadDesktopShelves(dataRoot)
	if err != nil {
		t.Fatalf("loadDesktopShelves: %v", err)
	}
	if len(loaded) != 2 {
		t.Fatalf("expected default and added shelves to be persisted, got %d", len(loaded))
	}

	if _, err := desktopApp.AddShelf("Again", libRoot); err == nil {
		t.Fatal("expected duplicate shelf directory to be rejected")
	}
}
