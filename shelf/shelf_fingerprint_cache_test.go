package shelf

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/voilelab/plainshelf/internal/fsutil"
	"github.com/voilelab/plainshelf/internal/hashutil"
	"github.com/voilelab/plainshelf/shelf/bookpkg"
)

// testAlgo is what a fingerprint task would pass in. The values are opaque to
// the cache, which is the point: it stores answers, it does not compute them.
var testAlgo = FingerprintAlgo{
	Normalize: "nfkc-strip-space-punct-v1",
	Shingle:   "char-5gram",
	Hash:      "xxhash64",
	K:         128,
}

// countingSourceFS counts how often a source.txt is opened, which is the cost
// this cache exists to avoid. Reads of meta.json and book.json are not counted:
// they are what opening a book costs with or without a fingerprint cache.
type countingSourceFS struct {
	fsutil.FS
	opens atomic.Int64
}

func (f *countingSourceFS) Open(name string) (fs.File, error) {
	if path.Base(name) == SourceFile {
		f.opens.Add(1)
	}
	return f.FS.Open(name)
}

// countSourceReads swaps a counting filesystem into a shelf whose initial scan
// has already finished, so only the I/O a test causes is counted. The shelf
// must have no book cache writer ID, or the export started by that scan would
// still be running. Compare countBookTreeReads.
func countSourceReads(t *testing.T, s *Shelf) *countingSourceFS {
	t.Helper()

	if s.bookCacheWriterID != "" {
		t.Fatal("countSourceReads needs a shelf without a book cache writer ID")
	}

	counting := &countingSourceFS{FS: writeRootForTest(t, s)}
	s.dbRoot = counting
	return counting
}

// fakeFingerprint stands in for the sketch builder. It counts its calls, which
// is how the tests tell a cache hit from a recomputation, and derives every
// field from the content so that two copies of one book produce one entry.
type fakeFingerprint struct {
	label string
	calls atomic.Int64
}

func (f *fakeFingerprint) build(content []byte) (FingerprintEntry, error) {
	f.calls.Add(1)

	sum, err := hashutil.MD5Hash(strings.NewReader(normalizeForTest(string(content))))
	if err != nil {
		return FingerprintEntry{}, err
	}

	return FingerprintEntry{
		NormMD5:   sum,
		NormChars: len(normalizeForTest(string(content))),
		Shingles:  len(content),
		Sketch:    f.label + ":" + sum,
	}, nil
}

func normalizeForTest(s string) string {
	return strings.ReplaceAll(s, " ", "")
}

// addFingerprintBook creates a book holding one source and backdates that
// source's modification time.
//
// The backdating is not cosmetic: a source written moments ago is deliberately
// left out of the index, because a coarse filesystem clock cannot tell a second
// write inside the same tick from no write at all (see
// fingerprintCacheRacyWindow). A book being fingerprinted in the real world is
// older than that; a book created by a test is not.
func addFingerprintBook(t *testing.T, s *Shelf, libRoot, title, content string) *Book {
	t.Helper()

	book, err := s.NewBookWith(nil, title, func(book *Book) error {
		source, err := book.NewSource(strings.NewReader(content))
		if err != nil {
			return err
		}
		return book.SetCurrentSource(source.ID())
	})
	if err != nil {
		t.Fatalf("NewBookWith(%q): %v", title, err)
	}

	shiftModTime(t, sourceFilePath(t, libRoot, book), -time.Hour)
	return book
}

// sourceFilePath is where a book's only source.txt really lives, for the tests
// that have to reach past PlainShelf and edit the shelf the way a user would.
func sourceFilePath(t *testing.T, libRoot string, book *Book) string {
	t.Helper()

	sources, err := book.ListSource()
	if err != nil {
		t.Fatalf("ListSource: %v", err)
	}
	if len(sources) != 1 {
		t.Fatalf("expected exactly one source, got %d", len(sources))
	}

	return path.Join(libRoot, sources[0].FolderPath(), SourceFile)
}

// reopen returns a fresh book and source read through root, standing in for the
// next run of a fingerprint task: nothing is carried over in memory, so every
// read the cache avoids is a read that really did not happen.
func reopen(t *testing.T, root fsutil.ReadFS, bookPath string) (*Book, *Source) {
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
func resolveFingerprint(t *testing.T, cache *FingerprintCache, root fsutil.ReadFS, bookPath string, builder *fakeFingerprint) FingerprintEntry {
	t.Helper()

	book, source := reopen(t, root, bookPath)

	entry, err := cache.Resolve(book, source, builder.build)
	if err != nil {
		t.Fatalf("Resolve(%q): %v", book.Title(), err)
	}
	return entry
}

func openFingerprintCache(t *testing.T, s *Shelf, algo FingerprintAlgo) *FingerprintCache {
	t.Helper()

	cache, err := s.OpenFingerprintCache(algo)
	if err != nil {
		t.Fatalf("OpenFingerprintCache: %v", err)
	}
	return cache
}

func saveFingerprintCache(t *testing.T, cache *FingerprintCache) {
	t.Helper()

	if err := cache.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}
}

