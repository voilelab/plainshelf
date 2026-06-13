package main

import (
	"os"
	"path/filepath"
	"testing"
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

func TestSaveAndLoadDesktopShelves(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "shelves.json")

	// Loading a missing config returns empty config without error.
	conf, err := loadDesktopShelves(configPath)
	if err != nil {
		t.Fatalf("loadDesktopShelves on missing file: %v", err)
	}
	if len(conf.Shelves) != 0 {
		t.Fatalf("expected empty shelves, got %d", len(conf.Shelves))
	}

	conf.Shelves = []desktopShelfEntry{
		{ID: "my-books", Name: "My Books", LibRoot: "/home/user/books"},
		{ID: "work", Name: "Work", LibRoot: "/home/user/work"},
	}
	if err := saveDesktopShelves(configPath, conf); err != nil {
		t.Fatalf("saveDesktopShelves: %v", err)
	}

	loaded, err := loadDesktopShelves(configPath)
	if err != nil {
		t.Fatalf("loadDesktopShelves after save: %v", err)
	}
	if len(loaded.Shelves) != 2 {
		t.Fatalf("expected 2 shelves, got %d", len(loaded.Shelves))
	}
	if loaded.Shelves[0].ID != "my-books" || loaded.Shelves[0].Name != "My Books" || loaded.Shelves[0].LibRoot != "/home/user/books" {
		t.Fatalf("unexpected first shelf: %+v", loaded.Shelves[0])
	}
	if loaded.Shelves[1].ID != "work" {
		t.Fatalf("unexpected second shelf ID: %q", loaded.Shelves[1].ID)
	}

	// Verify file permissions are restricted.
	info, err := os.Stat(configPath)
	if err != nil {
		t.Fatalf("stat config file: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("config file permissions = %o, want 0600", perm)
	}
}

func TestNormalizeDesktopShelfDirectory(t *testing.T) {
	cases := []struct {
		input   string
		want    string
		wantErr bool
	}{
		{input: "  /home/user/books  ", want: "/home/user/books"},
		{input: "/abs/path", want: "/abs/path"},
		{input: "", wantErr: true},
		{input: "  ", wantErr: true},
		{input: "relative/path", wantErr: true},
	}
	for _, tc := range cases {
		got, err := normalizeDesktopShelfDirectory(tc.input)
		if tc.wantErr {
			if err == nil {
				t.Errorf("normalizeDesktopShelfDirectory(%q): want error, got nil", tc.input)
			}
			continue
		}
		if err != nil {
			t.Errorf("normalizeDesktopShelfDirectory(%q): unexpected error: %v", tc.input, err)
			continue
		}
		if got != tc.want {
			t.Errorf("normalizeDesktopShelfDirectory(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestGenerateDesktopShelfID(t *testing.T) {
	cases := []struct {
		name        string
		existingIDs map[string]bool
		want        string
	}{
		{name: "My Books", existingIDs: map[string]bool{}, want: "my-books"},
		{name: "  Hello World  ", existingIDs: map[string]bool{}, want: "hello-world"},
		{name: "My Books", existingIDs: map[string]bool{"my-books": true}, want: "my-books-2"},
		{name: "My Books", existingIDs: map[string]bool{"my-books": true, "my-books-2": true}, want: "my-books-3"},
		{name: "!!!###", existingIDs: map[string]bool{}, want: "shelf"},
		{name: "shelf", existingIDs: map[string]bool{"shelf": true}, want: "shelf-2"},
	}
	for _, tc := range cases {
		got := generateDesktopShelfID(tc.name, tc.existingIDs)
		if got != tc.want {
			t.Errorf("generateDesktopShelfID(%q, %v) = %q, want %q", tc.name, tc.existingIDs, got, tc.want)
		}
	}
}
