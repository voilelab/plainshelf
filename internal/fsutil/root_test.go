package fsutil

import (
	"io/fs"
	"os"
	"testing"
)

func TestFSWalkRoot(t *testing.T) {
	rt, err := os.OpenRoot("test_dir")
	if err != nil {
		t.Fatalf("Failed to open root: %v", err)
	}
	defer rt.Close()

	ffs := NewRootFS(rt)

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
