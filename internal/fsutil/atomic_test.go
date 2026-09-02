package fsutil

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
)

var errInjected = errors.New("injected failure")

// faultFS wraps an FS and lets a test fail or observe individual operations.
type faultFS struct {
	FS

	mu sync.Mutex

	failWriteFile  bool
	failOpenWriter bool
	failRename     bool

	writtenPaths []string
}

func (f *faultFS) WriteFile(name string, data []byte) error {
	f.mu.Lock()
	f.writtenPaths = append(f.writtenPaths, name)
	fail := f.failWriteFile
	f.mu.Unlock()

	if fail {
		return errInjected
	}
	return f.FS.WriteFile(name, data)
}

func (f *faultFS) OpenWriter(name string) (io.WriteCloser, error) {
	f.mu.Lock()
	f.writtenPaths = append(f.writtenPaths, name)
	fail := f.failOpenWriter
	f.mu.Unlock()

	if fail {
		return nil, errInjected
	}
	return f.FS.OpenWriter(name)
}

func (f *faultFS) Rename(oldPath, newPath string) error {
	f.mu.Lock()
	fail := f.failRename
	f.mu.Unlock()

	if fail {
		return errInjected
	}
	return f.FS.Rename(oldPath, newPath)
}

func (f *faultFS) written() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.writtenPaths)
}

// errReader fails partway through so streaming writes can be interrupted.
type errReader struct {
	prefix string
	read   bool
}

func (e *errReader) Read(p []byte) (int, error) {
	if !e.read {
		e.read = true
		n := copy(p, e.prefix)
		return n, nil
	}
	return 0, errInjected
}

// leftoverTempFiles lists temp files that survived in root.
func leftoverTempFiles(t *testing.T, root string) []string {
	t.Helper()

	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("ReadDir(%q): %v", root, err)
	}

	var leftover []string
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".tmp") {
			leftover = append(leftover, entry.Name())
		}
	}
	return leftover
}

func TestTempPathKeepsTmpSuffixAndIsUnique(t *testing.T) {
	const name = "books/book.json"

	first := tempPath(name)
	second := tempPath(name)

	if !strings.HasSuffix(first, ".tmp") {
		// Sync tools are configured to ignore *.tmp; losing the suffix would
		// let them sync half-written files.
		t.Errorf("tempPath(%q) = %q, want a .tmp suffix", name, first)
	}
	if !strings.HasPrefix(first, name+".") {
		t.Errorf("tempPath(%q) = %q, want it to sit next to the destination", name, first)
	}
	if first == second {
		t.Errorf("tempPath(%q) returned the same name twice (%q); concurrent writers would collide", name, first)
	}
}

