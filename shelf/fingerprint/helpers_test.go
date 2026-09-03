package fingerprint

import (
	"encoding/json/v2"
	"io/fs"
	"os"
	"path"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/voilelab/plainshelf/internal/appcache"
	"github.com/voilelab/plainshelf/internal/fsutil"
	"github.com/voilelab/plainshelf/internal/hashutil"
	"github.com/voilelab/plainshelf/internal/logutil"
	"github.com/voilelab/plainshelf/shelf/bookpkg"
)

// appDir is where the cache file lives, standing in for the shelf's app/.
const appDir = "app"

// testAlgo is what a fingerprint task would pass in. The values are opaque to
// the cache, which is the point: it stores answers, it does not compute them.
var testAlgo = Algo{
	Normalize: "nfkc-strip-space-punct-v1",
	Shingle:   "char-5gram",
	Hash:      "xxhash64",
	K:         128,
}

func newLoggerForTest() logutil.Logger {
	logger, err := logutil.NewLogger(&logutil.LogConf{
		LogFile: logutil.LogFileConf{Type: logutil.LogFileTypeNone},
	})
	if err != nil {
		panic("failed to create logger for test: " + err.Error())
	}
	return *logger
}

// repairSourceHash is the RepairHash hook the tests wire in, the same one the
// shelf facade uses: it delegates the write to the source itself, so a read-only
// source reports fsutil.ErrReadOnly and the cache leaves it alone.
func repairSourceHash(source *bookpkg.Source, contentMD5 string) (bool, error) {
	return source.RepairContentHash(contentMD5)
}

// testShelf is a bare directory standing in for a shelf: an app/ dir for the
// cache file, books created directly through bookpkg, and a live-book set the
// test controls. It is everything the cache needs from a shelf and nothing it
// does not - no scan, no lock, no *Shelf.
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
		t.Fatalf("OpenRoot(%q): %v", libRoot, err)
	}
	t.Cleanup(func() { osRoot.Close() }) //nolint:errcheck // best-effort test cleanup

	base := fsutil.NewRootFS(osRoot)
	if err := base.MkdirAll(appDir); err != nil {
		t.Fatalf("creating %s/: %v", appDir, err)
	}

	return &testShelf{t: t, libRoot: libRoot, base: base, live: map[string]struct{}{}}
}

// liveBooks is the LiveBooks hook: the cache prunes against whatever set the
// test has declared live, and the test drops a book from it to model a delete.
func (ts *testShelf) liveBooks() (map[string]struct{}, bool) {
	return ts.live, true
}

// addBook creates a book holding one source at dir, under the given ID, and
// backdates that source's modification time by age.
//
// The backdating is not cosmetic: a source written moments ago is deliberately
// left out of the index, because a coarse filesystem clock cannot tell a second
// write inside the same tick from no write at all (see fsutil.RacyWindow). A
// book being fingerprinted in the real world is older than that; a book created
// by a test is not. Pass age 0 to leave a source racily fresh on purpose.
func (ts *testShelf) addBook(dir, id, title, content string, age time.Duration) *bookpkg.Book {
	t := ts.t
	t.Helper()

	book, err := bookpkg.Create(ts.base, newLoggerForTest(), dir, id, title)
	if err != nil {
		t.Fatalf("Create(%q): %v", dir, err)
	}
	source, err := book.NewSource(strings.NewReader(content))
	if err != nil {
		t.Fatalf("NewSource: %v", err)
	}
	if err := book.SetCurrentSource(source.ID()); err != nil {
		t.Fatalf("SetCurrentSource: %v", err)
	}

	ts.live[id] = struct{}{}

	if age != 0 {
		shiftModTime(t, ts.sourceFilePath(book), age)
	}
	return book
}

// countSourceReads returns a filesystem that counts how often a source.txt is
// opened for reading - the cost this cache exists to avoid - which the cache and
// the reopened books then read through. Books are created on the plain base
// handle first, so only the I/O a test causes past this point is counted.
func (ts *testShelf) countSourceReads() *countingFS {
	return &countingFS{FS: ts.base}
}

// sourceFilePath is where a book's only source.txt really lives on disk, for the
// tests that reach past PlainShelf and edit the shelf the way a user would.
func (ts *testShelf) sourceFilePath(book *bookpkg.Book) string {
	t := ts.t
	t.Helper()

	sources, err := book.ListSource()
	if err != nil {
		t.Fatalf("ListSource: %v", err)
	}
	if len(sources) != 1 {
		t.Fatalf("expected exactly one source, got %d", len(sources))
	}
	return path.Join(ts.libRoot, sources[0].FolderPath(), bookpkg.SourceFile)
}

// countingFS counts how often a source.txt is opened, which is the cost this
// cache exists to avoid. Reads of meta.json, book.json and the cache file are
// not counted: they are what opening a book and loading the cache cost with or
// without a fingerprint cache.
type countingFS struct {
	fsutil.FS
	opens atomic.Int64
}

func (f *countingFS) Open(name string) (fs.File, error) {
	if path.Base(name) == bookpkg.SourceFile {
		f.opens.Add(1)
	}
	return f.FS.Open(name)
}

