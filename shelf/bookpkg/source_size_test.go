package bookpkg

import (
	"errors"
	"io/fs"
	"testing"
	"testing/fstest"

	"github.com/voilelab/plainshelf/internal/fsutil"
)

type sizeReadFS struct {
	fsutil.ReadFS
	readErr error
}

func (f sizeReadFS) ReadDir(name string) ([]fs.DirEntry, error) {
	if f.readErr != nil {
		return nil, f.readErr
	}
	return f.ReadFS.ReadDir(name)
}

func TestSourceSizeIncludesAllAssets(t *testing.T) {
	files := fstest.MapFS{
		"source.txt":              {Data: []byte("missing.png\npage.jpg\n")},
		"meta.json":               {Data: []byte("excluded")},
		"assets/page.jpg":         {Data: []byte("page")},
		"assets/unlisted.png":     {Data: []byte("stray")},
		"assets/nested/notes.txt": {Data: []byte("notes")},
		"assets/link.png":         {Mode: fs.ModeSymlink, Data: []byte("excluded")},
	}
	source := &Source{root: files, folderPath: "."}
	size, err := source.SizeBytes()
	if err != nil || size != int64(len(files["source.txt"].Data)+14) {
		t.Fatalf("size = %d, %v, want content plus all regular assets", size, err)
	}
	source.root = sizeReadFS{ReadFS: files, readErr: fs.ErrPermission}
	if _, err := source.SizeBytes(); !errors.Is(err, fs.ErrPermission) {
		t.Fatalf("size read error = %v, want permission error instead of partial size", err)
	}
}
