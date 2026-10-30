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

const appDir = "app"

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

func repairSourceHash(source *bookpkg.Source, contentMD5 string) (bool, error) {
	return source.RepairContentHash(contentMD5)
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
		t.Fatalf("OpenRoot(%q): %v", libRoot, err)
	}
	t.Cleanup(func() {
		if err := osRoot.Close(); err != nil {
			t.Errorf("closing shelf root: %v", err)
		}
	})

	base := fsutil.NewRootFS(osRoot)
	if err := base.MkdirAll(appDir); err != nil {
		t.Fatalf("creating %s/: %v", appDir, err)
	}

	return &testShelf{t: t, libRoot: libRoot, base: base, live: map[string]struct{}{}}
}

func (ts *testShelf) liveBooks() (map[string]struct{}, bool) {
	return ts.live, true
}

func (ts *testShelf) addBook(dir, id, title, content string, modTimeOffset time.Duration) *bookpkg.Book {
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

	if modTimeOffset != 0 {
		shiftModTime(t, ts.sourceFilePath(book), modTimeOffset)
	}
	return book
}

func (ts *testShelf) countSourceReads() *countingFS {
	return &countingFS{FS: ts.base}
}

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

type countingFS struct {
	fsutil.FS
	opens atomic.Int64
	stats atomic.Int64
}

func (f *countingFS) Open(name string) (fs.File, error) {
	if path.Base(name) == bookpkg.SourceFile {
		f.opens.Add(1)
	}
	return f.FS.Open(name)
}

func (f *countingFS) Stat(name string) (fs.FileInfo, error) {
	f.stats.Add(1)
	return f.FS.Stat(name)
}

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

func resolveFingerprint(t *testing.T, cache *Cache, root fsutil.ReadFS, bookPath string, builder *fakeFingerprint) Entry {
	t.Helper()

	book, source := reopen(t, root, bookPath)

	entry, err := cache.Resolve(book, source, builder.build)
	if err != nil {
		t.Fatalf("Resolve(%q): %v", book.Title(), err)
	}
	return entry
}

func shiftModTime(t *testing.T, filePath string, offset time.Duration) {
	t.Helper()

	at := time.Now().Add(offset)
	if err := os.Chtimes(filePath, at, at); err != nil {
		t.Fatalf("shifting mod time of %s: %v", filePath, err)
	}
}

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

type rewriteAfterReadFS struct {
	fsutil.FS
	rewrite func()
	done    atomic.Bool
}

func (f *rewriteAfterReadFS) Open(name string) (fs.File, error) {
	file, err := f.FS.Open(name)
	if err != nil || path.Base(name) != bookpkg.SourceFile {
		return file, err
	}
	return closeHookFile{File: file, onClose: func() {
		if f.done.CompareAndSwap(false, true) {
			f.rewrite()
		}
	}}, nil
}

type closeHookFile struct {
	fs.File
	onClose func()
}

func (c closeHookFile) Close() error {
	err := c.File.Close()
	c.onClose()
	return err
}
