package scancache

import (
	"cmp"
	"io/fs"
	"path"
	"slices"
	"testing"
	"time"

	"github.com/voilelab/plainshelf/internal/fsutil"
	"github.com/voilelab/plainshelf/internal/util"
)

// These tests exercise the mtime trust chain directly against a fake ReadFS,
// with no shelf, no lock and no book packages: the cache is the layer that
// answers "which entries does this directory hold", and that is the whole layer
// under test here. The integration cases that need a real shelf - a snapshot
// surviving a reopen, and the large-tree measurement - stay in package shelf.

// fakeFS is an in-memory tree of directories only, which is all the walk reads:
// ReadDir lists a directory's entries and Stat reports its mtime. A path exists
// iff it has an mtime, and a directory's children are the paths one level below
// it. Mtimes are set explicitly so a test can put a directory inside or outside
// fsutil.RacyWindow, or hand a replacement the mtime of what it replaced.
type fakeFS struct {
	mod map[string]time.Time
}

func newFakeFS() *fakeFS {
	return &fakeFS{mod: map[string]time.Time{}}
}

// mkdir creates a directory at pth with modification time mod. Parents are not
// created implicitly: a test builds the tree top down, the way it reasons about
// it.
func (f *fakeFS) mkdir(pth string, mod time.Time) {
	f.mod[pth] = mod
}

// touch changes a directory's modification time, standing in for a child being
// added, removed or renamed under it.
func (f *fakeFS) touch(pth string, mod time.Time) {
	f.mod[pth] = mod
}

// rmdir removes a directory and everything under it.
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

// Open is never called by the walk; the walk descends through Stat and ReadDir.
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

// nullStore is a Store with no file behind it: loading always misses (so a walk
// starts from an empty snapshot) and writes are accepted and discarded. These
// tests never persist - the reopen case that does lives in package shelf.
type nullStore struct{}

func (nullStore) ReadFile(string) ([]byte, error)      { return nil, util.NewError("empty") }
func (nullStore) EnsureWritable() error                { return nil }
func (nullStore) WriteFileAtomic(string, []byte) error { return nil }

// openCache builds a cache over a null store, the way the shelf facade builds one
// over app/ but with nothing on disk to load.
func openCache(enabled bool) *Cache {
	return Open(Config{Store: nullStore{}, Enabled: enabled})
}

// walkTree runs one complete walk of root the way the shelf's iterateShelfTree
// does - NewWalk, a depth-first descent, then Install - but for the directory
// layer alone. It returns the set of directory paths it reached and what the
// walk cost. childrenTrusted is threaded down exactly as the real walk threads
// it, which is what puts the trust chain under test.
func walkTree(t *testing.T, cache *Cache, root fsutil.ReadFS, start string) (map[string]bool, Stats) {
	t.Helper()

	w := cache.NewWalk()
	found := map[string]bool{}

	var dfs func(pth string, child *DirChild, trusted bool)
	dfs = func(pth string, child *DirChild, trusted bool) {
		isDir, err := ChildIsDir(root, pth, child)
		if err != nil || !isDir {
			return
		}
		found[pth] = true

		children, childrenTrusted, err := w.ReadDir(root, pth, trusted)
		if err != nil {
			t.Fatalf("ReadDir(%q): %v", pth, err)
		}
		for i := range children {
			dfs(path.Join(pth, children[i].Name), &children[i], childrenTrusted)
		}
	}

	dfs(start, nil, true)
	return found, w.Install(true)
}

// A directory's mtime only reports on its direct children, so the cache has to
// be consulted per directory rather than per subtree: a child added deep in the
// tree must still be found while every directory above it is reused.
func TestScanCacheFindsChildAddedUnderReusedParent(t *testing.T) {
	aged := time.Now().Add(-time.Minute)

	f := newFakeFS()
	f.mkdir("books", aged)
	f.mkdir("books/Fiction", aged)
	f.mkdir("books/Fiction/Classics", aged)
	f.mkdir("books/Fiction/Classics/alpha", aged)

	cache := openCache(true)

	// Cold walk records the snapshot; warm walk proves it is fully reusable.
	walkTree(t, cache, f, "books")
	if _, stats := walkTree(t, cache, f, "books"); stats.ReusedDirs != stats.Dirs {
		t.Fatalf("warm walk reused %d of %d directories, want all", stats.ReusedDirs, stats.Dirs)
	}

	// Add a directory under Classics, which changes only Classics' mtime.
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

// Timestamps are coarse. A directory modified in the same tick the walk read it
// would keep that mtime, so remembering it would hide every later change to it
// forever. Such a directory must simply not enter the snapshot.
func TestScanCacheRefusesRacyDirectories(t *testing.T) {
	f := newFakeFS()
	// No ageing: every directory was modified moments ago, inside RacyWindow.
	f.mkdir("books", time.Now())
	f.mkdir("books/Fiction", time.Now())

	cache := openCache(true)

	walkTree(t, cache, f, "books")

	if _, warm := walkTree(t, cache, f, "books"); warm.ReusedDirs != 0 {
		t.Errorf("walk reused %d directories modified inside the racy window, want 0", warm.ReusedDirs)
	}

	// And the change that trusting a racy directory would have hidden is found.
	f.mkdir("books/Fiction/Poetry", time.Now())
	found, _ := walkTree(t, cache, f, "books")
	if !found["books/Fiction/Poetry"] {
		t.Errorf("walk did not find the directory added after a racy scan")
	}
}

// A directory's mtime identifies its content, not the directory. If one is
// moved away and another takes its place carrying the same mtime - which
// coarse-timestamp filesystems and timestamp-preserving copies both make
// possible - matching the mtime alone would serve the old directory's children
// forever, and the directories under the new one would never appear. The parent
// is what rules that out: the rename changes the parent's mtime, the parent is
// therefore relisted and distrusted, and the distrust cascades to the child.
func TestScanCacheDistrustsAReplacedDirectory(t *testing.T) {
	aged := time.Now().Add(-time.Minute)

	f := newFakeFS()
	f.mkdir("books", aged)
	f.mkdir("books/Fiction", aged)
	f.mkdir("books/Fiction/kept", aged)

	cache := openCache(true)

	// Cold then warm, so the snapshot is populated and proven reusable.
	walkTree(t, cache, f, "books")
	walkTree(t, cache, f, "books")

	ficModTime := f.mod["books/Fiction"]

	// Replace Fiction's content wholesale but give the replacement the exact
	// mtime the walk remembered; the rename bumps the parent's mtime.
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