func readFingerprintCacheAt(t *testing.T, libRoot string) fingerprintCacheFile {
	t.Helper()

	data, err := os.ReadFile(path.Join(libRoot, appFolder, fingerprintCacheFileName))
	if err != nil {
		t.Fatalf("reading the fingerprint cache: %v", err)
	}

	var stored fingerprintCacheFile
	if err := json.Unmarshal(data, &stored); err != nil {
		t.Fatalf("decoding the fingerprint cache: %v", err)
	}
	return stored
}

func writeFingerprintCacheAt(t *testing.T, libRoot string, stored fingerprintCacheFile) {
	t.Helper()

	data, err := json.Marshal(stored)
	if err != nil {
		t.Fatalf("encoding a fingerprint cache: %v", err)
	}
	if err := os.WriteFile(path.Join(libRoot, appFolder, fingerprintCacheFileName), data, 0644); err != nil {
		t.Fatalf("writing a fingerprint cache: %v", err)
	}
}

// The point of the whole file: a second run over an unchanged shelf opens no
// source at all. The first run reads each source once, the second reads none.
func TestFingerprintCacheAnswersUnchangedSourcesWithoutReadingThem(t *testing.T) {
	libRoot := t.TempDir()
	shelf := newTestShelf(t, &ShelfConf{LibRoot: libRoot, LockMode: "none"})

	book := addFingerprintBook(t, shelf, libRoot, "Dune", "the spice must flow")
	bookPath := book.FolderPath()

	counting := countSourceReads(t, shelf)
	builder := &fakeFingerprint{label: "v1"}

	first := openFingerprintCache(t, shelf, testAlgo)
	firstEntry := resolveFingerprint(t, first, counting, bookPath, builder)
	saveFingerprintCache(t, first)

	if got := counting.opens.Load(); got != 1 {
		t.Fatalf("the first run opened source.txt %d times, want 1", got)
	}
	if got := builder.calls.Load(); got != 1 {
		t.Fatalf("the first run built %d fingerprints, want 1", got)
	}

	// A fresh cache, loaded from disk: the second run shares nothing with the
	// first but the file.
	second := openFingerprintCache(t, shelf, testAlgo)
	secondEntry := resolveFingerprint(t, second, counting, bookPath, builder)

	if got := counting.opens.Load(); got != 1 {
		t.Errorf("the second run opened source.txt, total opens %d, want 1", got)
	}
	if got := builder.calls.Load(); got != 1 {
		t.Errorf("the second run built a fingerprint, total builds %d, want 1", got)
	}
	if secondEntry != firstEntry {
		t.Errorf("the cached fingerprint is %+v, want %+v", secondEntry, firstEntry)
	}
	if got := second.Stats().StatHits; got != 1 {
		t.Errorf("stat hits %d, want 1", got)
	}

	// And saving that run rewrites nothing: an unchanged shelf must not cost a
	// pointless upload on the transports this file will live on.
	cachePath := path.Join(libRoot, appFolder, fingerprintCacheFileName)
	before, err := os.ReadFile(cachePath)
	if err != nil {
		t.Fatalf("reading the fingerprint cache: %v", err)
	}
	writtenAt := shiftedModTime(t, cachePath, -time.Hour)

	saveFingerprintCache(t, second)

	after, err := os.ReadFile(cachePath)
	if err != nil {
		t.Fatalf("re-reading the fingerprint cache: %v", err)
	}
	if string(after) != string(before) {
		t.Error("saving an unchanged run rewrote the cache")
	}
	if info, err := os.Stat(cachePath); err != nil {
		t.Fatalf("stat: %v", err)
	} else if !info.ModTime().Equal(writtenAt) {
		t.Error("saving an unchanged run replaced the cache file")
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

// A book moved between layers keeps its fingerprints: the index is keyed on the
// book ID, which survives the move, and not on the path, which does not.
func TestFingerprintCacheSurvivesMovingABook(t *testing.T) {
	libRoot := t.TempDir()
	shelf := newTestShelf(t, &ShelfConf{LibRoot: libRoot, LockMode: "none"})

	book := addFingerprintBook(t, shelf, libRoot, "Dune", "the spice must flow")

	counting := countSourceReads(t, shelf)
	builder := &fakeFingerprint{label: "v1"}

	first := openFingerprintCache(t, shelf, testAlgo)
	before := resolveFingerprint(t, first, counting, book.FolderPath(), builder)
	saveFingerprintCache(t, first)

	moved, err := shelf.MoveBook(book.ID(), Layers{"Fiction", "Classics"})
	if err != nil {
		t.Fatalf("MoveBook: %v", err)
	}
	if moved.FolderPath() == book.FolderPath() {
		t.Fatalf("the book was not moved: still at %q", moved.FolderPath())
	}

	opensBefore := counting.opens.Load()
	second := openFingerprintCache(t, shelf, testAlgo)
	after := resolveFingerprint(t, second, counting, moved.FolderPath(), builder)

	if got := counting.opens.Load(); got != opensBefore {
		t.Errorf("the moved book was read again: opens %d, want %d", got, opensBefore)
	}
	if got := builder.calls.Load(); got != 1 {
		t.Errorf("the moved book was fingerprinted again: builds %d, want 1", got)
	}
	if after != before {
		t.Errorf("the moved book resolved to %+v, want %+v", after, before)
	}
}

// A book copied into the shelf has a new ID and a new stat, so the index cannot
// answer - but its content is already known, so it costs one read and no
// fingerprinting.
func TestFingerprintCacheRecognizesACopiedBookByItsContent(t *testing.T) {
	libRoot := t.TempDir()
	shelf := newTestShelf(t, &ShelfConf{LibRoot: libRoot, LockMode: "none"})

	const content = "the spice must flow"
	original := addFingerprintBook(t, shelf, libRoot, "Dune", content)
	copied := addFingerprintBook(t, shelf, libRoot, "Dune (copy)", content)

	if original.ID() == copied.ID() {
		t.Fatal("the copy shares the original's book ID")
	}

	counting := countSourceReads(t, shelf)
	builder := &fakeFingerprint{label: "v1"}
	cache := openFingerprintCache(t, shelf, testAlgo)

	first := resolveFingerprint(t, cache, counting, original.FolderPath(), builder)
	opensAfterOriginal := counting.opens.Load()

	second := resolveFingerprint(t, cache, counting, copied.FolderPath(), builder)

	if got := counting.opens.Load(); got != opensAfterOriginal+1 {
		t.Errorf("the copy cost %d reads, want 1", got-opensAfterOriginal)
	}
	if got := builder.calls.Load(); got != 1 {
		t.Errorf("the copy was fingerprinted again: builds %d, want 1", got)
	}
	if second != first {
		t.Errorf("the copy resolved to %+v, want %+v", second, first)
	}
	if got := cache.Stats().ContentHits; got != 1 {
		t.Errorf("content hits %d, want 1", got)
	}

	saveFingerprintCache(t, cache)
	stored := readFingerprintCacheAt(t, libRoot)
	if len(stored.Entries) != 1 {
		t.Errorf("two identical books stored %d entries, want 1", len(stored.Entries))
	}
	if len(stored.Index) != 2 {
		t.Errorf("two books stored %d index records, want 2", len(stored.Index))
	}
}

// A source edited outside PlainShelf fails the stat check, is fingerprinted
// again, and has the md5_hash in its meta.json - which nothing else validates -
// repaired from the content that was just read.
func TestFingerprintCacheRepairsAStaleSourceHash(t *testing.T) {
	libRoot := t.TempDir()
	shelf := newTestShelf(t, &ShelfConf{LibRoot: libRoot, LockMode: "none"})

	book := addFingerprintBook(t, shelf, libRoot, "Dune", "the spice must flow")
	bookPath := book.FolderPath()
	sourcePath := sourceFilePath(t, libRoot, book)

	counting := countSourceReads(t, shelf)
	builder := &fakeFingerprint{label: "v1"}

	cache := openFingerprintCache(t, shelf, testAlgo)
	before := resolveFingerprint(t, cache, counting, bookPath, builder)
	saveFingerprintCache(t, cache)

	const edited = "the spice must flow, and then some"
	if err := os.WriteFile(sourcePath, []byte(edited), 0644); err != nil {
		t.Fatalf("editing source.txt: %v", err)
	}
	shiftModTime(t, sourcePath, -30*time.Minute)

	_, staleSource := reopen(t, counting, bookPath)
	if ok, err := staleSource.VerifyContent(); err != nil || ok {
		t.Fatalf("the edited source should carry a stale hash: ok=%v err=%v", ok, err)
	}

	second := openFingerprintCache(t, shelf, testAlgo)
	after := resolveFingerprint(t, second, counting, bookPath, builder)

	if got := builder.calls.Load(); got != 2 {
		t.Errorf("the edited source was not fingerprinted again: builds %d, want 2", got)
	}
	if after == before {
		t.Error("the edited source resolved to the fingerprint of its old content")
	}
	if got := second.Stats().Repaired; got != 1 {
		t.Errorf("repaired %d hashes, want 1", got)
	}

	_, repaired := reopen(t, counting, bookPath)
	if ok, err := repaired.VerifyContent(); err != nil || !ok {
		t.Errorf("meta.json still disagrees with the content: ok=%v err=%v", ok, err)
	}
}

// A cache built with other rules is discarded whole rather than partly reused:
// nothing can vouch for the comparability of an entry whose normalization or
// sketch parameters are not the ones in use.
func TestFingerprintCacheIsDiscardedWhenTheAlgorithmChanges(t *testing.T) {
	libRoot := t.TempDir()
	shelf := newTestShelf(t, &ShelfConf{LibRoot: libRoot, LockMode: "none"})

	book := addFingerprintBook(t, shelf, libRoot, "Dune", "the spice must flow")
	bookPath := book.FolderPath()

	counting := countSourceReads(t, shelf)
	oldBuilder := &fakeFingerprint{label: "v1"}

	first := openFingerprintCache(t, shelf, testAlgo)
	resolveFingerprint(t, first, counting, bookPath, oldBuilder)
	saveFingerprintCache(t, first)

	newAlgo := testAlgo
	newAlgo.Normalize = "nfkc-strip-space-punct-v2"
	newBuilder := &fakeFingerprint{label: "v2"}

	second := openFingerprintCache(t, shelf, newAlgo)
	rebuilt := resolveFingerprint(t, second, counting, bookPath, newBuilder)

	if got := newBuilder.calls.Load(); got != 1 {
		t.Errorf("the new algorithm reused a stored fingerprint: builds %d, want 1", got)
	}
	saveFingerprintCache(t, second)

	stored := readFingerprintCacheAt(t, libRoot)
	if stored.Algo != newAlgo {
		t.Errorf("the stored algorithm is %+v, want %+v", stored.Algo, newAlgo)
	}
	if len(stored.Entries) != 1 {
		t.Fatalf("the rewritten cache holds %d entries, want 1", len(stored.Entries))
	}
	for _, entry := range stored.Entries {
		if entry != rebuilt {
			t.Errorf("the rewritten cache kept %+v, want %+v", entry, rebuilt)
		}
	}
}

// Two machines sharing a shelf must not overwrite each other: entries are a
// union, so neither side loses work it computed.
func TestFingerprintCacheMergesAnotherWritersEntries(t *testing.T) {
	libRoot := t.TempDir()
	shelf := newTestShelf(t, &ShelfConf{LibRoot: libRoot, LockMode: "none"})

	mine := addFingerprintBook(t, shelf, libRoot, "Mine", "a book only this machine has read")
	theirs := addFingerprintBook(t, shelf, libRoot, "Theirs", "a book only the other machine has read")

	counting := countSourceReads(t, shelf)
	builder := &fakeFingerprint{label: "v1"}

	cache := openFingerprintCache(t, shelf, testAlgo)
	resolveFingerprint(t, cache, counting, mine.FolderPath(), builder)
	saveFingerprintCache(t, cache)

	// What the other machine wrote in the meantime: its own book, plus a later
	// look at the book this one already knows.
	_, theirSource := reopen(t, counting, theirs.FolderPath())
	theirStat, err := theirSource.ContentStat()
	if err != nil {
		t.Fatalf("contentStat: %v", err)
	}
	theirKey := fingerprintIndexKey(theirs.ID(), theirSource.ID())
	const theirMD5 = "0123456789abcdef0123456789abcdef"

	stored := readFingerprintCacheAt(t, libRoot)
	myKey := fingerprintIndexKey(mine.ID(), func() string {
		_, source := reopen(t, counting, mine.FolderPath())
		return source.ID()
	}())
	myRecord, ok := stored.Index[myKey]
	if !ok {
		t.Fatalf("the saved cache has no record for %q", myKey)
	}
	newer := myRecord
	newer.ModTime = myRecord.ModTime.Add(time.Minute)

	stored.Index[myKey] = newer
	stored.Index[theirKey] = fingerprintIndexEntry{Size: theirStat.Size, ModTime: theirStat.ModTime, MD5: theirMD5}
	stored.Entries[theirMD5] = FingerprintEntry{NormMD5: theirMD5, NormChars: 7, Shingles: 7, Sketch: "theirs"}
	writeFingerprintCacheAt(t, libRoot, stored)

	// Any later save has to fold that in rather than replace it.
	saveFingerprintCache(t, cache)

	merged := readFingerprintCacheAt(t, libRoot)
	if len(merged.Entries) != 2 {
		t.Errorf("the merged cache holds %d entries, want 2", len(merged.Entries))
	}
	if got := merged.Entries[theirMD5].Sketch; got != "theirs" {
		t.Errorf("the other machine's entry is %q, want %q", got, "theirs")
	}
	if _, ok := merged.Index[theirKey]; !ok {
		t.Errorf("the other machine's index record for %q was dropped", theirKey)
	}
	if got := merged.Index[myKey].ModTime; !got.Equal(newer.ModTime) {
		t.Errorf("the shared index record kept mtime %v, want the newer %v", got, newer.ModTime)
	}
}

// The newer-observation rule on its own, including the case a whole-file test
// cannot stage: the older record arriving second.
func TestMergeFingerprintIndexKeepsTheNewerRecord(t *testing.T) {
	older := time.Date(2026, 3, 15, 8, 30, 0, 0, time.UTC)
	newer := older.Add(time.Hour)

	index := map[string]fingerprintIndexEntry{
		"book-a/source-1": {Size: 1, ModTime: newer, MD5: "newer"},
		"book-b/source-1": {Size: 2, ModTime: older, MD5: "older"},
	}
	other := map[string]fingerprintIndexEntry{
		"book-a/source-1": {Size: 3, ModTime: older, MD5: "older"},
		"book-b/source-1": {Size: 4, ModTime: newer, MD5: "newer"},
		"book-c/source-1": {Size: 5, ModTime: older, MD5: "only-theirs"},
	}

	mergeFingerprintIndex(index, other)

	for key, want := range map[string]string{
		"book-a/source-1": "newer",
		"book-b/source-1": "newer",
		"book-c/source-1": "only-theirs",
	} {
		if got := index[key].MD5; got != want {
			t.Errorf("%s merged to %q, want %q", key, got, want)
		}
	}
}

// Records for books the shelf no longer holds are collected on the next save,
// along with the entries nothing points at any more. An entry a surviving book
// still hashes to stays.
func TestFingerprintCachePrunesDeletedBooks(t *testing.T) {
	libRoot := t.TempDir()
	shelf := newTestShelf(t, &ShelfConf{LibRoot: libRoot, LockMode: "none"})

	const shared = "a text two books hold"
	kept := addFingerprintBook(t, shelf, libRoot, "Kept", shared)
	alsoKept := addFingerprintBook(t, shelf, libRoot, "Also Kept", shared)
	doomed := addFingerprintBook(t, shelf, libRoot, "Doomed", "a text only one book holds")

	counting := countSourceReads(t, shelf)
	builder := &fakeFingerprint{label: "v1"}
	cache := openFingerprintCache(t, shelf, testAlgo)

	for _, book := range []*Book{kept, alsoKept, doomed} {
		resolveFingerprint(t, cache, counting, book.FolderPath(), builder)
	}
	saveFingerprintCache(t, cache)

	before := readFingerprintCacheAt(t, libRoot)
	if len(before.Index) != 3 || len(before.Entries) != 2 {
		t.Fatalf("before the delete the cache holds %d records and %d entries, want 3 and 2", len(before.Index), len(before.Entries))
	}

	if err := shelf.DeleteBook(doomed.ID()); err != nil {
		t.Fatalf("DeleteBook: %v", err)
	}

	// A run that computes nothing new still collects: this is the only moment
	// the deleted book's records can be noticed.
	next := openFingerprintCache(t, shelf, testAlgo)
	for _, book := range []*Book{kept, alsoKept} {
		resolveFingerprint(t, next, counting, book.FolderPath(), builder)
	}
	saveFingerprintCache(t, next)

	after := readFingerprintCacheAt(t, libRoot)
	for key := range after.Index {
		if strings.HasPrefix(key, doomed.ID()+"/") {
			t.Errorf("the deleted book still has an index record at %q", key)
		}
	}
	if len(after.Index) != 2 {
		t.Errorf("the pruned cache holds %d index records, want 2", len(after.Index))
	}
	if len(after.Entries) != 1 {
		t.Errorf("the pruned cache holds %d entries, want 1 (the one both surviving books share)", len(after.Entries))
	}
}

// A read-only shelf reads the cache and refuses to write it, with an error a
// caller can recognize rather than a panic. A stale md5_hash it cannot repair
// is not allowed to fail the lookup either.
func TestFingerprintCacheOnAReadOnlyShelf(t *testing.T) {
	libRoot := t.TempDir()

	writable := newTestShelf(t, &ShelfConf{LibRoot: libRoot, LockMode: "none"})
	book := addFingerprintBook(t, writable, libRoot, "Dune", "the spice must flow")
	bookPath := book.FolderPath()
	sourcePath := sourceFilePath(t, libRoot, book)

	builder := &fakeFingerprint{label: "v1"}
	cache := openFingerprintCache(t, writable, testAlgo)
	stored := resolveFingerprint(t, cache, writable.dbRoot, bookPath, builder)
	saveFingerprintCache(t, cache)

	if err := writable.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	readOnly := newTestShelf(t, &ShelfConf{LibRoot: libRoot, LockMode: "none", ReadOnly: true})
	reader := openFingerprintCache(t, readOnly, testAlgo)

	if got := resolveFingerprint(t, reader, readOnly.dbRoot, bookPath, builder); got != stored {
		t.Errorf("the read-only shelf resolved to %+v, want the stored %+v", got, stored)
	}
	if got := builder.calls.Load(); got != 1 {
		t.Errorf("the read-only shelf rebuilt a fingerprint: builds %d, want 1", got)
	}

	if err := reader.Save(); !errors.Is(err, fsutil.ErrReadOnly) {
		t.Errorf("Save on a read-only shelf returned %v, want %v", err, fsutil.ErrReadOnly)
	}

	// A source edited underneath a read-only shelf: the fingerprint is still
	// answered, and the repair that cannot happen is not an error.
	if err := os.WriteFile(sourcePath, []byte("edited from outside"), 0644); err != nil {
		t.Fatalf("editing source.txt: %v", err)
	}
	shiftModTime(t, sourcePath, -30*time.Minute)

	edited := resolveFingerprint(t, reader, readOnly.dbRoot, bookPath, builder)
	if edited == stored {
		t.Error("the edited source resolved to the fingerprint of its old content")
	}
	if got := reader.Stats().Repaired; got != 0 {
		t.Errorf("a read-only shelf reported %d repairs, want 0", got)
	}
}

// A source written moments ago is deliberately not indexed: on a filesystem
// with a coarse clock its stat cannot yet prove the content behind it. The
// content level still answers, so the cost is a read and never a rebuild.
func TestFingerprintCacheWillNotIndexAFreshlyWrittenSource(t *testing.T) {
	libRoot := t.TempDir()
	shelf := newTestShelf(t, &ShelfConf{LibRoot: libRoot, LockMode: "none"})

	book, err := shelf.NewBookWith(nil, "Just Written", func(book *Book) error {
		source, err := book.NewSource(strings.NewReader("written moments ago"))
		if err != nil {
			return err
		}
		return book.SetCurrentSource(source.ID())
	})
	if err != nil {
		t.Fatalf("NewBookWith: %v", err)
	}

	counting := countSourceReads(t, shelf)
	builder := &fakeFingerprint{label: "v1"}
	cache := openFingerprintCache(t, shelf, testAlgo)

	resolveFingerprint(t, cache, counting, book.FolderPath(), builder)
	resolveFingerprint(t, cache, counting, book.FolderPath(), builder)

	if got := counting.opens.Load(); got != 2 {
		t.Errorf("a racily fresh source was answered from the index: opens %d, want 2", got)
	}
	if got := builder.calls.Load(); got != 1 {
		t.Errorf("a racily fresh source was fingerprinted twice: builds %d, want 1", got)
	}
	if got := cache.Stats().ContentHits; got != 1 {
		t.Errorf("content hits %d, want 1", got)
	}
}

func TestOpenFingerprintCacheRefusesAnIncompleteAlgorithm(t *testing.T) {
	shelf := newTestShelf(t, &ShelfConf{LibRoot: t.TempDir(), LockMode: "none"})

	incomplete := map[string]FingerprintAlgo{
		"no normalization": {Shingle: "char-5gram", Hash: "xxhash64", K: 128},
		"no shingling":     {Normalize: "v1", Hash: "xxhash64", K: 128},
		"no hash":          {Normalize: "v1", Shingle: "char-5gram", K: 128},
		"no k":             {Normalize: "v1", Shingle: "char-5gram", Hash: "xxhash64"},
	}

	for name, algo := range incomplete {
		if _, err := shelf.OpenFingerprintCache(algo); !errors.Is(err, ErrIncompleteFingerprintAlgo) {
			t.Errorf("%s: OpenFingerprintCache returned %v, want %v", name, err, ErrIncompleteFingerprintAlgo)
		}
	}
}

// A cache file this build cannot read is a miss, never an error: it is derived
// from the shelf, and the run that finds it rebuilds it.
func TestFingerprintCacheIgnoresAnUnusableFile(t *testing.T) {
	for _, testCase := range []struct {
		name    string
		content []byte
	}{
		{"not json", []byte("{not json")},
		{"a newer schema", []byte(`{"schema_version":99,"algo":{},"index":{},"entries":{}}`)},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			libRoot := t.TempDir()
			shelf := newTestShelf(t, &ShelfConf{LibRoot: libRoot, LockMode: "none"})
			book := addFingerprintBook(t, shelf, libRoot, "Dune", "the spice must flow")

			if err := os.WriteFile(path.Join(libRoot, appFolder, fingerprintCacheFileName), testCase.content, 0644); err != nil {
				t.Fatalf("planting a cache file: %v", err)
			}

			builder := &fakeFingerprint{label: "v1"}
			cache := openFingerprintCache(t, shelf, testAlgo)
			resolveFingerprint(t, cache, shelf.dbRoot, book.FolderPath(), builder)
			saveFingerprintCache(t, cache)

			stored := readFingerprintCacheAt(t, libRoot)
			if stored.SchemaVersion != fingerprintCacheSchemaVersion || len(stored.Entries) != 1 {
				t.Errorf("the rewritten cache is version %d with %d entries, want %d and 1", stored.SchemaVersion, len(stored.Entries), fingerprintCacheSchemaVersion)
			}
		})
	}
}

