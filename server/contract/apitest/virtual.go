package apitest

import (
	"io/fs"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/voilelab/plainshelf/internal/fsutil"
	"github.com/voilelab/plainshelf/internal/util"
	"github.com/voilelab/plainshelf/shelf"
)

const VirtualShelfID = "virtual_shelf"

type VirtualSource struct {
	inner fsutil.ReadFS
	down  atomic.Bool
}

func (v *VirtualSource) SetOffline(offline bool) {
	v.down.Store(offline)
}

func (v *VirtualSource) check(name string) error {
	if v.down.Load() {
		return util.Errorf("%s: %w", name, fsutil.ErrSourceUnavailable)
	}
	return nil
}

func (v *VirtualSource) Open(name string) (fs.File, error) {
	if err := v.check(name); err != nil {
		return nil, err
	}
	return v.inner.Open(name)
}

func (v *VirtualSource) ReadDir(name string) ([]fs.DirEntry, error) {
	if err := v.check(name); err != nil {
		return nil, err
	}
	return v.inner.ReadDir(name)
}

func (v *VirtualSource) Stat(name string) (fs.FileInfo, error) {
	if err := v.check(name); err != nil {
		return nil, err
	}
	return v.inner.Stat(name)
}

func SeedVirtualSource(t *testing.T, titles ...string) (string, map[string]string) {
	t.Helper()

	libRoot := t.TempDir()
	s, err := shelf.NewShelf(&shelf.ShelfConf{LibRoot: libRoot, LockMode: "none", BookCacheWriterID: "apitest-source"})
	if err != nil {
		t.Fatalf("NewShelf: %v", err)
	}
	if err := s.WaitReady(t.Context()); err != nil {
		t.Fatalf("WaitReady: %v", err)
	}

	ids := map[string]string{}
	for _, title := range titles {
		book, err := s.NewBook(shelf.FolderPath{}, title)
		if err != nil {
			t.Fatalf("NewBook(%q): %v", title, err)
		}
		source, err := book.NewSource(strings.NewReader(title + " text"))
		if err != nil {
			t.Fatalf("NewSource: %v", err)
		}
		if err := book.SetCurrentSource(source.ID()); err != nil {
			t.Fatalf("SetCurrentSource: %v", err)
		}
		ids[title] = book.ID()
	}
	if _, err := s.ExportBookCache(); err != nil {
		t.Fatalf("ExportBookCache: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	return libRoot, ids
}

func (env *Env) AddVirtualShelf(t *testing.T, libRoot string) *VirtualSource {
	t.Helper()

	rt, err := os.OpenRoot(libRoot)
	if err != nil {
		t.Fatalf("OpenRoot: %v", err)
	}
	t.Cleanup(func() { rt.Close() })

	source := &VirtualSource{inner: fsutil.NewRootFS(rt)}
	conf := shelf.ShelfConfWithID{ID: VirtualShelfID, LibRoot: libRoot, ScanInterval: "1h"}
	if err := env.App.AddVirtualShelf(conf, source, "test://"+libRoot); err != nil {
		t.Fatalf("AddVirtualShelf: %v", err)
	}
	shelfData, ok := env.App.ShelfManager().GetShelf(VirtualShelfID)
	if !ok {
		t.Fatal("virtual shelf missing after AddVirtualShelf")
	}
	if err := shelfData.WaitReady(t.Context()); err != nil {
		t.Fatalf("WaitReady: %v", err)
	}
	return source
}
