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

func TestLoadDesktopSelectedBookFiles(t *testing.T) {
	tmpDir := t.TempDir()
	bookPath := filepath.Join(tmpDir, "book.txt")
	expectedContent := []byte("hello world")
	if err := os.WriteFile(bookPath, expectedContent, 0o600); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	files, err := loadDesktopSelectedBookFiles([]string{"", bookPath})
	if err != nil {
		t.Fatalf("loadDesktopSelectedBookFiles returned error: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected one loaded file, got %d", len(files))
	}

	file := files[0]
	if file.Path != bookPath {
		t.Fatalf("expected path %q, got %q", bookPath, file.Path)
	}
	if file.Name != "book.txt" {
		t.Fatalf("expected name %q, got %q", "book.txt", file.Name)
	}
	if string(file.Content) != string(expectedContent) {
		t.Fatalf("expected content %q, got %q", string(expectedContent), string(file.Content))
	}
}