func TestWriteFileAtomicReplacesExistingFile(t *testing.T) {
	root := t.TempDir()
	ffs := newTestFS(t, root)

	const name = "book.json"

	if err := WriteFileAtomic(ffs, name, []byte("first")); err != nil {
		t.Fatalf("WriteFileAtomic first write: %v", err)
	}
	if err := WriteFileAtomic(ffs, name, []byte("second")); err != nil {
		t.Fatalf("WriteFileAtomic second write: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(root, name))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(data) != "second" {
		t.Errorf("content = %q, want %q", string(data), "second")
	}

	if leftover := leftoverTempFiles(t, root); len(leftover) != 0 {
		t.Errorf("temp files left behind after successful writes: %v", leftover)
	}
}

func TestWriteFileAtomicLeavesDestinationIntactOnWriteFailure(t *testing.T) {
	root := t.TempDir()
	ffs := newTestFS(t, root)

	const name = "book.json"
	if err := ffs.WriteFile(name, []byte("original")); err != nil {
		t.Fatalf("seed WriteFile: %v", err)
	}

	faulty := &faultFS{FS: ffs, failWriteFile: true}
	if err := WriteFileAtomic(faulty, name, []byte("replacement")); !errors.Is(err, errInjected) {
		t.Fatalf("WriteFileAtomic error = %v, want %v", err, errInjected)
	}

	data, err := os.ReadFile(filepath.Join(root, name))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(data) != "original" {
		t.Errorf("content = %q, want the destination to be untouched (%q)", string(data), "original")
	}
}

func TestWriteFileAtomicRemovesTempOnRenameFailure(t *testing.T) {
	root := t.TempDir()
	faulty := &faultFS{FS: newTestFS(t, root), failRename: true}

	if err := WriteFileAtomic(faulty, "book.json", []byte("payload")); !errors.Is(err, errInjected) {
		t.Fatalf("WriteFileAtomic error = %v, want %v", err, errInjected)
	}

	if _, err := os.Stat(filepath.Join(root, "book.json")); !os.IsNotExist(err) {
		t.Errorf("destination should not exist after a failed rename, stat err = %v", err)
	}
	if leftover := leftoverTempFiles(t, root); len(leftover) != 0 {
		t.Errorf("temp files left behind after a failed rename: %v", leftover)
	}
}

func TestWriteAtomicStreamsReader(t *testing.T) {
	root := t.TempDir()
	ffs := newTestFS(t, root)

	const name = "source.txt"
	const content = "line one\nline two\n"

	if err := WriteAtomic(ffs, name, strings.NewReader(content)); err != nil {
		t.Fatalf("WriteAtomic: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(root, name))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(data) != content {
		t.Errorf("content = %q, want %q", string(data), content)
	}

	if leftover := leftoverTempFiles(t, root); len(leftover) != 0 {
		t.Errorf("temp files left behind after a successful stream write: %v", leftover)
	}
}

func TestWriteAtomicLeavesDestinationIntactWhenReaderFails(t *testing.T) {
	root := t.TempDir()
	ffs := newTestFS(t, root)

	const name = "source.txt"
	if err := ffs.WriteFile(name, []byte("original")); err != nil {
		t.Fatalf("seed WriteFile: %v", err)
	}

	err := WriteAtomic(ffs, name, &errReader{prefix: "partial content"})
	if !errors.Is(err, errInjected) {
		t.Fatalf("WriteAtomic error = %v, want %v", err, errInjected)
	}

	data, readErr := os.ReadFile(filepath.Join(root, name))
	if readErr != nil {
		t.Fatalf("ReadFile: %v", readErr)
	}
	if string(data) != "original" {
		t.Errorf("content = %q, want the destination to be untouched (%q)", string(data), "original")
	}
	if leftover := leftoverTempFiles(t, root); len(leftover) != 0 {
		t.Errorf("partial stream left temp files behind: %v", leftover)
	}
}

func TestWriteAtomicRemovesTempWhenOpenWriterFails(t *testing.T) {
	root := t.TempDir()
	faulty := &faultFS{FS: newTestFS(t, root), failOpenWriter: true}

	if err := WriteAtomic(faulty, "source.txt", strings.NewReader("payload")); !errors.Is(err, errInjected) {
		t.Fatalf("WriteAtomic error = %v, want %v", err, errInjected)
	}
	if leftover := leftoverTempFiles(t, root); len(leftover) != 0 {
		t.Errorf("temp files left behind: %v", leftover)
	}
}

// Two writers racing on the same destination must not share a temp file.
// With a fixed "<name>.tmp" they would overwrite each other's staged data.
func TestWriteFileAtomicConcurrentWritersUseDistinctTempFiles(t *testing.T) {
	root := t.TempDir()
	faulty := &faultFS{FS: newTestFS(t, root)}

	const name = "book.json"
	const writers = 16

	var wg sync.WaitGroup
	errs := make([]error, writers)
	for i := range writers {
		wg.Go(func() {
			errs[i] = WriteFileAtomic(faulty, name, []byte(strings.Repeat("x", i+1)))
		})
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Errorf("writer %d: %v", i, err)
		}
	}

	seen := make(map[string]bool, writers)
	for _, p := range faulty.written() {
		if seen[p] {
			t.Fatalf("temp path %q was used by more than one concurrent writer", p)
		}
		seen[p] = true
	}
	if len(seen) != writers {
		t.Errorf("observed %d temp paths, want %d", len(seen), writers)
	}

	if leftover := leftoverTempFiles(t, root); len(leftover) != 0 {
		t.Errorf("temp files left behind after concurrent writes: %v", leftover)
	}
}
