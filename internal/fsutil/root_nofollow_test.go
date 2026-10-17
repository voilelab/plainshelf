package fsutil

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func TestOpenNoFollow(t *testing.T) {
	dir := t.TempDir()
	for name, body := range map[string]string{"a/b/file": "ok", "hidden/secret": "no"} {
		p := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for link, target := range map[string]string{"a/dirlink": "../hidden", "a/filelink": "../hidden/secret"} {
		if err := os.Symlink(target, filepath.Join(dir, filepath.FromSlash(link))); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { root.Close() })
	fsys := ReadOnly(NewRootFS(root)).(NoFollowFS)

	f, err := fsys.OpenNoFollow("a/b/file")
	if err != nil {
		t.Fatalf("OpenNoFollow(a/b/file): %v", err)
	}
	bs, _ := io.ReadAll(f)
	f.Close()
	if string(bs) != "ok" {
		t.Errorf("read %q", bs)
	}
	for _, name := range []string{".", "a", "a/b"} {
		f, err := fsys.OpenNoFollow(name)
		if err != nil {
			t.Errorf("OpenNoFollow(%s): %v", name, err)
			continue
		}
		f.Close()
	}

	for _, name := range []string{"a/dirlink", "a/dirlink/secret", "a/filelink", "a/b/file/x", "a/missing"} {
		if f, err := fsys.OpenNoFollow(name); !errors.Is(err, fs.ErrNotExist) {
			if f != nil {
				f.Close()
			}
			t.Errorf("OpenNoFollow(%s) = %v, want fs.ErrNotExist", name, err)
		}
	}
}
