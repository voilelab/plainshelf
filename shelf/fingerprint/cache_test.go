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
	"github.com/voilelab/plainshelf/internal/jsonopt"
	"github.com/voilelab/plainshelf/shelf/bookpkg"
)

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

func TestFingerprintCacheSurvivesMovingABook(t *testing.T) {
	ts := newTestShelf(t)
	book := ts.addBook("a.bookpkg", "book-dune", "Dune", "the spice must flow", -time.Hour)

	counting := ts.countSourceReads()
	builder := &fakeFingerprint{label: "v1"}

	first := openCache(t, ts, counting, testAlgo)
	before := resolveFingerprint(t, first, counting, book.PackagePath(), builder)
	saveCache(t, first)

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

func TestFingerprintCacheMergesAnotherWritersEntries(t *testing.T) {
	ts := newTestShelf(t)
	mine := ts.addBook("mine.bookpkg", "book-mine", "Mine", "a book only this machine has read", -time.Hour)
	theirs := ts.addBook("theirs.bookpkg", "book-theirs", "Theirs", "a book only the other machine has read", -time.Hour)

	counting := ts.countSourceReads()
	builder := &fakeFingerprint{label: "v1"}

	cache := openCache(t, ts, counting, testAlgo)
	resolveFingerprint(t, cache, counting, mine.PackagePath(), builder)
	saveCache(t, cache)

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

func TestMergeIndexFallsBackToModTimeForRecordsWithoutSeenAt(t *testing.T) {
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

	delete(ts.live, doomed.ID())

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

func TestFingerprintCacheOnReadOnlyShelfResolvesButRefusesSaveAndRepair(t *testing.T) {
	ts := newTestShelf(t)
	book := ts.addBook("dune.bookpkg", "book-dune", "Dune", "the spice must flow", -time.Hour)
	bookPath := book.PackagePath()
	sourcePath := ts.sourceFilePath(book)

	builder := &fakeFingerprint{label: "v1"}
	cache := openCache(t, ts, ts.base, testAlgo)
	stored := resolveFingerprint(t, cache, ts.base, bookPath, builder)
	saveCache(t, cache)

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

func TestOpenDefaultsAMissingLogger(t *testing.T) {
	ts := newTestShelf(t)
	book := ts.addBook("dune.bookpkg", "book-dune", "Dune", "the spice must flow", -time.Hour)

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

func firstDifference(a, b string) (string, string) {
	i := 0
	for i < min(len(a), len(b)) && a[i] == b[i] {
		i++
	}
	return a[i:min(i+60, len(a))], b[i:min(i+60, len(b))]
}

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

	raw := `{"schema_version": 1, "schema_version": 1}`
	if err := os.WriteFile(path.Join(ts.libRoot, appDir, cacheFileName), []byte(raw), 0644); err != nil {
		t.Fatalf("writing a fingerprint cache: %v", err)
	}

	cache := openCache(t, ts, ts.base, testAlgo)
	if got := len(cache.entries); got != 0 {
		t.Errorf("entries = %d, want the unreadable cache discarded", got)
	}
}

func TestCacheFileMarshalsSchemaVersionAsTheFirstKey(t *testing.T) {
	data, err := json.Marshal(cacheFile{SchemaVersion: schemaVersion}, jsonopt.DiskCompact())
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.HasPrefix(string(data), `{"schema_version":`) {
		t.Errorf("schema_version is not the first key: %s", data)
	}
}

func TestFingerprintCacheNeverPairsANewerStatWithOlderContent(t *testing.T) {
	ts := newTestShelf(t)
	book := ts.addBook("dune.bookpkg", "book-dune", "Dune", "the spice must flow", -time.Hour)
	bookPath := book.PackagePath()
	sourcePath := ts.sourceFilePath(book)
	const rewritten = "the spice must flow, and so must the water"

	rewriting := &rewriteAfterReadFS{FS: ts.base, rewrite: func() {
		if err := os.WriteFile(sourcePath, []byte(rewritten), 0o644); err != nil {
			t.Errorf("rewriting source.txt: %v", err)
		}
		shiftModTime(t, sourcePath, -30*time.Minute)
	}}
	builder := &fakeFingerprint{label: "v1"}

	first := openCache(t, ts, rewriting, testAlgo)
	resolveFingerprint(t, first, rewriting, bookPath, builder)
	saveCache(t, first)

	second := openCache(t, ts, ts.base, testAlgo)
	got := resolveFingerprint(t, second, ts.base, bookPath, builder)
	want, err := builder.build([]byte(rewritten))
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if got != want {
		t.Errorf("after a write between stat and read, the next run answered %+v, want the rewritten content's %+v", got, want)
	}
}

func TestMergeIndexOrdersSeenAtBeforeModTime(t *testing.T) {
	older := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	newer := older.Add(time.Hour)
	for _, tc := range []struct {
		name          string
		currentSeenAt time.Time
		otherSeenAt   time.Time
		want          string
	}{
		{"later observation wins despite older mtime", older, newer, "other"},
		{"earlier observation loses despite being merged later", newer, older, "current"},
		{"observed record beats legacy record", time.Time{}, older, "other"},
		{"legacy record does not replace observed record", older, time.Time{}, "current"},
		{"equal observation falls back to mtime", older, older, "current"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			index := map[string]indexEntry{"book/source": {MD5: "current", ModTime: newer, SeenAt: tc.currentSeenAt}}
			other := map[string]indexEntry{"book/source": {MD5: "other", ModTime: older, SeenAt: tc.otherSeenAt}}
			mergeIndex(index, other)
			if got := index["book/source"].MD5; got != tc.want {
				t.Errorf("merged fingerprint = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestFingerprintCacheRecordsZeroModTime(t *testing.T) {
	ts := newTestShelf(t)
	cache := openCache(t, ts, ts.base, testAlgo)
	cache.record("book/source", &bookpkg.FileStat{}, time.Now(), "content", Entry{}, false)
	if _, ok := cache.LookupRecorded("book", "source"); !ok {
		t.Error("zero source mtime was not recorded")
	}
}
