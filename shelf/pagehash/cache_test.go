package pagehash

import (
	"errors"
	"io/fs"
	"os"
	"path"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/voilelab/plainshelf/internal/appcache"
	"github.com/voilelab/plainshelf/internal/fsutil"
	"github.com/voilelab/plainshelf/internal/logutil"
	"github.com/voilelab/plainshelf/shelf/bookpkg"
)

type testPage struct {
	name string
	data string
}

type testShelf struct {
	t       *testing.T
	libRoot string
	base    fsutil.FS
	live    map[string]struct{}
}

func newTestShelf(t *testing.T) *testShelf {
	t.Helper()
	libRoot := t.TempDir()
	osRoot, err := os.OpenRoot(libRoot)
	if err != nil {
		t.Fatalf("OpenRoot: %v", err)
	}
	t.Cleanup(func() { _ = osRoot.Close() })
	base := fsutil.NewRootFS(osRoot)
	if err := base.MkdirAll("app"); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	return &testShelf{t: t, libRoot: libRoot, base: base, live: map[string]struct{}{}}
}

func (ts *testShelf) liveBooks() (map[string]struct{}, bool) {
	return ts.live, true
}

func testLogger() logutil.Logger {
	logger, err := logutil.NewLogger(&logutil.LogConf{LogFile: logutil.LogFileConf{Type: logutil.LogFileTypeNone}})
	if err != nil {
		panic(err)
	}
	return *logger
}

func (ts *testShelf) addComic(id string, pages ...testPage) *bookpkg.Source {
	t := ts.t
	t.Helper()
	book, err := bookpkg.Create(ts.base, testLogger(), id+".bookpkg", id, id)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	names := make([]string, 0, len(pages))
	for _, page := range pages {
		names = append(names, page.name)
	}
	source, err := book.NewSourceWithOptions(strings.NewReader(strings.Join(names, "\n")+"\n"),
		bookpkg.NewSourceOptions{Format: bookpkg.BookFormatImages})
	if err != nil {
		t.Fatalf("NewSource: %v", err)
	}
	for _, page := range pages {
		ts.writePage(source, page)
	}
	ts.live[id] = struct{}{}
	return source
}

func (ts *testShelf) writePage(source *bookpkg.Source, page testPage) {
	t := ts.t
	t.Helper()
	if err := source.WriteAsset(page.name, []byte(page.data)); err != nil {
		t.Fatalf("WriteAsset: %v", err)
	}
	assetPath, err := source.AssetPath(page.name)
	if err != nil {
		t.Fatalf("AssetPath: %v", err)
	}
	settled := time.Now().Add(-time.Hour)
	if err := os.Chtimes(path.Join(ts.libRoot, assetPath), settled, settled); err != nil {
		t.Fatalf("Chtimes: %v", err)
	}
}

type countingFS struct {
	fsutil.FS
	assetOpens atomic.Int64
}

func (f *countingFS) Open(name string) (fs.File, error) {
	if path.Base(path.Dir(name)) == bookpkg.SourceAssetsFolder {
		f.assetOpens.Add(1)
	}
	return f.FS.Open(name)
}

func (ts *testShelf) open(root fsutil.ReadFS) *Cache {
	return Open(Config{Store: appcache.NewFSStore(root, "app"), Logger: testLogger(), LiveBooks: ts.liveBooks})
}

func (ts *testShelf) reopenSource(root fsutil.ReadFS, id string) *bookpkg.Source {
	t := ts.t
	t.Helper()
	book, err := bookpkg.Open(root, testLogger(), id+".bookpkg")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	sources, err := book.ListSource()
	if err != nil || len(sources) != 1 {
		t.Fatalf("ListSource: %v, %d sources", err, len(sources))
	}
	return sources[0]
}

func mustResolve(t *testing.T, cache *Cache, bookID string, source *bookpkg.Source) []string {
	t.Helper()
	hashes, err := cache.Resolve(bookID, source)
	if err != nil {
		t.Fatalf("Resolve(%s): %v", bookID, err)
	}
	return hashes
}

func TestResolveIgnoresNamesAndOrder(t *testing.T) {
	ts := newTestShelf(t)
	a := ts.addComic("a", testPage{"001.jpg", "one"}, testPage{"002.jpg", "two"})
	b := ts.addComic("b", testPage{"zz.png", "two"}, testPage{"aa.png", "one"})
	cache := ts.open(ts.base)

	if got, want := mustResolve(t, cache, "a", a), mustResolve(t, cache, "b", b); !slices.Equal(got, want) {
		t.Fatalf("same images under other names and order hashed differently: %v vs %v", got, want)
	}
}

func TestResolveSameNamesOtherImages(t *testing.T) {
	ts := newTestShelf(t)
	a := ts.addComic("a", testPage{"001.jpg", "one"})
	b := ts.addComic("b", testPage{"001.jpg", "other"})
	cache := ts.open(ts.base)

	if got, other := mustResolve(t, cache, "a", a), mustResolve(t, cache, "b", b); slices.Equal(got, other) {
		t.Fatalf("different images under the same name hashed alike: %v", got)
	}
}

