package scancache

import (
	"cmp"
	"io/fs"
	"path"
	"slices"
	"testing"
	"testing/fstest"
	"time"

	"github.com/voilelab/plainshelf/internal/fsutil"
	"github.com/voilelab/plainshelf/internal/util"
)

type fakeFS struct {
	mod map[string]time.Time
}

func newFakeFS() *fakeFS {
	return &fakeFS{mod: map[string]time.Time{}}
}

func (f *fakeFS) mkdir(pth string, mod time.Time) {
	f.mod[pth] = mod
}

func (f *fakeFS) touch(pth string, mod time.Time) {
	f.mod[pth] = mod
}

func (f *fakeFS) rmdir(pth string) {
	for k := range f.mod {
		if k == pth || len(k) > len(pth) && k[:len(pth)+1] == pth+"/" {
			delete(f.mod, k)
		}
	}
}

func (f *fakeFS) Stat(name string) (fs.FileInfo, error) {
	mod, ok := f.mod[name]
	if !ok {
		return nil, util.Errorf("%w", fs.ErrNotExist)
	}
	return fakeInfo{name: path.Base(name), mod: mod}, nil
}

func (f *fakeFS) ReadDir(name string) ([]fs.DirEntry, error) {
	if _, ok := f.mod[name]; !ok {
		return nil, util.Errorf("%w", fs.ErrNotExist)
	}

	var entries []fs.DirEntry
	for k := range f.mod {
		if k != name && path.Dir(k) == name {
			entries = append(entries, fakeDirEntry{name: path.Base(k)})
		}
	}
	slices.SortFunc(entries, func(a, b fs.DirEntry) int { return cmp.Compare(a.Name(), b.Name()) })
	return entries, nil
}

func (f *fakeFS) Open(string) (fs.File, error) {
	return nil, util.Errorf("%w", fs.ErrNotExist)
}

type fakeInfo struct {
	name string
	mod  time.Time
}

func (i fakeInfo) Name() string       { return i.name }
func (fakeInfo) Size() int64          { return 0 }
func (fakeInfo) Mode() fs.FileMode    { return fs.ModeDir | 0755 }
func (i fakeInfo) ModTime() time.Time { return i.mod }
func (fakeInfo) IsDir() bool          { return true }
func (fakeInfo) Sys() any             { return nil }

type fakeDirEntry struct {
	name string
}

func (e fakeDirEntry) Name() string               { return e.name }
func (fakeDirEntry) IsDir() bool                  { return true }
func (fakeDirEntry) Type() fs.FileMode            { return fs.ModeDir }
func (e fakeDirEntry) Info() (fs.FileInfo, error) { return fakeInfo{name: e.name}, nil }

type nullStore struct{}

func (nullStore) ReadFile(string) ([]byte, error)      { return nil, util.NewError("empty") }
func (nullStore) EnsureWritable() error                { return nil }
func (nullStore) WriteFileAtomic(string, []byte) error { return nil }

func openCache(enabled bool) *Cache {
	return Open(Config{Store: nullStore{}, Enabled: enabled})
}

func walkTree(t *testing.T, cache *Cache, root fsutil.ReadFS, start string) (map[string]bool, Stats) {
	t.Helper()

	w := cache.NewWalk()
	found := map[string]bool{}

	var dfs func(pth string, child *DirChild, parentListingReused bool)
	dfs = func(pth string, child *DirChild, parentListingReused bool) {
		isDir, err := ChildIsDir(root, pth, child)
		if err != nil || !isDir {
			return
		}
		found[pth] = true

		children, childrenTrusted, err := w.ReadDir(root, pth, parentListingReused)
		if err != nil {
			t.Fatalf("ReadDir(%q): %v", pth, err)
		}
		for i := range children {
			dfs(path.Join(pth, children[i].Name), &children[i], childrenTrusted)
		}
	}

	dfs(start, nil, true)
	return found, w.Install()
}

func TestScanCacheFindsChildOfReusedParent(t *testing.T) {
	aged := time.Now().Add(-time.Minute)

	f := newFakeFS()
	f.mkdir("books", aged)
	f.mkdir("books/Fiction", aged)
	f.mkdir("books/Fiction/Classics", aged)
	f.mkdir("books/Fiction/Classics/alpha", aged)

	cache := openCache(true)

	walkTree(t, cache, f, "books")
	if _, stats := walkTree(t, cache, f, "books"); stats.ReusedDirs != stats.Dirs {
		t.Fatalf("warm walk reused %d of %d directories, want all", stats.ReusedDirs, stats.Dirs)
	}

	f.mkdir("books/Fiction/Classics/beta", aged)
	f.touch("books/Fiction/Classics", time.Now().Add(-30*time.Second))

	found, warm := walkTree(t, cache, f, "books")

	if warm.ReadDirs == 0 {
		t.Errorf("walk listed no directory, but Fiction/Classics changed")
	}
	if warm.ReusedDirs == 0 {
		t.Errorf("walk reused no directory, but books and books/Fiction did not change")
	}
	if !found["books/Fiction/Classics/beta"] {
		t.Errorf("the walk did not find the directory added under a reused parent")
	}
}

