package fsutil

import (
	"io"
	"io/fs"
	"strings"
	"testing"

	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/test"
)

func TestFSWalkFyneURI(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()

	fyneURI, err := storage.ParseURI("file://test_dir")
	if err != nil {
		t.Fatalf("Failed to parse URI: %v", err)
	}

	listableURI, err := storage.ListerForURI(fyneURI)
	if err != nil {
		t.Fatalf("Failed to create ListableURI: %v", err)
	}

	ffs := NewFyneURIFS(listableURI)

	getPaths := []string{}

	err = fs.WalkDir(ffs, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		getPaths = append(getPaths, path)
		return nil
	})
	if err != nil {
		t.Fatalf("WalkDir failed: %v", err)
	}

	expectedPaths := []string{
		".",
		"a",
		"b",
		"b/c",
	}

	if len(getPaths) != len(expectedPaths) {
		t.Fatalf("Expected %d paths, got %d", len(expectedPaths), len(getPaths))
	}

	for i, expected := range expectedPaths {
		if getPaths[i] != expected {
			t.Errorf("Expected path %q, got %q", expected, getPaths[i])
		}
	}
}

func TestFyneURIFSOpenSupportsReadAll(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()

	fyneURI, err := storage.ParseURI("file://test_dir")
	if err != nil {
		t.Fatalf("Failed to parse URI: %v", err)
	}

	listableURI, err := storage.ListerForURI(fyneURI)
	if err != nil {
		t.Fatalf("Failed to create ListableURI: %v", err)
	}

	ffs := NewFyneURIFS(listableURI)

	file, err := ffs.Open("a")
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}

	if strings.TrimSpace(string(data)) != "a" {
		t.Fatalf("Expected file contents %q, got %q", "a", string(data))
	}
}