func TestReopenedCacheReadsNoImage(t *testing.T) {
	ts := newTestShelf(t)
	ts.addComic("a", testPage{"001.jpg", "one"}, testPage{"002.jpg", "two"})
	first := ts.open(ts.base)
	want := mustResolve(t, first, "a", ts.reopenSource(ts.base, "a"))
	if err := first.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	counting := &countingFS{FS: ts.base}
	second := ts.open(counting)
	got := mustResolve(t, second, "a", ts.reopenSource(counting, "a"))

	if n := counting.assetOpens.Load(); n != 0 {
		t.Fatalf("second run opened %d images, want 0", n)
	}
	if !slices.Equal(got, want) {
		t.Fatalf("hashes changed across runs: %v vs %v", got, want)
	}
	if stats := second.Stats(); stats.StatHits != 2 || stats.Hashed != 0 {
		t.Fatalf("stats = %+v, want 2 stat hits and nothing hashed", stats)
	}
}

func TestChangedPageIsRehashed(t *testing.T) {
	ts := newTestShelf(t)
	source := ts.addComic("a", testPage{"001.jpg", "one"})
	cache := ts.open(ts.base)
	before := mustResolve(t, cache, "a", source)

	ts.writePage(source, testPage{"001.jpg", "replaced"})
	after := mustResolve(t, cache, "a", source)

	if slices.Equal(before, after) {
		t.Fatalf("a page rewritten with other bytes kept its hash %v", before)
	}
}

func TestRecordedNeedsSamePageList(t *testing.T) {
	ts := newTestShelf(t)
	source := ts.addComic("a", testPage{"001.jpg", "one"})
	cache := ts.open(ts.base)
	want := mustResolve(t, cache, "a", source)
	listMD5 := source.GetMeta().MD5Hash

	if got, ok := cache.Recorded("a", source.ID(), listMD5); !ok || !slices.Equal(got, want) {
		t.Fatalf("Recorded = %v, %v; want %v", got, ok, want)
	}
	if _, ok := cache.Recorded("a", source.ID(), "other-list"); ok {
		t.Fatal("Recorded answered for a page list it never hashed")
	}
}

func TestSavePrunesDeletedBooks(t *testing.T) {
	ts := newTestShelf(t)
	a := ts.addComic("a", testPage{"001.jpg", "one"})
	b := ts.addComic("b", testPage{"001.jpg", "two"})
	cache := ts.open(ts.base)
	mustResolve(t, cache, "a", a)
	mustResolve(t, cache, "b", b)

	delete(ts.live, "b")
	if err := cache.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	reopened := ts.open(ts.base)
	if _, ok := reopened.Recorded("b", b.ID(), b.GetMeta().MD5Hash); ok {
		t.Fatal("a deleted book's page hashes survived Save")
	}
	if _, ok := reopened.Recorded("a", a.ID(), a.GetMeta().MD5Hash); !ok {
		t.Fatal("a live book's page hashes were pruned")
	}
}

func TestOpenDiscardsOtherSchema(t *testing.T) {
	ts := newTestShelf(t)
	source := ts.addComic("a", testPage{"001.jpg", "one"})
	stale := `{"schema_version":99,"hash":"md5","sources":{"a/` + source.ID() +
		`":{"list_md5":"` + source.GetMeta().MD5Hash + `","pages":{"001.jpg":{"size":3,"md5":"x"}}}}}`
	if err := ts.base.WriteFile("app/"+cacheFileName, []byte(stale)); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if _, ok := ts.open(ts.base).Recorded("a", source.ID(), source.GetMeta().MD5Hash); ok {
		t.Fatal("a cache file of another schema_version was read")
	}
}

func TestResolveRefusesMissingPages(t *testing.T) {
	ts := newTestShelf(t)
	source := ts.addComic("a", testPage{"001.jpg", "one"})
	cache := ts.open(ts.base)
	mustResolve(t, cache, "a", source)

	if err := source.UpdateContent(strings.NewReader("001.jpg\n002.jpg\n")); err != nil {
		t.Fatalf("UpdateContent: %v", err)
	}

	if _, err := cache.Resolve("a", source); !errors.Is(err, ErrMissingPages) {
		t.Fatalf("Resolve = %v, want ErrMissingPages for a listed page with no image", err)
	}
	if _, ok := cache.Recorded("a", source.ID(), source.GetMeta().MD5Hash); ok {
		t.Fatal("a comic with a missing page kept a recorded page set")
	}
}

func TestSaveKeepsNewerUntouchedRecords(t *testing.T) {
	ts := newTestShelf(t)
	a := ts.addComic("a", testPage{"001.jpg", "one"})
	b := ts.addComic("b", testPage{"001.jpg", "two"})
	seed := ts.open(ts.base)
	mustResolve(t, seed, "a", a)
	mustResolve(t, seed, "b", b)
	if err := seed.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	stale := ts.open(ts.base)
	fresh := ts.open(ts.base)
	ts.writePage(b, testPage{"001.jpg", "changed"})
	want := mustResolve(t, fresh, "b", b)
	if err := fresh.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}
	mustResolve(t, stale, "a", a)
	if err := stale.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, ok := ts.open(ts.base).Recorded("b", b.ID(), b.GetMeta().MD5Hash)
	if !ok || !slices.Equal(got, want) {
		t.Fatalf("Recorded(b) = %v, %v; want the other run's newer %v", got, ok, want)
	}
}
