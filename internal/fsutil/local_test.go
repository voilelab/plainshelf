package fsutil

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func TestFSWalkLocal(t *testing.T) {
	ffs := NewLocalFS("test_dir")

	getPaths := []string{}

	err := fs.WalkDir(ffs, ".", func(path string, d fs.DirEntry, err error) error {
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

func TestWriteFileLocalCreateAndTruncate(t *testing.T) {
	root := t.TempDir()
	ffs := NewLocalFS(root)

	const fileName = "local_write_file.txt"

	if err := ffs.WriteFile(fileName, []byte("first content")); err != nil {
		t.Fatalf("WriteFile first write failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(root, fileName))
	if err != nil {
		t.Fatalf("ReadFile first write failed: %v", err)
	}
	if string(data) != "first content" {
		t.Fatalf("expected first content, got %q", string(data))
	}

	if err := ffs.WriteFile(fileName, []byte("x")); err != nil {
		t.Fatalf("WriteFile second write failed: %v", err)
	}

	data, err = os.ReadFile(filepath.Join(root, fileName))
	if err != nil {
		t.Fatalf("ReadFile second write failed: %v", err)
	}
	if string(data) != "x" {
		t.Fatalf("expected truncated content %q, got %q", "x", string(data))
	}
}