// An entry is keyed by content and never revisited, so a builder that produces
// nothing must not be allowed to store that nothing.
func TestFingerprintCacheRefusesAnEmptyFingerprint(t *testing.T) {
	libRoot := t.TempDir()
	shelf := newTestShelf(t, &ShelfConf{LibRoot: libRoot, LockMode: "none"})
	book := addFingerprintBook(t, shelf, libRoot, "Dune", "the spice must flow")

	cache := openFingerprintCache(t, shelf, testAlgo)
	target, source := reopen(t, shelf.dbRoot, book.FolderPath())

	if _, err := cache.Resolve(target, source, func([]byte) (FingerprintEntry, error) {
		return FingerprintEntry{}, nil
	}); err == nil {
		t.Fatal("Resolve accepted a fingerprint with no sketch")
	}

	builder := &fakeFingerprint{label: "v1"}
	if _, err := cache.Resolve(target, source, builder.build); err != nil {
		t.Fatalf("Resolve after a failed build: %v", err)
	}
	if got := builder.calls.Load(); got != 1 {
		t.Errorf("the refused fingerprint was cached: builds %d, want 1", got)
	}
}

// A legacy source with no md5_hash keeps not having one. Filling it in would
// rewrite every such meta.json on the first fingerprint run, and a missing hash
// is honest in a way a wrong one is not.
func TestFingerprintCacheLeavesAMissingSourceHashAlone(t *testing.T) {
	libRoot := t.TempDir()
	shelf := newTestShelf(t, &ShelfConf{LibRoot: libRoot, LockMode: "none"})
	book := addFingerprintBook(t, shelf, libRoot, "Legacy", "imported before hashes were stored")

	sources, err := book.ListSource()
	if err != nil {
		t.Fatalf("ListSource: %v", err)
	}
	metaPath := path.Join(libRoot, sources[0].FolderPath(), SourceMetaFile)

	raw, err := os.ReadFile(metaPath)
	if err != nil {
		t.Fatalf("reading meta.json: %v", err)
	}
	var meta map[string]any
	if err := json.Unmarshal(raw, &meta); err != nil {
		t.Fatalf("decoding meta.json: %v", err)
	}
	delete(meta, "md5_hash")
	stripped, err := json.Marshal(meta)
	if err != nil {
		t.Fatalf("encoding meta.json: %v", err)
	}
	if err := os.WriteFile(metaPath, stripped, 0644); err != nil {
		t.Fatalf("writing meta.json: %v", err)
	}

	builder := &fakeFingerprint{label: "v1"}
	cache := openFingerprintCache(t, shelf, testAlgo)
	resolveFingerprint(t, cache, shelf.dbRoot, book.FolderPath(), builder)

	after, err := os.ReadFile(metaPath)
	if err != nil {
		t.Fatalf("re-reading meta.json: %v", err)
	}
	var repaired SourceMeta
	if err := json.Unmarshal(after, &repaired); err != nil {
		t.Fatalf("decoding meta.json: %v", err)
	}
	if repaired.MD5Hash != "" {
		t.Errorf("meta.json gained md5_hash %q, want it left absent", repaired.MD5Hash)
	}
	if got := cache.Stats().Repaired; got != 0 {
		t.Errorf("reported %d repairs, want 0", got)
	}
}

