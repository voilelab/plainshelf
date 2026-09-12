package fsutil

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadTextFileMissingIsEmpty(t *testing.T) {
	got, err := ReadTextFile(filepath.Join(t.TempDir(), "absent.json"))
	if err != nil {
		t.Fatalf("ReadTextFile: %v", err)
	}
	if got != "" {
		t.Fatalf("ReadTextFile of a missing file = %q, want empty", got)
	}
}

func TestWriteTextFileAtomicCreatesParentDir(t *testing.T) {
	// The desktop app used to skip this and relied on startup having made the
	// data root first; a write into a fresh directory must now succeed on its own.
	path := filepath.Join(t.TempDir(), "nested", "deeper", "doc.json")
	if err := WriteTextFileAtomic(path, ".doc-*.json", `{"v":1}`); err != nil {
		t.Fatalf("WriteTextFileAtomic: %v", err)
	}

	got, err := ReadTextFile(path)
	if err != nil {
		t.Fatalf("ReadTextFile: %v", err)
	}
	if got != `{"v":1}` {
		t.Fatalf("stored document = %q, want %q", got, `{"v":1}`)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("stored document mode = %04o, want 0600", perm)
	}
}

func TestWriteTextFileAtomicReplacesAndLeavesNoTemp(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "doc.json")
	if err := WriteTextFileAtomic(path, ".doc-*.json", "first"); err != nil {
		t.Fatalf("first write: %v", err)
	}
	if err := WriteTextFileAtomic(path, ".doc-*.json", "second"); err != nil {
		t.Fatalf("second write: %v", err)
	}

	got, err := ReadTextFile(path)
	if err != nil {
		t.Fatalf("ReadTextFile: %v", err)
	}
	if got != "second" {
		t.Fatalf("stored document = %q, want %q", got, "second")
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 1 || entries[0].Name() != "doc.json" {
		t.Fatalf("directory holds %v, want only doc.json", entries)
	}
}