// fakeFingerprint stands in for the sketch builder. It counts its calls, which
// is how the tests tell a cache hit from a recomputation, and derives every
// field from the content so that two copies of one book produce one entry.
type fakeFingerprint struct {
	label string
	calls atomic.Int64
}

func (f *fakeFingerprint) build(content []byte) (Entry, error) {
	f.calls.Add(1)

	sum, err := hashutil.MD5Hash(strings.NewReader(normalizeForTest(string(content))))
	if err != nil {
		return Entry{}, err
	}

	return Entry{
		NormMD5:   sum,
		NormChars: len(normalizeForTest(string(content))),
		Shingles:  len(content),
		Sketch:    f.label + ":" + sum,
	}, nil
}

func normalizeForTest(s string) string {
	return strings.ReplaceAll(s, " ", "")
}

// openCache opens a cache over root, wired to the test shelf the way the shelf
// facade wires it to a real one.
func openCache(t *testing.T, ts *testShelf, root fsutil.ReadFS, algo Algo) *Cache {
	t.Helper()

	cache, err := Open(Config{
		Store:      appcache.NewFSStore(root, appDir),
		Algo:       algo,
		Logger:     newLoggerForTest(),
		LiveBooks:  ts.liveBooks,
		RepairHash: repairSourceHash,
	})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	return cache
}

func saveCache(t *testing.T, cache *Cache) {
	t.Helper()

	if err := cache.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}
}

// reopen returns a fresh book and source read through root, standing in for the
// next run of a fingerprint task: nothing is carried over in memory, so every
// read the cache avoids is a read that really did not happen.
func reopen(t *testing.T, root fsutil.ReadFS, bookPath string) (*bookpkg.Book, *bookpkg.Source) {
	t.Helper()

	book, err := bookpkg.Open(root, newLoggerForTest(), bookPath)
	if err != nil {
		t.Fatalf("bookpkg.Open(%q): %v", bookPath, err)
	}

	sources, err := book.ListSource()
	if err != nil {
		t.Fatalf("ListSource: %v", err)
	}
	if len(sources) != 1 {
		t.Fatalf("expected exactly one source under %q, got %d", bookPath, len(sources))
	}

	return book, sources[0]
}

// resolveFingerprint resolves a book that is opened fresh for the call, the way
// a new run of a fingerprint task would see it.
func resolveFingerprint(t *testing.T, cache *Cache, root fsutil.ReadFS, bookPath string, builder *fakeFingerprint) Entry {
	t.Helper()

	book, source := reopen(t, root, bookPath)

	entry, err := cache.Resolve(book, source, builder.build)
	if err != nil {
		t.Fatalf("Resolve(%q): %v", book.Title(), err)
	}
	return entry
}

// shiftModTime moves a file's modification time by offset from now.
func shiftModTime(t *testing.T, filePath string, offset time.Duration) {
	t.Helper()

	at := time.Now().Add(offset)
	if err := os.Chtimes(filePath, at, at); err != nil {
		t.Fatalf("shifting mod time of %s: %v", filePath, err)
	}
}

// shiftedModTime backdates a file and reports the time it was given, so a test
// can tell "not rewritten" from "rewritten with the same bytes".
func shiftedModTime(t *testing.T, filePath string, offset time.Duration) time.Time {
	t.Helper()

	shiftModTime(t, filePath, offset)

	info, err := os.Stat(filePath)
	if err != nil {
		t.Fatalf("stat %s: %v", filePath, err)
	}
	return info.ModTime()
}

func readCacheAt(t *testing.T, libRoot string) cacheFile {
	t.Helper()

	data, err := os.ReadFile(path.Join(libRoot, appDir, cacheFileName))
	if err != nil {
		t.Fatalf("reading the fingerprint cache: %v", err)
	}

	var stored cacheFile
	if err := json.Unmarshal(data, &stored); err != nil {
		t.Fatalf("decoding the fingerprint cache: %v", err)
	}
	return stored
}

func writeCacheAt(t *testing.T, libRoot string, stored cacheFile) {
	t.Helper()

	data, err := json.Marshal(stored)
	if err != nil {
		t.Fatalf("encoding a fingerprint cache: %v", err)
	}
	if err := os.WriteFile(path.Join(libRoot, appDir, cacheFileName), data, 0644); err != nil {
		t.Fatalf("writing a fingerprint cache: %v", err)
	}
}

// metaMatchesContent reports whether a source's stored md5_hash agrees with the
// bytes on disk. Source.VerifyContent used to answer this; production never
// asked, because a caller that has just read the file already knows.
func metaMatchesContent(t *testing.T, source *bookpkg.Source) bool {
	t.Helper()
	f, err := source.Open()
	if err != nil {
		t.Fatalf("Open source content: %v", err)
	}
	defer f.Close()
	sum, err := hashutil.MD5Hash(f)
	if err != nil {
		t.Fatalf("MD5Hash: %v", err)
	}
	return sum == source.GetMeta().MD5Hash
}
