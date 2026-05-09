package fsutil

import (
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"golang.org/x/net/webdav"
)

func TestFSWalkWebDAV(t *testing.T) {
	tmpDir := t.TempDir()
	writeFile(t, filepath.Join(tmpDir, "library", "a"), "a")
	writeFile(t, filepath.Join(tmpDir, "library", "b", "c"), "c")

	const user = "alice"
	const pass = "secret"

	srv := newMockWebDAVServer(t, tmpDir, user, pass)
	t.Cleanup(srv.Close)

	ffs, err := NewWebDAVFS(&WebDAVConf{
		Host:     srv.URL,
		User:     user,
		Password: pass,
		BaseDir:  "/library",
	})
	if err != nil {
		t.Fatalf("NewWebDAVFS failed: %v", err)
	}

	gotPaths := []string{}
	err = fs.WalkDir(ffs, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		gotPaths = append(gotPaths, path)
		return nil
	})
	if err != nil {
		t.Fatalf("WalkDir failed: %v", err)
	}

	expectedPaths := []string{".", "a", "b", "b/c"}
	if !reflect.DeepEqual(gotPaths, expectedPaths) {
		t.Fatalf("unexpected walk paths, got %v, expected %v", gotPaths, expectedPaths)
	}

	f, err := ffs.Open("a")
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer f.Close()

	data, err := io.ReadAll(f)
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}
	if strings.TrimSpace(string(data)) != "a" {
		t.Fatalf("unexpected file contents: %q", string(data))
	}
}

func TestWebDAVFSOperations(t *testing.T) {
	tmpDir := t.TempDir()

	const user = "alice"
	const pass = "secret"

	srv := newMockWebDAVServer(t, tmpDir, user, pass)
	t.Cleanup(srv.Close)

	ffs, err := NewWebDAVFS(&WebDAVConf{
		Host:     srv.URL,
		User:     user,
		Password: pass,
		BaseDir:  "/library",
	})
	if err != nil {
		t.Fatalf("NewWebDAVFS failed: %v", err)
	}

	if err := ffs.MkdirAll("books/2026"); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}

	w, err := ffs.OpenWriter("books/2026/draft.txt")
	if err != nil {
		t.Fatalf("OpenWriter failed: %v", err)
	}

	if _, err := w.Write([]byte("hello webdav")); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	if err := w.Close(); err != nil {
		t.Fatalf("Close writer failed: %v", err)
	}

	if err := ffs.Rename("books/2026/draft.txt", "books/2026/final.txt"); err != nil {
		t.Fatalf("Rename failed: %v", err)
	}

	st, err := ffs.Stat("books/2026/final.txt")
	if err != nil {
		t.Fatalf("Stat failed: %v", err)
	}
	if st.IsDir() {
		t.Fatalf("expected final.txt to be a file")
	}

	f, err := ffs.Open("books/2026/final.txt")
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer f.Close()

	b, err := io.ReadAll(f)
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}
	if string(b) != "hello webdav" {
		t.Fatalf("unexpected file contents: %q", string(b))
	}

	entries, err := ffs.ReadDir("books/2026")
	if err != nil {
		t.Fatalf("ReadDir failed: %v", err)
	}

	entryNames := make([]string, 0, len(entries))
	for _, e := range entries {
		entryNames = append(entryNames, e.Name())
	}
	slices.Sort(entryNames)

	if !reflect.DeepEqual(entryNames, []string{"final.txt"}) {
		t.Fatalf("unexpected entries: %v", entryNames)
	}

	if err := ffs.Remove("books/2026/final.txt"); err != nil {
		t.Fatalf("Remove failed: %v", err)
	}

	if _, err := ffs.Stat("books/2026/final.txt"); err == nil {
		t.Fatalf("expected Stat to fail after remove")
	}

	if err := ffs.RemoveAll("books"); err != nil {
		t.Fatalf("RemoveAll failed: %v", err)
	}

	if _, err := ffs.Stat("books"); err == nil {
		t.Fatalf("expected books dir to be removed")
	}
}

func TestNewWebDAVFSAuthFailure(t *testing.T) {
	tmpDir := t.TempDir()

	const user = "alice"
	const pass = "secret"

	srv := newMockWebDAVServer(t, tmpDir, user, pass)
	t.Cleanup(srv.Close)

	_, err := NewWebDAVFS(&WebDAVConf{
		Host:     srv.URL,
		User:     user,
		Password: "wrong",
		BaseDir:  "/library",
	})
	if err == nil {
		t.Fatalf("expected auth failure")
	}
}

func newMockWebDAVServer(t *testing.T, root, user, pass string) *httptest.Server {
	t.Helper()

	h := &webdav.Handler{
		Prefix:     "/",
		FileSystem: webdav.Dir(root),
		LockSystem: webdav.NewMemLS(),
	}

	secured := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, p, ok := r.BasicAuth()
		if !ok || u != user || p != pass {
			w.Header().Set("WWW-Authenticate", `Basic realm="test"`)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		h.ServeHTTP(w, r)
	})

	return httptest.NewServer(secured)
}

func writeFile(t *testing.T, filePath, content string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		t.Fatalf("MkdirAll(%q) failed: %v", filePath, err)
	}

	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile(%q) failed: %v", filePath, err)
	}
}