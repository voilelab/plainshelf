package fingerprint

import (
	"encoding/json/v2"
	"errors"
	"fmt"
	"os"
	"path"
	"strings"
	"testing"
	"time"

	"github.com/voilelab/plainshelf/internal/appcache"
	"github.com/voilelab/plainshelf/internal/fsutil"
	"github.com/voilelab/plainshelf/shelf/bookpkg"
)

// The point of the whole file: a second run over an unchanged shelf opens no
// source at all. The first run reads each source once, the second reads none.
func TestFingerprintCacheAnswersUnchangedSourcesWithoutReadingThem(t *testing.T) {
	ts := newTestShelf(t)
	book := ts.addBook("dune.bookpkg", "book-dune", "Dune", "the spice must flow", -time.Hour)
	bookPath := book.PackagePath()

	counting := ts.countSourceReads()
	builder := &fakeFingerprint{label: "v1"}

	first := openCache(t, ts, counting, testAlgo)
	firstEntry := resolveFingerprint(t, first, counting, bookPath, builder)
	saveCache(t, first)

	if got := counting.opens.Load(); got != 1 {
		t.Fatalf("the first run opened source.txt %d times, want 1", got)
	}
	if got := builder.calls.Load(); got != 1 {
		t.Fatalf("the first run built %d fingerprints, want 1", got)
	}

	// A fresh cache, loaded from disk: the second run shares nothing with the
	// first but the file.
	second := openCache(t, ts, counting, testAlgo)
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
	cachePath := path.Join(ts.libRoot, appDir, cacheFileName)
	before, err := os.ReadFile(cachePath)
	if err != nil {
		t.Fatalf("reading the fingerprint cache: %v", err)
	}
	writtenAt := shiftedModTime(t, cachePath, -time.Hour)

	saveCache(t, second)

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

// A book moved between layers keeps its fingerprints: the index is keyed on the
// book ID, which survives the move, and not on the path, which does not.
func TestFingerprintCacheSurvivesMovingABook(t *testing.T) {
	ts := newTestShelf(t)
	book := ts.addBook("a.bookpkg", "book-dune", "Dune", "the spice must flow", -time.Hour)

	counting := ts.countSourceReads()
	builder := &fakeFingerprint{label: "v1"}

	first := openCache(t, ts, counting, testAlgo)
	before := resolveFingerprint(t, first, counting, book.PackagePath(), builder)
	saveCache(t, first)

	// A move is a rename of the package directory: the book ID inside book.json
	// and the source's stat both survive it, the folder name does not.
	if err := os.Rename(path.Join(ts.libRoot, "a.bookpkg"), path.Join(ts.libRoot, "b.bookpkg")); err != nil {
		t.Fatalf("moving the book: %v", err)
	}

	opensBefore := counting.opens.Load()
	second := openCache(t, ts, counting, testAlgo)
	after := resolveFingerprint(t, second, counting, "b.bookpkg", builder)

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
	ts := newTestShelf(t)

	const content = "the spice must flow"
	original := ts.addBook("dune.bookpkg", "book-orig", "Dune", content, -time.Hour)
	copied := ts.addBook("dune-copy.bookpkg", "book-copy", "Dune (copy)", content, -time.Hour)

	if original.ID() == copied.ID() {
		t.Fatal("the copy shares the original's book ID")
	}

	counting := ts.countSourceReads()
	builder := &fakeFingerprint{label: "v1"}
	cache := openCache(t, ts, counting, testAlgo)

	first := resolveFingerprint(t, cache, counting, original.PackagePath(), builder)
	opensAfterOriginal := counting.opens.Load()

	second := resolveFingerprint(t, cache, counting, copied.PackagePath(), builder)

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

	saveCache(t, cache)
	stored := readCacheAt(t, ts.libRoot)
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
	ts := newTestShelf(t)
	book := ts.addBook("dune.bookpkg", "book-dune", "Dune", "the spice must flow", -time.Hour)
	bookPath := book.PackagePath()
	sourcePath := ts.sourceFilePath(book)

	counting := ts.countSourceReads()
	builder := &fakeFingerprint{label: "v1"}

	cache := openCache(t, ts, counting, testAlgo)
	before := resolveFingerprint(t, cache, counting, bookPath, builder)
	saveCache(t, cache)

	const edited = "the spice must flow, and then some"
	if err := os.WriteFile(sourcePath, []byte(edited), 0644); err != nil {
		t.Fatalf("editing source.txt: %v", err)
	}
	shiftModTime(t, sourcePath, -30*time.Minute)

	_, staleSource := reopen(t, counting, bookPath)
	if metaMatchesContent(t, staleSource) {
		t.Fatal("the edited source should carry a stale hash")
	}

	second := openCache(t, ts, counting, testAlgo)
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
	if !metaMatchesContent(t, repaired) {
		t.Error("meta.json still disagrees with the content")
	}
}

// A cache built with other rules is discarded whole rather than partly reused:
// nothing can vouch for the comparability of an entry whose normalization or
// sketch parameters are not the ones in use.
func TestFingerprintCacheIsDiscardedWhenTheAlgorithmChanges(t *testing.T) {
	ts := newTestShelf(t)
	book := ts.addBook("dune.bookpkg", "book-dune", "Dune", "the spice must flow", -time.Hour)
	bookPath := book.PackagePath()

	counting := ts.countSourceReads()
	oldBuilder := &fakeFingerprint{label: "v1"}

	first := openCache(t, ts, counting, testAlgo)
	resolveFingerprint(t, first, counting, bookPath, oldBuilder)
	saveCache(t, first)

	newAlgo := testAlgo
	newAlgo.Normalize = "nfkc-strip-space-punct-v2"
	newBuilder := &fakeFingerprint{label: "v2"}

	second := openCache(t, ts, counting, newAlgo)
	rebuilt := resolveFingerprint(t, second, counting, bookPath, newBuilder)

	if got := newBuilder.calls.Load(); got != 1 {
		t.Errorf("the new algorithm reused a stored fingerprint: builds %d, want 1", got)
	}
	saveCache(t, second)

	stored := readCacheAt(t, ts.libRoot)
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
	ts := newTestShelf(t)
	mine := ts.addBook("mine.bookpkg", "book-mine", "Mine", "a book only this machine has read", -time.Hour)
	theirs := ts.addBook("theirs.bookpkg", "book-theirs", "Theirs", "a book only the other machine has read", -time.Hour)

	counting := ts.countSourceReads()
	builder := &fakeFingerprint{label: "v1"}

	cache := openCache(t, ts, counting, testAlgo)
	resolveFingerprint(t, cache, counting, mine.PackagePath(), builder)
	saveCache(t, cache)

	// What the other machine wrote in the meantime: its own book, plus a later
	// look at the book this one already knows.
	_, theirSource := reopen(t, counting, theirs.PackagePath())
	theirStat, err := theirSource.ContentStat()
	if err != nil {
		t.Fatalf("ContentStat: %v", err)
	}
	theirKey := indexKey(theirs.ID(), theirSource.ID())
	const theirMD5 = "0123456789abcdef0123456789abcdef"

	stored := readCacheAt(t, ts.libRoot)
	myKey := indexKey(mine.ID(), func() string {
		_, source := reopen(t, counting, mine.PackagePath())
		return source.ID()
	}())
	myRecord, ok := stored.Index[myKey]
	if !ok {
		t.Fatalf("the saved cache has no record for %q", myKey)
	}
	newer := myRecord
	newer.ModTime = myRecord.ModTime.Add(time.Minute)

	stored.Index[myKey] = newer
	stored.Index[theirKey] = indexEntry{Size: theirStat.Size, ModTime: theirStat.ModTime, MD5: theirMD5}
	stored.Entries[theirMD5] = Entry{NormMD5: theirMD5, NormChars: 7, Shingles: 7, Sketch: "theirs"}
	writeCacheAt(t, ts.libRoot, stored)

	// Any later save has to fold that in rather than replace it.
	saveCache(t, cache)

	merged := readCacheAt(t, ts.libRoot)
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
func TestMergeIndexKeepsTheNewerRecord(t *testing.T) {
	older := time.Date(2026, 3, 15, 8, 30, 0, 0, time.UTC)
	newer := older.Add(time.Hour)

	index := map[string]indexEntry{
		"book-a/source-1": {Size: 1, ModTime: newer, MD5: "newer"},
		"book-b/source-1": {Size: 2, ModTime: older, MD5: "older"},
	}
	other := map[string]indexEntry{
		"book-a/source-1": {Size: 3, ModTime: older, MD5: "older"},
		"book-b/source-1": {Size: 4, ModTime: newer, MD5: "newer"},
		"book-c/source-1": {Size: 5, ModTime: older, MD5: "only-theirs"},
	}

	mergeIndex(index, other)

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
	ts := newTestShelf(t)

	const shared = "a text two books hold"
	kept := ts.addBook("kept.bookpkg", "book-kept", "Kept", shared, -time.Hour)
	alsoKept := ts.addBook("also.bookpkg", "book-also", "Also Kept", shared, -time.Hour)
	doomed := ts.addBook("doomed.bookpkg", "book-doomed", "Doomed", "a text only one book holds", -time.Hour)

	counting := ts.countSourceReads()
	builder := &fakeFingerprint{label: "v1"}
	cache := openCache(t, ts, counting, testAlgo)

	for _, book := range []*bookpkg.Book{kept, alsoKept, doomed} {
		resolveFingerprint(t, cache, counting, book.PackagePath(), builder)
	}
	saveCache(t, cache)

	before := readCacheAt(t, ts.libRoot)
	if len(before.Index) != 3 || len(before.Entries) != 2 {
		t.Fatalf("before the delete the cache holds %d records and %d entries, want 3 and 2", len(before.Index), len(before.Entries))
	}

	// A delete is a book leaving the live set: the pruning input the caller now
	// owns, rather than a bookCache the cache used to read.
	delete(ts.live, doomed.ID())

	// A run that computes nothing new still collects: this is the only moment
	// the deleted book's records can be noticed.
	next := openCache(t, ts, counting, testAlgo)
	for _, book := range []*bookpkg.Book{kept, alsoKept} {
		resolveFingerprint(t, next, counting, book.PackagePath(), builder)
	}
	saveCache(t, next)

	after := readCacheAt(t, ts.libRoot)
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
	ts := newTestShelf(t)
	book := ts.addBook("dune.bookpkg", "book-dune", "Dune", "the spice must flow", -time.Hour)
	bookPath := book.PackagePath()
	sourcePath := ts.sourceFilePath(book)

	builder := &fakeFingerprint{label: "v1"}
	cache := openCache(t, ts, ts.base, testAlgo)
	stored := resolveFingerprint(t, cache, ts.base, bookPath, builder)
	saveCache(t, cache)

	// The same shelf reopened without write access: reads still work, every
	// write is refused with fsutil.ErrReadOnly.
	readOnly := fsutil.ReadOnly(ts.base)
	reader := openCache(t, ts, readOnly, testAlgo)

	if got := resolveFingerprint(t, reader, readOnly, bookPath, builder); got != stored {
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

	edited := resolveFingerprint(t, reader, readOnly, bookPath, builder)
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
	ts := newTestShelf(t)
	book := ts.addBook("fresh.bookpkg", "book-fresh", "Just Written", "written moments ago", 0)

	counting := ts.countSourceReads()
	builder := &fakeFingerprint{label: "v1"}
	cache := openCache(t, ts, counting, testAlgo)

	resolveFingerprint(t, cache, counting, book.PackagePath(), builder)
	resolveFingerprint(t, cache, counting, book.PackagePath(), builder)

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

// A caller that supplies only the required Config fields - Store and Algo, no
// Logger, LiveBooks or RepairHash - gets a working cache, not a panic on the
// first log line. This is the standalone / read-only reader's contract.
func TestOpenDefaultsAMissingLogger(t *testing.T) {
	ts := newTestShelf(t)
	book := ts.addBook("dune.bookpkg", "book-dune", "Dune", "the spice must flow", -time.Hour)

	// Open logs on the missing-cache-file path; reaching past it already proves
	// the zero-value logger did not panic.
	cache, err := Open(Config{Store: appcache.NewFSStore(ts.base, appDir), Algo: testAlgo})
	if err != nil {
		t.Fatalf("Open with no logger: %v", err)
	}

	builder := &fakeFingerprint{label: "v1"}
	target, source := reopen(t, ts.base, book.PackagePath())
	if _, err := cache.Resolve(target, source, builder.build); err != nil {
		t.Fatalf("Resolve with no logger: %v", err)
	}
	if err := cache.Save(); err != nil {
		t.Fatalf("Save with no logger: %v", err)
	}
}

func TestOpenRefusesAnIncompleteAlgorithm(t *testing.T) {
	ts := newTestShelf(t)

	incomplete := map[string]Algo{
		"no normalization": {Shingle: "char-5gram", Hash: "xxhash64", K: 128},
		"no shingling":     {Normalize: "v1", Hash: "xxhash64", K: 128},
		"no hash":          {Normalize: "v1", Shingle: "char-5gram", K: 128},
		"no k":             {Normalize: "v1", Shingle: "char-5gram", Hash: "xxhash64"},
	}

	for name, algo := range incomplete {
		_, err := Open(Config{Store: appcache.NewFSStore(ts.base, appDir), Algo: algo, Logger: newLoggerForTest()})
		if !errors.Is(err, ErrIncompleteAlgo) {
			t.Errorf("%s: Open returned %v, want %v", name, err, ErrIncompleteAlgo)
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
			ts := newTestShelf(t)
			book := ts.addBook("dune.bookpkg", "book-dune", "Dune", "the spice must flow", -time.Hour)

			if err := os.WriteFile(path.Join(ts.libRoot, appDir, cacheFileName), testCase.content, 0644); err != nil {
				t.Fatalf("planting a cache file: %v", err)
			}

			builder := &fakeFingerprint{label: "v1"}
			cache := openCache(t, ts, ts.base, testAlgo)
			resolveFingerprint(t, cache, ts.base, book.PackagePath(), builder)
			saveCache(t, cache)

			stored := readCacheAt(t, ts.libRoot)
			if stored.SchemaVersion != schemaVersion || len(stored.Entries) != 1 {
				t.Errorf("the rewritten cache is version %d with %d entries, want %d and 1", stored.SchemaVersion, len(stored.Entries), schemaVersion)
			}
		})
	}
}

// An entry is keyed by content and never revisited, so a builder that produces
// nothing must not be allowed to store that nothing.
func TestFingerprintCacheRefusesAnEmptyFingerprint(t *testing.T) {
	ts := newTestShelf(t)
	book := ts.addBook("dune.bookpkg", "book-dune", "Dune", "the spice must flow", -time.Hour)

	cache := openCache(t, ts, ts.base, testAlgo)
	target, source := reopen(t, ts.base, book.PackagePath())

	if _, err := cache.Resolve(target, source, func([]byte) (Entry, error) {
		return Entry{}, nil
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
	ts := newTestShelf(t)
	book := ts.addBook("legacy.bookpkg", "book-legacy", "Legacy", "imported before hashes were stored", -time.Hour)

	sources, err := book.ListSource()
	if err != nil {
		t.Fatalf("ListSource: %v", err)
	}
	metaPath := path.Join(ts.libRoot, sources[0].FolderPath(), bookpkg.SourceMetaFile)

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
	cache := openCache(t, ts, ts.base, testAlgo)
	resolveFingerprint(t, cache, ts.base, book.PackagePath(), builder)

	after, err := os.ReadFile(metaPath)
	if err != nil {
		t.Fatalf("re-reading meta.json: %v", err)
	}
	var repaired bookpkg.SourceMeta
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
	ts := newTestShelf(t)
	book := ts.addBook("fresh.bookpkg", "book-fresh", "Just Written", "written moments ago", 0)

	counting := ts.countSourceReads()
	builder := &fakeFingerprint{label: "v1"}

	first := openCache(t, ts, counting, testAlgo)
	resolveFingerprint(t, first, counting, book.PackagePath(), builder)
	saveCache(t, first)

	stored := readCacheAt(t, ts.libRoot)
	if len(stored.Index) != 0 {
		t.Errorf("a racily fresh source was indexed: %d records, want 0", len(stored.Index))
	}
	if len(stored.Entries) != 1 {
		t.Fatalf("the saved cache holds %d entries, want the one just built", len(stored.Entries))
	}

	second := openCache(t, ts, counting, testAlgo)
	resolveFingerprint(t, second, counting, book.PackagePath(), builder)

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
	ts := newTestShelf(t)
	book := ts.addBook("dune.bookpkg", "book-dune", "Dune", "the spice must flow", -time.Hour)
	bookPath := book.PackagePath()
	sourcePath := ts.sourceFilePath(book)

	counting := ts.countSourceReads()
	builder := &fakeFingerprint{label: "v1"}

	first := openCache(t, ts, counting, testAlgo)
	resolveFingerprint(t, first, counting, bookPath, builder)
	saveCache(t, first)

	// Restored from a backup: other content, and a modification time older than
	// the one already recorded.
	if err := os.WriteFile(sourcePath, []byte("the spice as it was two backups ago"), 0644); err != nil {
		t.Fatalf("restoring source.txt: %v", err)
	}
	shiftModTime(t, sourcePath, -24*time.Hour)

	second := openCache(t, ts, counting, testAlgo)
	restored := resolveFingerprint(t, second, counting, bookPath, builder)
	saveCache(t, second)

	stored := readCacheAt(t, ts.libRoot)
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
	third := openCache(t, ts, counting, testAlgo)
	resolveFingerprint(t, third, counting, bookPath, builder)

	if got := counting.opens.Load(); got != opensBefore {
		t.Errorf("the restored source was read again: opens %d, want %d", got, opensBefore)
	}
	if got := builder.calls.Load(); got != 2 {
		t.Errorf("the restored source was fingerprinted again: builds %d, want 2", got)
	}
}

// TestFingerprintCacheWithManyEntriesIsByteStable is the reverse condition for
// the json/v2 conversion. The cache skips its write by comparing the freshly
// encoded file against the bytes on disk, and cacheFile holds two maps — so the
// skip is only real while the encoder sorts them. json/v2 does not, unless the
// option set says so, and nothing about dropping it fails loudly: the file
// round-trips, and the single-book test above still passes because one entry
// has no order to vary. Twelve entries do, so this is where a missing
// Deterministic shows up as what it costs in the field, a re-upload of an
// unchanged cache on every scan.
func TestFingerprintCacheWithManyEntriesIsByteStable(t *testing.T) {
	ts := newTestShelf(t)

	counting := ts.countSourceReads()
	builder := &fakeFingerprint{label: "v1"}

	bookPaths := make([]string, 0, 12)
	for i := range 12 {
		id := fmt.Sprintf("book-%02d", i)
		book := ts.addBook(id+".bookpkg", id, "Title "+id, "content "+id, -time.Hour)
		bookPaths = append(bookPaths, book.PackagePath())
	}

	first := openCache(t, ts, counting, testAlgo)
	for _, bookPath := range bookPaths {
		resolveFingerprint(t, first, counting, bookPath, builder)
	}
	saveCache(t, first)

	cachePath := path.Join(ts.libRoot, appDir, cacheFileName)
	want, err := os.ReadFile(cachePath)
	if err != nil {
		t.Fatalf("reading the fingerprint cache: %v", err)
	}

	// Repeated because a random map order agrees with the first one now and
	// then; eight rounds of twelve entries do not agree by luck.
	for round := range 8 {
		cache := openCache(t, ts, counting, testAlgo)
		for _, bookPath := range bookPaths {
			resolveFingerprint(t, cache, counting, bookPath, builder)
		}
		writtenAt := shiftedModTime(t, cachePath, -time.Hour)
		saveCache(t, cache)

		got, err := os.ReadFile(cachePath)
		if err != nil {
			t.Fatalf("re-reading the fingerprint cache: %v", err)
		}
		if string(got) != string(want) {
			wantAt, gotAt := firstDifference(string(want), string(got))
			t.Fatalf("round %d re-encoded the same cache differently, from %q to %q", round, wantAt, gotAt)
		}
		if info, err := os.Stat(cachePath); err != nil {
			t.Fatalf("stat: %v", err)
		} else if !info.ModTime().Equal(writtenAt) {
			t.Fatalf("round %d rewrote an unchanged cache", round)
		}
	}
}

// firstDifference returns a short window of a and b around the first byte they
// disagree on. The cache is one long line, so printing both in full turns a
// one-key order change into a screenful of identical hashes.
func firstDifference(a, b string) (string, string) {
	i := 0
	for i < min(len(a), len(b)) && a[i] == b[i] {
		i++
	}
	return a[i:min(i+60, len(a))], b[i:min(i+60, len(b))]
}

// The fingerprint cache is rebuildable, so the strictness the shelf's
// hand-editable files gained is deliberately not applied to it: a duplicate
// member makes it a cache miss, not a failure a user has to repair. Nothing
// writes such a file - a sync tool merging two copies, or a truncated write,
// is what leaves one behind.
func TestFingerprintCacheWithDuplicateMemberIsDiscardedNotReported(t *testing.T) {
	ts := newTestShelf(t)

	writeCacheAt(t, ts.libRoot, cacheFile{
		SchemaVersion: schemaVersion,
		Algo:          testAlgo,
		Index:         map[string]indexEntry{"sources/20260315-a1/source.txt": {MD5: "abc"}},
		Entries:       map[string]Entry{"abc": {NormMD5: "abc"}},
	})
	if got := len(openCache(t, ts, ts.base, testAlgo).entries); got != 1 {
		t.Fatalf("entries = %d before the file is broken, want 1", got)
	}

	// Not a typo a person makes in this file - a merge by a sync tool, or two
	// writes interleaved - but it is what the strict decoder now refuses.
	raw := `{"schema_version": 1, "schema_version": 1}`
	if err := os.WriteFile(path.Join(ts.libRoot, appDir, cacheFileName), []byte(raw), 0644); err != nil {
		t.Fatalf("writing a fingerprint cache: %v", err)
	}

	cache := openCache(t, ts, ts.base, testAlgo)
	if got := len(cache.entries); got != 0 {
		t.Errorf("entries = %d, want the unreadable cache discarded", got)
	}
}