func TestScanCacheRefusesRacyDirectories(t *testing.T) {
	f := newFakeFS()
	f.mkdir("books", time.Now())
	f.mkdir("books/Fiction", time.Now())

	cache := openCache(true)

	walkTree(t, cache, f, "books")

	if _, warm := walkTree(t, cache, f, "books"); warm.ReusedDirs != 0 {
		t.Errorf("walk reused %d directories modified inside the racy window, want 0", warm.ReusedDirs)
	}

	f.mkdir("books/Fiction/Poetry", time.Now())
	found, _ := walkTree(t, cache, f, "books")
	if !found["books/Fiction/Poetry"] {
		t.Errorf("walk did not find the directory added after a racy scan")
	}
}

func TestScanCacheDistrustsAReplacedDirectory(t *testing.T) {
	aged := time.Now().Add(-time.Minute)

	f := newFakeFS()
	f.mkdir("books", aged)
	f.mkdir("books/Fiction", aged)
	f.mkdir("books/Fiction/kept", aged)

	cache := openCache(true)

	walkTree(t, cache, f, "books")
	walkTree(t, cache, f, "books")

	ficModTime := f.mod["books/Fiction"]

	f.rmdir("books/Fiction")
	f.mkdir("books/Fiction", ficModTime)
	f.mkdir("books/Fiction/brought-in", aged)
	f.touch("books", time.Now().Add(-30*time.Second))

	found, _ := walkTree(t, cache, f, "books")
	if !found["books/Fiction/brought-in"] {
		t.Errorf("walk did not find the replacement directory's child")
	}
	if found["books/Fiction/kept"] {
		t.Errorf("walk still reports the replaced directory's child")
	}
}

func TestDirChildrenListsOnlyKeptFiles(t *testing.T) {
	fsys := fstest.MapFS{
		"d/folder.json":         {Data: []byte("{}")},
		"d/notes.txt":           {Data: []byte("x")},
		"d/Sub/folder.json/x":   {Data: []byte("x")},
		"d/book.bookpkg/x.json": {Data: []byte("x")},
	}
	entries, err := fsys.ReadDir("d")
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}

	got := dirChildren(entries, []string{"folder.json"})
	want := []DirChild{{Name: "Sub"}, {Name: "book.bookpkg"}, {Name: "folder.json", File: true}}
	if !slices.Equal(got, want) {
		t.Fatalf("dirChildren = %+v, want %+v", got, want)
	}

	sub, err := fsys.ReadDir("d/Sub")
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if got := dirChildren(sub, []string{"folder.json"}); !slices.Equal(got, []DirChild{{Name: "folder.json"}}) {
		t.Fatalf("a directory named folder.json = %+v, want it listed as a directory", got)
	}
}

type addChildAfterListingFS struct {
	*fakeFS
	dir   string
	added bool
}

func (f *addChildAfterListingFS) ReadDir(name string) ([]fs.DirEntry, error) {
	entries, err := f.fakeFS.ReadDir(name)
	if name == f.dir && !f.added {
		f.added = true
		f.mkdir(path.Join(name, "Poetry"), time.Now().Add(-time.Minute))
		f.touch(name, time.Now().Add(-30*time.Second))
	}
	return entries, err
}

func TestScanCacheRejectsRacyListing(t *testing.T) {
	aged := time.Now().Add(-time.Minute)

	f := &addChildAfterListingFS{fakeFS: newFakeFS(), dir: "books/Fiction"}
	f.mkdir("books", aged)
	f.mkdir("books/Fiction", aged)

	cache := openCache(true)

	walkTree(t, cache, f, "books")
	found, _ := walkTree(t, cache, f, "books")
	if !found["books/Fiction/Poetry"] {
		t.Errorf("a child added between Stat and ReadDir was not found by the next walk")
	}
}

func TestScanCacheIgnoresZeroDirModTime(t *testing.T) {
	f := newFakeFS()
	f.mkdir("books", time.Time{})
	cache := openCache(true)
	walkTree(t, cache, f, "books")
	if _, stats := walkTree(t, cache, f, "books"); stats.ReadDirs != 1 || stats.ReusedDirs != 0 {
		t.Errorf("zero mtime walk = %+v, want one listing and no reuse", stats)
	}
}

func TestAbandonPreservesSnapshotAndStats(t *testing.T) {
	originalModTime := time.Now().Add(-time.Minute)
	f := newFakeFS()
	f.mkdir("books", originalModTime)
	cache := openCache(true)
	walkTree(t, cache, f, "books")

	f.mkdir("books/new", time.Now().Add(-time.Minute))
	f.touch("books", time.Now().Add(-30*time.Second))
	w := cache.NewWalk()
	children, reused, err := w.ReadDir(f, "books", true)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(children) != 1 || reused {
		t.Fatalf("ReadDir = %+v, reused %v, want the changed listing", children, reused)
	}
	stats := w.Abandon()
	if stats.Dirs != 1 || stats.ReadDirs != 1 || stats.Duration <= 0 || cache.LastStats() != stats {
		t.Errorf("Abandon stats = %+v, last stats = %+v", stats, cache.LastStats())
	}
	if got := cache.dirs["books"]; len(got.Children) != 0 || !got.ModTime.Equal(originalModTime) {
		t.Errorf("abandoned walk replaced the previous snapshot: %+v", got)
	}

	if found, stats := walkTree(t, cache, f, "books"); !found["books/new"] || stats.ReadDirs == 0 {
		t.Errorf("walk after abandonment = %v, stats = %+v, want the new directory found by listing", found, stats)
	}
}