// A source too freshly written to index still has its fingerprint kept: it was
// computed by this very run, so collecting it would mean building it again next
// time for nothing.
func TestFingerprintCacheKeepsAnUnindexedFingerprintItJustBuilt(t *testing.T) {
	libRoot := t.TempDir()
	shelf := newTestShelf(t, &ShelfConf{LibRoot: libRoot, LockMode: "none"})

	book, err := shelf.NewBookWith(nil, "Just Written", func(book *Book) error {
		source, err := book.NewSource(strings.NewReader("written moments ago"))
		if err != nil {
			return err
		}
		return book.SetCurrentSource(source.ID())
	})
	if err != nil {
		t.Fatalf("NewBookWith: %v", err)
	}

	counting := countSourceReads(t, shelf)
	builder := &fakeFingerprint{label: "v1"}

	first := openFingerprintCache(t, shelf, testAlgo)
	resolveFingerprint(t, first, counting, book.FolderPath(), builder)
	saveFingerprintCache(t, first)

	stored := readFingerprintCacheAt(t, libRoot)
	if len(stored.Index) != 0 {
		t.Errorf("a racily fresh source was indexed: %d records, want 0", len(stored.Index))
	}
	if len(stored.Entries) != 1 {
		t.Fatalf("the saved cache holds %d entries, want the one just built", len(stored.Entries))
	}

	second := openFingerprintCache(t, shelf, testAlgo)
	resolveFingerprint(t, second, counting, book.FolderPath(), builder)

	if got := builder.calls.Load(); got != 1 {
		t.Errorf("the next run rebuilt a fingerprint the last one saved: builds %d, want 1", got)
	}
	if got := second.Stats().ContentHits; got != 1 {
		t.Errorf("content hits %d, want 1", got)
	}
}

