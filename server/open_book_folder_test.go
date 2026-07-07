package server

import (
	"strings"
	"testing"
)

func TestOpenBookFolder(t *testing.T) {
	env := newAPITestEnv(t)
	created := importTextBook(t, env, "Open Finder Target", "/docs", "finder-target.txt", "hello")

	openedPath := ""
	restore := openFinder
	openFinder = func(path string) error {
		openedPath = path
		return nil
	}
	defer func() {
		openFinder = restore
	}()

	if err := env.app.OpenBookFolder("default_shelf", created.Meta.ID); err != nil {
		t.Fatalf("OpenBookFolder: %v", err)
	}

	if openedPath == "" {
		t.Fatal("expected OpenBookFolder to call openFinder with a path")
	}
	if !strings.Contains(openedPath, "/docs/") {
		t.Fatalf("openFinder path %q does not contain expected layer segment", openedPath)
	}
}

func TestOpenBookFolderValidation(t *testing.T) {
	env := newAPITestEnv(t)

	if err := env.app.OpenBookFolder("", "book-id"); err == nil {
		t.Fatal("expected error for empty shelf ID")
	}
	if err := env.app.OpenBookFolder("default_shelf", ""); err == nil {
		t.Fatal("expected error for empty book ID")
	}
	if err := env.app.OpenBookFolder("missing_shelf", "book-id"); err == nil {
		t.Fatal("expected error for missing shelf")
	}
	if err := env.app.OpenBookFolder("default_shelf", "missing_book"); err == nil {
		t.Fatal("expected error for missing book")
	}
}
