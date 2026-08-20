package fsutil

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

// newTestFS opens the FS implementation under test on root.
func newTestFS(t *testing.T, root string) FS {
	t.Helper()

	rt, err := os.OpenRoot(root)
	if err != nil {
		t.Fatalf("Failed to open root: %v", err)
	}
	t.Cleanup(func() { rt.Close() })
	return NewRootFS(rt)
}

func TestFSWalk(t *testing.T) {
	expectedPaths := []string{
		".",
		"a",
		"b",
		"b/c",
	}

	ffs := newTestFS(t, "test_dir")

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
}

func TestWriteFileCreateAndTruncate(t *testing.T) {
	root := t.TempDir()
	ffs := newTestFS(t, root)

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
}

// Writable hands back a handle that can write when the implementation supports
// it, and refuses one that only reads. It is the single narrowing point every
// caller holding a ReadFS goes through, so a wrong answer here would either
// panic a read-only build or block a writable one.
func TestWritableNarrowsOnlyWritableImplementations(t *testing.T) {
	var writable ReadFS = newTestFS(t, t.TempDir())

	if _, err := Writable(writable); err != nil {
		t.Errorf("Writable(RootFS) error = %v, want nil", err)
	}

	if _, err := Writable(readOnlyStub{}); !errors.Is(err, ErrReadOnly) {
		t.Errorf("Writable(readOnlyStub) error = %v, want %v", err, ErrReadOnly)
	}
}

// ReadOnly is what a caller reaches for when the implementation it holds can
// write but this handle must not: the wrapper has to keep reading and has to
// stop Writable from handing the writes straight back.
func TestReadOnlyKeepsReadsAndRefusesNarrowing(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "book.json"), []byte("{}"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	readOnly := ReadOnly(newTestFS(t, root))

	if _, err := readOnly.Stat("book.json"); err != nil {
		t.Errorf("Stat through a read-only handle: %v", err)
	}
	if _, err := readOnly.ReadDir("."); err != nil {
		t.Errorf("ReadDir through a read-only handle: %v", err)
	}
	file, err := readOnly.Open("book.json")
	if err != nil {
		t.Errorf("Open through a read-only handle: %v", err)
	} else {
		file.Close()
	}

	if _, err := Writable(readOnly); !errors.Is(err, ErrReadOnly) {
		t.Errorf("Writable(ReadOnly(RootFS)) error = %v, want %v", err, ErrReadOnly)
	}
}

// readOnlyStub implements ReadFS and nothing more, standing in for a backend
// that has no write half at all.
type readOnlyStub struct{}

func (readOnlyStub) Open(string) (fs.File, error)          { return nil, fs.ErrNotExist }
func (readOnlyStub) ReadDir(string) ([]fs.DirEntry, error) { return nil, fs.ErrNotExist }
func (readOnlyStub) Stat(string) (fs.FileInfo, error)      { return nil, fs.ErrNotExist }
