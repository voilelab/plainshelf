package shelfzip

import (
	"archive/zip"
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

// writeTree creates files (path -> content; a trailing "/" makes a directory) under root.
func writeTree(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for p, content := range files {
		full := filepath.Join(root, filepath.FromSlash(p))
		if p[len(p)-1] == '/' {
			if err := os.MkdirAll(full, 0o755); err != nil {
				t.Fatal(err)
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// readTree lists root as path -> content, directories with a trailing "/".
func readTree(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || p == root {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			out[rel+"/"] = ""
			return nil
		}
		b, err := os.ReadFile(p)
		out[rel] = string(b)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func zipOf(t *testing.T, entries map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	names := make([]string, 0, len(entries))
	for n := range entries {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		w, err := zw.Create(n)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(entries[n])); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func importBytes(b []byte, root string) error {
	return Import(bytes.NewReader(b), int64(len(b)), root)
}

func TestRoundTripKeepsBooksTrashAndEmptyFolders(t *testing.T) {
	src := t.TempDir()
	writeTree(t, src, map[string]string{
		"books/在瀏覽器裡建立的書.bookpkg/book.json":       `{"title":"在瀏覽器裡建立的書"}`,
		"books/收藏/x.bookpkg/sources/1/source.txt": "text",
		"books/空的資料夾/":                            "",
		"trash/books/old.bookpkg/book.json":       "{}",
		"app/book-cache-abc.json":                 "cache",
	})
	var buf bytes.Buffer
	if err := Export(&buf, src); err != nil {
		t.Fatalf("Export: %v", err)
	}

	dst := t.TempDir()
	if err := importBytes(buf.Bytes(), dst); err != nil {
		t.Fatalf("Import: %v", err)
	}
	got := readTree(t, dst)
	want := readTree(t, src)
	for p := range want {
		if p == "app/" || filepath.Dir(p) == "app" {
			delete(want, p)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("imported tree = %v, want %v", got, want)
	}
	for p, c := range want {
		if got[p] != c {
			t.Errorf("%s = %q, want %q", p, got[p], c)
		}
	}
}

func TestImportReplacesExistingBooks(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"books/old.bookpkg/book.json": "old",
		"app/keep.json":               "kept",
	})
	if err := importBytes(zipOf(t, map[string]string{"books/new.bookpkg/book.json": "new"}), root); err != nil {
		t.Fatalf("Import: %v", err)
	}
	got := readTree(t, root)
	if _, ok := got["books/old.bookpkg/book.json"]; ok {
		t.Error("old book survived the import")
	}
	if got["books/new.bookpkg/book.json"] != "new" {
		t.Errorf("new book = %q", got["books/new.bookpkg/book.json"])
	}
	if got["app/keep.json"] != "kept" {
		t.Error("import touched app/")
	}
	if _, err := os.Stat(root + ".import"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("staging directory left behind: %v", err)
	}
}

func TestImportStripsOneWrappingFolder(t *testing.T) {
	root := t.TempDir()
	z := zipOf(t, map[string]string{
		"shelf/books/a.bookpkg/book.json": "a",
		"shelf/app/cache.json":            "x",
	})
	if err := importBytes(z, root); err != nil {
		t.Fatalf("Import: %v", err)
	}
	if got := readTree(t, root)["books/a.bookpkg/book.json"]; got != "a" {
		t.Errorf("book = %q", got)
	}
}

func TestImportRejectsWithoutTouchingTheShelf(t *testing.T) {
	cases := map[string]struct {
		entries map[string]string
		want    error
	}{
		"parent escape":   {map[string]string{"books/a/book.json": "a", "books/../../evil": "x"}, ErrUnsafePath},
		"absolute path":   {map[string]string{"books/a/book.json": "a", "/etc/evil": "x"}, ErrUnsafePath},
		"backslash":       {map[string]string{"books/a/book.json": "a", `books\..\evil`: "x"}, ErrUnsafePath},
		"no books":        {map[string]string{"notes/readme.txt": "x"}, ErrNotShelf},
		"only app folder": {map[string]string{"app/cache.json": "x"}, ErrNotShelf},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			writeTree(t, root, map[string]string{"books/existing.bookpkg/book.json": "keep"})
			err := importBytes(zipOf(t, tc.entries), root)
			if !errors.Is(err, tc.want) {
				t.Fatalf("Import err = %v, want %v", err, tc.want)
			}
			if got := readTree(t, root)["books/existing.bookpkg/book.json"]; got != "keep" {
				t.Errorf("existing book = %q after a rejected import", got)
			}
		})
	}
}

// withLimit lowers maxImportBytes for one test.
func withLimit(t *testing.T, n int64) {
	t.Helper()
	old := maxImportBytes
	maxImportBytes = n
	t.Cleanup(func() { maxImportBytes = old })
}

func TestImportRejectsOversizedZip(t *testing.T) {
	withLimit(t, 4<<20)
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.CreateHeader(&zip.FileHeader{Name: "books/big.bookpkg/source.txt", Method: zip.Deflate})
	if err != nil {
		t.Fatal(err)
	}
	chunk := make([]byte, 1<<20)
	for range maxImportBytes>>20 + 1 {
		if _, err := w.Write(chunk); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := importBytes(buf.Bytes(), t.TempDir()); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("Import err = %v, want ErrTooLarge", err)
	}
}
