package appcache

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/voilelab/plainshelf/internal/fsutil"
)

const testDir = ".app"

func newTestStore(t *testing.T) (*FSStore, string) {
	t.Helper()
	base := t.TempDir()
	if err := os.Mkdir(filepath.Join(base, testDir), 0o755); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(base)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = root.Close() })
	return NewFSStore(fsutil.NewRootFS(root), testDir), base
}

func TestFSStoreWriteThenRead(t *testing.T) {
	store, base := newTestStore(t)

	if err := store.WriteFileAtomic("cache.json", []byte("v1")); err != nil {
		t.Fatal(err)
	}
	got, err := store.ReadFile("cache.json")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "v1" {
		t.Fatalf("ReadFile = %q, want %q", got, "v1")
	}

	onDisk, err := os.ReadFile(filepath.Join(base, testDir, "cache.json"))
	if err != nil || string(onDisk) != "v1" {
		t.Fatalf("file under the store dir = %q, %v; want %q", onDisk, err, "v1")
	}
}

func TestFSStoreOverwriteLeavesNoTemp(t *testing.T) {
	store, base := newTestStore(t)

	for _, data := range []string{"old", "new"} {
		if err := store.WriteFileAtomic("cache.json", []byte(data)); err != nil {
			t.Fatal(err)
		}
	}
	got, err := store.ReadFile("cache.json")
	if err != nil || string(got) != "new" {
		t.Fatalf("ReadFile = %q, %v; want %q", got, err, "new")
	}

	entries, err := os.ReadDir(filepath.Join(base, testDir))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Fatalf("store dir holds %v, want only cache.json", names)
	}
}

func TestFSStoreReadMissing(t *testing.T) {
	store, _ := newTestStore(t)

	if _, err := store.ReadFile("absent.json"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("ReadFile of a missing file = %v, want fs.ErrNotExist", err)
	}
}

func TestFSStoreReadOnlyRoot(t *testing.T) {
	writable, base := newTestStore(t)
	if err := writable.WriteFileAtomic("cache.json", []byte("kept")); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(base)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = root.Close() })
	store := NewFSStore(fsutil.ReadOnly(fsutil.NewRootFS(root)), testDir)

	if err := store.EnsureWritable(); !errors.Is(err, fsutil.ErrReadOnly) {
		t.Fatalf("EnsureWritable = %v, want fsutil.ErrReadOnly", err)
	}
	if err := store.WriteFileAtomic("cache.json", []byte("overwritten")); !errors.Is(err, fsutil.ErrReadOnly) {
		t.Fatalf("WriteFileAtomic = %v, want fsutil.ErrReadOnly", err)
	}
	got, err := store.ReadFile("cache.json")
	if err != nil || string(got) != "kept" {
		t.Fatalf("ReadFile on a read-only root = %q, %v; want %q", got, err, "kept")
	}
}

func TestFSStoreEnsureWritable(t *testing.T) {
	store, _ := newTestStore(t)

	if err := store.EnsureWritable(); err != nil {
		t.Fatalf("EnsureWritable on a writable root = %v, want nil", err)
	}
}
