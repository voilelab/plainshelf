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

	// beforeReadDir runs at the start of every ReadDir, so a test can change the
	// tree in the window between the walk's Stat of a directory and its listing
	// of it. Optional.
	beforeReadDir func(name string)
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
	if f.beforeReadDir != nil {
		f.beforeReadDir(name)
	}

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

// verifyTree runs one complete verifying walk of root the way a user-pressed
// rescan does, and returns what the snapshot got wrong.
func verifyTree(t *testing.T, cache *Cache, root fsutil.ReadFS, start string) ([]Mismatch, Stats) {
	t.Helper()

	w := cache.NewVerifyingWalk()

	var dfs func(pth string, child *DirChild, trusted bool)
	dfs = func(pth string, child *DirChild, trusted bool) {
		isDir, err := ChildIsDir(root, pth, child)
		if err != nil || !isDir {
			return
		}

		children, childrenTrusted, err := w.ReadDir(root, pth, trusted)
		if err != nil {
			t.Fatalf("ReadDir(%q): %v", pth, err)
		}
		for i := range children {
			dfs(path.Join(pth, children[i].Name), &children[i], childrenTrusted)
		}
	}

	dfs(start, nil, true)
	return w.Mismatches(), w.Install(true)
}

// The failure the diagnosis exists for: a gateway that does not touch a
// directory's mtime when a child is added. The mtime says nothing changed, so
// every ordinary walk reuses the snapshot and the new book is invisible - not
// late, permanently. A verifying walk lists the directory anyway and names it.
func TestVerifyingWalkReportsAChildAddedWithoutAnMtimeChange(t *testing.T) {
	aged := time.Now().Add(-time.Minute)

	f := newFakeFS()
	f.mkdir("books", aged)
	f.mkdir("books/Fiction", aged)

	cache := openCache(true)
	walkTree(t, cache, f, "books")

	// Added without touching books/Fiction's mtime, which is the whole fault.
	f.mkdir("books/Fiction/dropped-in.bookpkg", aged)

	// An ordinary walk cannot see it, which is what makes the report the only
	// way a user could learn about it.
	if found, _ := walkTree(t, cache, f, "books"); found["books/Fiction/dropped-in.bookpkg"] {
		t.Fatal("an ordinary walk found the child, so this filesystem does not reproduce the fault")
	}

	mismatches, stats := verifyTree(t, cache, f, "books")

	if len(mismatches) != 1 {
		t.Fatalf("mismatches = %v, want exactly books/Fiction", mismatches)
	}
	if mismatches[0].Dir != "books/Fiction" {
		t.Errorf("mismatch dir = %q, want %q", mismatches[0].Dir, "books/Fiction")
	}
	if !slices.Equal(mismatches[0].Missing, []string{"dropped-in.bookpkg"}) {
		t.Errorf("missing = %v, want the child the snapshot does not list", mismatches[0].Missing)
	}
	if len(mismatches[0].Stale) != 0 {
		t.Errorf("stale = %v, want none: nothing was removed", mismatches[0].Stale)
	}
	if stats.ReusedDirs != 0 {
		t.Errorf("reused %d directories, want 0: a verifying walk lists them all", stats.ReusedDirs)
	}
	if stats.CheckedDirs != 2 {
		t.Errorf("checked %d directories, want both books and books/Fiction", stats.CheckedDirs)
	}
}

// The opposite report from the same cause: a book deleted from outside that the
// snapshot goes on listing.
func TestVerifyingWalkReportsAChildRemovedWithoutAnMtimeChange(t *testing.T) {
	aged := time.Now().Add(-time.Minute)

	f := newFakeFS()
	f.mkdir("books", aged)
	f.mkdir("books/gone.bookpkg", aged)

	cache := openCache(true)
	walkTree(t, cache, f, "books")

	booksModTime := f.mod["books"]
	f.rmdir("books/gone.bookpkg")
	f.touch("books", booksModTime)

	mismatches, _ := verifyTree(t, cache, f, "books")

	if len(mismatches) != 1 {
		t.Fatalf("mismatches = %v, want exactly books", mismatches)
	}
	if !slices.Equal(mismatches[0].Stale, []string{"gone.bookpkg"}) {
		t.Errorf("stale = %v, want the child the snapshot still lists", mismatches[0].Stale)
	}
}

