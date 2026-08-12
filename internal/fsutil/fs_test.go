package fsutil

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

// LocalFS and RootFS are two implementations of the same interface, so they are
// held to one set of expectations rather than two copies of the same test.
func implementations(t *testing.T) map[string]func(root string) FS {
	t.Helper()

	return map[string]func(root string) FS{
		"LocalFS": func(root string) FS {
			return NewLocalFS(root)
		},
		"RootFS": func(root string) FS {
			rt, err := os.OpenRoot(root)
			if err != nil {
				t.Fatalf("Failed to open root: %v", err)
			}
			t.Cleanup(func() { rt.Close() })
			return NewRootFS(rt)
		},
	}
}

func TestFSWalk(t *testing.T) {
	expectedPaths := []string{
		".",
		"a",
		"b",
		"b/c",
	}

	for name, newFS := range implementations(t) {
		t.Run(name, func(t *testing.T) {
			ffs := newFS("test_dir")

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

			if len(getPaths) != len(expectedPaths) {
				t.Fatalf("Expected %d paths, got %d", len(expectedPaths), len(getPaths))
			}

			for i, expected := range expectedPaths {
				if getPaths[i] != expected {
					t.Errorf("Expected path %q, got %q", expected, getPaths[i])
				}
			}
		})
	}
}

func TestWriteFileCreateAndTruncate(t *testing.T) {
	for name, newFS := range implementations(t) {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			ffs := newFS(root)

			const fileName = "write_file.txt"

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

			// A shorter second write must not leave a tail of the first behind.
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
		})
	}
}