// A source restored from a backup has new content under an older modification
// time. The record this run observed must win anyway, or the stale one would be
// reinstated on every save and the source reread and rebuilt forever.
func TestFingerprintCacheKeepsTheRecordItJustObserved(t *testing.T) {
	libRoot := t.TempDir()
	shelf := newTestShelf(t, &ShelfConf{LibRoot: libRoot, LockMode: "none"})

	book := addFingerprintBook(t, shelf, libRoot, "Dune", "the spice must flow")
	bookPath := book.FolderPath()
	sourcePath := sourceFilePath(t, libRoot, book)

	counting := countSourceReads(t, shelf)
	builder := &fakeFingerprint{label: "v1"}

	first := openFingerprintCache(t, shelf, testAlgo)
	resolveFingerprint(t, first, counting, bookPath, builder)
	saveFingerprintCache(t, first)

	// Restored from a backup: other content, and a modification time older than
	// the one already recorded.
	if err := os.WriteFile(sourcePath, []byte("the spice as it was two backups ago"), 0644); err != nil {
		t.Fatalf("restoring source.txt: %v", err)
	}
	shiftModTime(t, sourcePath, -24*time.Hour)

	second := openFingerprintCache(t, shelf, testAlgo)
	restored := resolveFingerprint(t, second, counting, bookPath, builder)
	saveFingerprintCache(t, second)

	stored := readFingerprintCacheAt(t, libRoot)
	if len(stored.Index) != 1 {
		t.Fatalf("the saved cache holds %d index records, want 1", len(stored.Index))
	}
	for key, record := range stored.Index {
		if _, ok := stored.Entries[record.MD5]; !ok {
			t.Fatalf("the record at %q names an entry the cache does not hold", key)
		}
		if stored.Entries[record.MD5] != restored {
			t.Errorf("the record at %q resolves to %+v, want the restored %+v", key, stored.Entries[record.MD5], restored)
		}
	}

	// And the run after that answers from the index, which is what the merge
	// getting it wrong would cost.
	opensBefore := counting.opens.Load()
	third := openFingerprintCache(t, shelf, testAlgo)
	resolveFingerprint(t, third, counting, bookPath, builder)

	if got := counting.opens.Load(); got != opensBefore {
		t.Errorf("the restored source was read again: opens %d, want %d", got, opensBefore)
	}
	if got := builder.calls.Load(); got != 2 {
		t.Errorf("the restored source was fingerprinted again: builds %d, want 2", got)
	}
}