// The reverse condition: a shelf whose directory times are trustworthy must
// produce nothing at all, whether or not it changed since the last walk. A
// diagnosis that fires on a healthy shelf is worse than none.
func TestVerifyingWalkIsSilentOnAConsistentShelf(t *testing.T) {
	aged := time.Now().Add(-time.Minute)

	f := newFakeFS()
	f.mkdir("books", aged)
	f.mkdir("books/Fiction", aged)

	cache := openCache(true)
	walkTree(t, cache, f, "books")

	if mismatches, _ := verifyTree(t, cache, f, "books"); len(mismatches) != 0 {
		t.Errorf("mismatches = %v on an unchanged shelf, want none", mismatches)
	}

	// A change the mount did report is the cache working, not failing: the
	// parent is relisted, so there is no snapshot claim left to contradict.
	f.mkdir("books/Fiction/Classics", aged)
	f.touch("books/Fiction", time.Now().Add(-30*time.Second))

	if mismatches, _ := verifyTree(t, cache, f, "books"); len(mismatches) != 0 {
		t.Errorf("mismatches = %v after a change the mount reported, want none", mismatches)
	}
}

// The other reverse condition: with scan_cache off there is no snapshot to
// disagree with, so the check has nothing to say and must not invent a fault.
func TestVerifyingWalkReportsNothingWithTheCacheOff(t *testing.T) {
	aged := time.Now().Add(-time.Minute)

	f := newFakeFS()
	f.mkdir("books", aged)
	f.mkdir("books/Fiction", aged)

	cache := openCache(false)
	walkTree(t, cache, f, "books")

	f.mkdir("books/Fiction/dropped-in.bookpkg", aged)

	mismatches, stats := verifyTree(t, cache, f, "books")
	if len(mismatches) != 0 {
		t.Errorf("mismatches = %v with scan_cache off, want none", mismatches)
	}
	if stats.CheckedDirs != 0 {
		t.Errorf("checked %d directories with scan_cache off, want 0", stats.CheckedDirs)
	}
}

// An ordinary walk must be exactly what it was: the fast path is the reason the
// cache exists, and the diagnosis is not allowed to cost it anything.
func TestOrdinaryWalkStillReusesAndReportsNoMismatches(t *testing.T) {
	aged := time.Now().Add(-time.Minute)

	f := newFakeFS()
	f.mkdir("books", aged)
	f.mkdir("books/Fiction", aged)
	f.mkdir("books/Fiction/Classics", aged)

	cache := openCache(true)
	walkTree(t, cache, f, "books")

	w := cache.NewWalk()
	found := map[string]bool{}
	var dfs func(pth string, child *DirChild, trusted bool)
	dfs = func(pth string, child *DirChild, trusted bool) {
		isDir, err := ChildIsDir(f, pth, child)
		if err != nil || !isDir {
			return
		}
		found[pth] = true
		children, childrenTrusted, err := w.ReadDir(f, pth, trusted)
		if err != nil {
			t.Fatalf("ReadDir(%q): %v", pth, err)
		}
		for i := range children {
			dfs(path.Join(pth, children[i].Name), &children[i], childrenTrusted)
		}
	}
	dfs("books", nil, true)
	stats := w.Install(true)

	if stats.ReadDirs != 0 {
		t.Errorf("ordinary warm walk listed %d directories, want 0", stats.ReadDirs)
	}
	if stats.ReusedDirs != stats.Dirs {
		t.Errorf("ordinary warm walk reused %d of %d directories, want all", stats.ReusedDirs, stats.Dirs)
	}
	if len(w.Mismatches()) != 0 {
		t.Errorf("ordinary walk reported %v, want no mismatches", w.Mismatches())
	}
}

// A working filesystem may change a directory between the walk's Stat and its
// ReadDir - a sync client finishing its copy while the user presses the button,
// which is exactly when it is most likely. The listing then disagrees with a
// snapshot that was accurate when it was read, and reporting that would send the
// user to turn off the setting that is doing its job.
func TestVerifyingWalkDoesNotBlameAConcurrentChange(t *testing.T) {
	aged := time.Now().Add(-time.Minute)

	f := newFakeFS()
	f.mkdir("books", aged)
	f.mkdir("books/Fiction", aged)

	cache := openCache(true)
	walkTree(t, cache, f, "books")

	// The child lands after the walk has stat'd Fiction and before it lists it,
	// and this filesystem does report the change by moving Fiction's mtime.
	f.beforeReadDir = func(name string) {
		if name != "books/Fiction" {
			return
		}
		f.beforeReadDir = nil
		f.mkdir("books/Fiction/arrived-mid-walk.bookpkg", aged)
		f.touch("books/Fiction", time.Now().Add(-30*time.Second))
	}

	mismatches, stats := verifyTree(t, cache, f, "books")

	if len(mismatches) != 0 {
		t.Errorf("mismatches = %+v, want none: the mount reported the change itself", mismatches)
	}
	if stats.CheckedDirs != 2 {
		t.Errorf("checked %d directories, want both: the walk still paid for the listing", stats.CheckedDirs)
	}

	// And the next walk still finds the child, because the mtime that moved is
	// not the one the snapshot now carries.
	if found, _ := walkTree(t, cache, f, "books"); !found["books/Fiction/arrived-mid-walk.bookpkg"] {
		t.Error("the walk after the concurrent change did not find the child")
	}
}
