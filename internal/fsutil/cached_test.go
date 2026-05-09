package fsutil

import (
	"errors"
	"io"
	"os"
	"path"
	"path/filepath"
	"testing"
)

func TestCachedFSOpenCacheHit(t *testing.T) {
	mainDir := t.TempDir()
	cacheDir := t.TempDir()

	const name = "books/ch1.txt"

	writeFile(t, filepath.Join(mainDir, name), "from-main")
	writeFile(t, filepath.Join(cacheDir, name), "from-cache")

	cfs := NewCachedFS(NewLocalFS(mainDir), NewLocalFS(cacheDir))

	f, err := cfs.Open(name)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer f.Close()

	b, err := io.ReadAll(f)
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}

	if got := string(b); got != "from-cache" {
		t.Fatalf("unexpected contents, got %q, expected %q", got, "from-cache")
	}
}

func TestCachedFSOpenCacheMissPopulatesCache(t *testing.T) {
	mainDir := t.TempDir()
	cacheDir := t.TempDir()

	const name = "books/ch2.txt"

	writeFile(t, filepath.Join(mainDir, name), "chapter-2")

	cfs := NewCachedFS(NewLocalFS(mainDir), NewLocalFS(cacheDir))

	f, err := cfs.Open(name)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	b, err := io.ReadAll(f)
	_ = f.Close()
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}

	if got := string(b); got != "chapter-2" {
		t.Fatalf("unexpected contents from open, got %q, expected %q", got, "chapter-2")
	}

	cacheBytes, err := os.ReadFile(filepath.Join(cacheDir, name))
	if err != nil {
		t.Fatalf("expected cache file to exist: %v", err)
	}
	if got := string(cacheBytes); got != "chapter-2" {
		t.Fatalf("unexpected cached contents, got %q, expected %q", got, "chapter-2")
	}
}

func TestCachedFSReadDirAndStatFallbackToMain(t *testing.T) {
	mainDir := t.TempDir()
	cacheDir := t.TempDir()

	writeFile(t, filepath.Join(mainDir, "library", "a.txt"), "a")
	writeFile(t, filepath.Join(mainDir, "library", "b.txt"), "b")

	cfs := NewCachedFS(NewLocalFS(mainDir), NewLocalFS(cacheDir))

	entries, err := cfs.ReadDir("library")
	if err != nil {
		t.Fatalf("ReadDir failed: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}

	st, err := cfs.Stat("library/a.txt")
	if err != nil {
		t.Fatalf("Stat failed: %v", err)
	}
	if st.IsDir() {
		t.Fatalf("expected file info, got directory")
	}
}

func TestCachedFSOpenWriterWritesToMainAndCache(t *testing.T) {
	mainDir := t.TempDir()
	cacheDir := t.TempDir()

	const name = "drafts/2026/chapter.txt"

	cfs := NewCachedFS(NewLocalFS(mainDir), NewLocalFS(cacheDir))
	if err := cfs.MkdirAll(path.Dir(name)); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}

	w, err := cfs.OpenWriter(name)
	if err != nil {
		t.Fatalf("OpenWriter failed: %v", err)
	}

	if _, err := w.Write([]byte("hello-cached-fs")); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	if err := w.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	mainBytes, err := os.ReadFile(filepath.Join(mainDir, name))
	if err != nil {
		t.Fatalf("reading main file failed: %v", err)
	}
	if got := string(mainBytes); got != "hello-cached-fs" {
		t.Fatalf("unexpected main contents, got %q", got)
	}

	cacheBytes, err := os.ReadFile(filepath.Join(cacheDir, name))
	if err != nil {
		t.Fatalf("reading cache file failed: %v", err)
	}
	if got := string(cacheBytes); got != "hello-cached-fs" {
		t.Fatalf("unexpected cache contents, got %q", got)
	}
}

func TestCachedFSOpenWriterFallsBackWhenCacheWriterFails(t *testing.T) {
	mainDir := t.TempDir()
	cacheDir := t.TempDir()

	const name = "drafts/fallback.txt"

	mainFS := NewLocalFS(mainDir)
	cacheFS := &openWriterErrorFS{FS: NewLocalFS(cacheDir)}
	cfs := NewCachedFS(mainFS, cacheFS)
	if err := cfs.MkdirAll(path.Dir(name)); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}

	w, err := cfs.OpenWriter(name)
	if err != nil {
		t.Fatalf("OpenWriter failed: %v", err)
	}

	if _, err := w.Write([]byte("main-only")); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	if err := w.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	mainBytes, err := os.ReadFile(filepath.Join(mainDir, name))
	if err != nil {
		t.Fatalf("reading main file failed: %v", err)
	}
	if got := string(mainBytes); got != "main-only" {
		t.Fatalf("unexpected main contents, got %q", got)
	}

	if _, err := os.Stat(filepath.Join(cacheDir, name)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected cache file not to exist, got err=%v", err)
	}
}

type openWriterErrorFS struct {
	FS
}

func (f *openWriterErrorFS) OpenWriter(name string) (io.WriteCloser, error) {
	return nil, errors.New("forced OpenWriter failure")
}
