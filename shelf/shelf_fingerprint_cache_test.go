package shelf

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/voilelab/plainshelf/internal/fsutil"
)

const fingerprintBody = "第一章\n\n這是一段用來建立指紋的內文，長度足夠產生若干 shingle。\n"

// sweep fingerprints every source in the shelf and reports what each one cost,
// keyed the way the cache is. It is the shelf-level equivalent of one run of the
// fingerprint_sources task.
func sweep(t *testing.T, s *Shelf, cache *FingerprintCache) map[string]FingerprintOutcome {
	t.Helper()

	targets, failed, err := s.FingerprintTargets()
	if err != nil {
		t.Fatalf("FingerprintTargets: %v", err)
	}
	if len(failed) != 0 {
		t.Fatalf("FingerprintTargets could not list sources of %v", failed)
	}

	outcomes := map[string]FingerprintOutcome{}
	for _, target := range targets {
		outcome, err := s.Fingerprint(cache, target)
		if err != nil {
			t.Fatalf("Fingerprint(%s/%s): %v", target.BookID, target.SourceID, err)
		}
		outcomes[FingerprintKey(target.BookID, target.SourceID)] = outcome
	}
	return outcomes
}

func onlyOutcome(t *testing.T, outcomes map[string]FingerprintOutcome, key string) FingerprintOutcome {
	t.Helper()

	outcome, ok := outcomes[key]
	if !ok {
		t.Fatalf("no outcome for %q, got %v", key, outcomes)
	}
	return outcome
}

// The first sweep fingerprints everything; a second sweep over an unchanged
// shelf answers from the index alone, which is what makes the cache worth
// keeping at all.
func TestFingerprintSweepReusesUnchangedSources(t *testing.T) {
	s := newTestShelf(t, &ShelfConf{LibRoot: path.Join(t.TempDir(), "shelf")})

	book, source := newBookWithSource(t, s, nil, "Fingerprinted", fingerprintBody)
	key := FingerprintKey(book.ID(), source.ID())

	cache := s.LoadFingerprintCache()
	if got := onlyOutcome(t, sweep(t, s, cache), key); got != FingerprintComputed {
		t.Fatalf("first sweep outcome = %v, want FingerprintComputed", got)
	}

	entry, ok := cache.Entries[cache.Index[key].MD5]
	if !ok {
		t.Fatal("the computed fingerprint was not stored")
	}
	if entry.Sketch == "" || entry.NormMD5 == "" || entry.NormChars == 0 || entry.Shingles == 0 {
		t.Errorf("stored fingerprint is incomplete: %+v", entry)
	}

	if err := s.SaveFingerprintCache(cache); err != nil {
		t.Fatalf("SaveFingerprintCache: %v", err)
	}

	// A fresh load, so the second sweep starts from the file and not from the
	// value the first one happened to leave in memory.
	reloaded := s.LoadFingerprintCache()
	if got := onlyOutcome(t, sweep(t, s, reloaded), key); got != FingerprintReused {
		t.Errorf("second sweep outcome = %v, want FingerprintReused", got)
	}
}

// A book ID survives a move between layers, so reorganizing a shelf must not
// cost a single re-read. A cache keyed by path would fail this.
func TestFingerprintSweepSurvivesMovingABook(t *testing.T) {
	s := newTestShelf(t, &ShelfConf{LibRoot: path.Join(t.TempDir(), "shelf")})

	book, source := newBookWithSource(t, s, Layers{"fiction"}, "Moved", fingerprintBody)
	key := FingerprintKey(book.ID(), source.ID())

	cache := s.LoadFingerprintCache()
	sweep(t, s, cache)

	if _, err := s.MoveBook(book.ID(), Layers{"archive"}); err != nil {
		t.Fatalf("MoveBook: %v", err)
	}

	if got := onlyOutcome(t, sweep(t, s, cache), key); got != FingerprintReused {
		t.Errorf("outcome after moving the book = %v, want FingerprintReused", got)
	}
}

// A second copy of a text is a new book with a new ID, so its index key misses
// and the file has to be read. Its fingerprint is already stored under the
// content's own hash, though, so nothing is computed twice.
func TestFingerprintSweepDeduplicatesIdenticalContent(t *testing.T) {
	s := newTestShelf(t, &ShelfConf{LibRoot: path.Join(t.TempDir(), "shelf")})

	original, originalSource := newBookWithSource(t, s, nil, "Original", fingerprintBody)

	cache := s.LoadFingerprintCache()
	sweep(t, s, cache)

	copied, copiedSource := newBookWithSource(t, s, nil, "Copy", fingerprintBody)

	outcomes := sweep(t, s, cache)
	if got := onlyOutcome(t, outcomes, FingerprintKey(original.ID(), originalSource.ID())); got != FingerprintReused {
		t.Errorf("original outcome = %v, want FingerprintReused", got)
	}
	if got := onlyOutcome(t, outcomes, FingerprintKey(copied.ID(), copiedSource.ID())); got != FingerprintDeduped {
		t.Errorf("copy outcome = %v, want FingerprintDeduped", got)
	}

	if len(cache.Entries) != 1 {
		t.Errorf("stored %d fingerprints for one distinct text, want 1", len(cache.Entries))
	}
	if len(cache.Index) != 2 {
		t.Errorf("indexed %d sources, want 2", len(cache.Index))
	}
}

// Editing source.txt outside PlainShelf changes its stat, which is the whole
// staleness check. meta.json's md5_hash is not consulted: it has no invalidation
// of its own and is wrong exactly in this case.
func TestFingerprintSweepRecomputesAnEditedSource(t *testing.T) {
	libRoot := path.Join(t.TempDir(), "shelf")
	s := newTestShelf(t, &ShelfConf{LibRoot: libRoot})

	book, source := newBookWithSource(t, s, nil, "Edited", fingerprintBody)
	key := FingerprintKey(book.ID(), source.ID())

	cache := s.LoadFingerprintCache()
	sweep(t, s, cache)
	before := cache.Index[key].MD5

	sourcePath := filepath.Join(libRoot, filepath.FromSlash(source.FolderPath()), SourceFile)
	if err := os.WriteFile(sourcePath, []byte(fingerprintBody+"新增的一段內文，讓長度與雜湊都不同。\n"), 0o644); err != nil {
		t.Fatalf("rewrite source: %v", err)
	}

	if got := onlyOutcome(t, sweep(t, s, cache), key); got != FingerprintComputed {
		t.Errorf("outcome after editing the source = %v, want FingerprintComputed", got)
	}
	if cache.Index[key].MD5 == before {
		t.Error("the index still points at the content the source no longer holds")
	}
}

// A source that cannot be read is reported to the caller rather than skipped, so
// a sweep can put it in its failure list.
func TestFingerprintReportsAnUnreadableSource(t *testing.T) {
	libRoot := path.Join(t.TempDir(), "shelf")
	s := newTestShelf(t, &ShelfConf{LibRoot: libRoot})

	book, source := newBookWithSource(t, s, nil, "Damaged", fingerprintBody)

	sourcePath := filepath.Join(libRoot, filepath.FromSlash(source.FolderPath()), SourceFile)
	if err := os.Remove(sourcePath); err != nil {
		t.Fatalf("remove source: %v", err)
	}

	cache := s.LoadFingerprintCache()
	_, err := s.Fingerprint(cache, FingerprintTarget{
		BookID:     book.ID(),
		SourceID:   source.ID(),
		SourcePath: source.FolderPath(),
	})
	if err == nil {
		t.Fatal("Fingerprint() succeeded on a source with no content file")
	}
	if len(cache.Index) != 0 || len(cache.Entries) != 0 {
		t.Errorf("a failed source left something in the cache: %+v", cache)
	}
}

// Saving reclaims what the shelf no longer refers to: the index keys of a
// deleted book, and then the fingerprints nothing points at any more.
func TestSaveFingerprintCacheReclaimsDeletedBooks(t *testing.T) {
	s := newTestShelf(t, &ShelfConf{LibRoot: path.Join(t.TempDir(), "shelf")})

	kept, keptSource := newBookWithSource(t, s, nil, "Kept", fingerprintBody)
	removed, _ := newBookWithSource(t, s, nil, "Removed", fingerprintBody+"另一本書的內容。\n")

	cache := s.LoadFingerprintCache()
	sweep(t, s, cache)
	if err := s.SaveFingerprintCache(cache); err != nil {
		t.Fatalf("SaveFingerprintCache: %v", err)
	}
	if len(s.LoadFingerprintCache().Entries) != 2 {
		t.Fatalf("expected both books to be fingerprinted before the deletion")
	}

	if err := s.DeleteBook(removed.ID()); err != nil {
		t.Fatalf("DeleteBook: %v", err)
	}

	cache = s.LoadFingerprintCache()
	sweep(t, s, cache)
	if err := s.SaveFingerprintCache(cache); err != nil {
		t.Fatalf("SaveFingerprintCache: %v", err)
	}

	reclaimed := s.LoadFingerprintCache()
	keptKey := FingerprintKey(kept.ID(), keptSource.ID())
	if _, ok := reclaimed.Index[keptKey]; !ok {
		t.Errorf("the surviving book lost its index entry")
	}
	if len(reclaimed.Index) != 1 {
		t.Errorf("index holds %d entries after the deletion, want 1: %+v", len(reclaimed.Index), reclaimed.Index)
	}
	if len(reclaimed.Entries) != 1 {
		t.Errorf("cache holds %d fingerprints after the deletion, want 1", len(reclaimed.Entries))
	}
}

// Two machines sharing a shelf merge by union, because a fingerprint is a pure
// function of the bytes it describes and cannot disagree with itself.
func TestFingerprintCacheMergeKeepsBothSides(t *testing.T) {
	older := time.Date(2026, 3, 15, 8, 30, 0, 0, time.UTC)
	newer := older.Add(time.Hour)

	theirs := NewFingerprintCache()
	theirs.Entries["md5-a"] = FingerprintEntry{NormMD5: "norm-a", Sketch: "sketch-a"}
	theirs.Entries["md5-b"] = FingerprintEntry{NormMD5: "norm-b", Sketch: "sketch-b"}
	theirs.Index["book-1/src-1"] = FingerprintIndexEntry{Size: 10, ModTime: newer, MD5: "md5-a"}
	theirs.Index["book-2/src-1"] = FingerprintIndexEntry{Size: 20, ModTime: older, MD5: "md5-b"}

	ours := NewFingerprintCache()
	ours.Entries["md5-c"] = FingerprintEntry{NormMD5: "norm-c", Sketch: "sketch-c"}
	ours.Index["book-1/src-1"] = FingerprintIndexEntry{Size: 30, ModTime: older, MD5: "md5-c"}
	ours.Index["book-3/src-1"] = FingerprintIndexEntry{Size: 40, ModTime: older, MD5: "md5-c"}

	merged := NewFingerprintCache()
	merged.mergeFrom(theirs)
	merged.mergeFrom(ours)

	for _, md5 := range []string{"md5-a", "md5-b", "md5-c"} {
		if _, ok := merged.Entries[md5]; !ok {
			t.Errorf("merge lost the fingerprint %q", md5)
		}
	}

	// book-1 was seen more recently on the other machine, so its record wins
	// even though ours was merged last.
	if got := merged.Index["book-1/src-1"]; got.MD5 != "md5-a" {
		t.Errorf("book-1 index = %+v, want the newer record", got)
	}
	if got := merged.Index["book-3/src-1"]; got.MD5 != "md5-c" {
		t.Errorf("merge lost an index key only this side had: %+v", merged.Index)
	}
}

// The algo block is compared as a whole and any difference discards the file.
// Partly reusing it would mix two algorithms in one cache, which nothing
// downstream could detect.
func TestLoadFingerprintCacheDiscardsAForeignAlgorithm(t *testing.T) {
	libRoot := path.Join(t.TempDir(), "shelf")
	s := newTestShelf(t, &ShelfConf{LibRoot: libRoot})

	_, _ = newBookWithSource(t, s, nil, "Fingerprinted", fingerprintBody)

	cache := s.LoadFingerprintCache()
	sweep(t, s, cache)
	if err := s.SaveFingerprintCache(cache); err != nil {
		t.Fatalf("SaveFingerprintCache: %v", err)
	}

	cachePath := filepath.Join(libRoot, appFolder, FingerprintCacheFileName)
	raw, err := os.ReadFile(cachePath)
	if err != nil {
		t.Fatalf("read cache: %v", err)
	}

	var onDisk map[string]any
	if err := json.Unmarshal(raw, &onDisk); err != nil {
		t.Fatalf("decode cache: %v", err)
	}
	algo, ok := onDisk["algo"].(map[string]any)
	if !ok {
		t.Fatalf("cache has no algo block: %s", raw)
	}
	algo["normalize"] = "some-other-normalizer-v9"

	tampered, err := json.Marshal(onDisk)
	if err != nil {
		t.Fatalf("encode cache: %v", err)
	}
	if err := os.WriteFile(cachePath, tampered, 0o644); err != nil {
		t.Fatalf("write cache: %v", err)
	}

	reloaded := s.LoadFingerprintCache()
	if len(reloaded.Index) != 0 || len(reloaded.Entries) != 0 {
		t.Errorf("a cache built by another algorithm was partly kept: %+v", reloaded)
	}
	if reloaded.Algo != currentFingerprintAlgo() {
		t.Errorf("replacement cache algo = %+v, want this build's", reloaded.Algo)
	}
}

// An unchanged sweep must not rewrite the file. On the transports this cache
// matters for, an identical rewrite is a pointless upload and a pointless
// conflict opportunity for a sync client.
func TestSaveFingerprintCacheSkipsAnUnchangedWrite(t *testing.T) {
	libRoot := path.Join(t.TempDir(), "shelf")
	s := newTestShelf(t, &ShelfConf{LibRoot: libRoot})

	_, _ = newBookWithSource(t, s, nil, "Fingerprinted", fingerprintBody)

	cache := s.LoadFingerprintCache()
	sweep(t, s, cache)
	if err := s.SaveFingerprintCache(cache); err != nil {
		t.Fatalf("SaveFingerprintCache: %v", err)
	}

	cachePath := filepath.Join(libRoot, appFolder, FingerprintCacheFileName)
	before, err := os.Stat(cachePath)
	if err != nil {
		t.Fatalf("stat cache: %v", err)
	}
	shiftModTime(t, cachePath, -time.Hour)

	again := s.LoadFingerprintCache()
	sweep(t, s, again)
	if err := s.SaveFingerprintCache(again); err != nil {
		t.Fatalf("SaveFingerprintCache: %v", err)
	}

	after, err := os.Stat(cachePath)
	if err != nil {
		t.Fatalf("stat cache: %v", err)
	}
	if after.ModTime().After(before.ModTime()) {
		t.Error("an unchanged sweep rewrote the fingerprint cache")
	}
}

// A read-only shelf still compares what it already has; only writing back is
// refused, and refused with the error the API turns into a readable answer.
func TestFingerprintCacheOnReadOnlyShelf(t *testing.T) {
	libRoot := t.TempDir()
	bookID := seedReadOnlyShelf(t, libRoot)

	writable := newTestShelf(t, &ShelfConf{LibRoot: libRoot, LockMode: "none"})
	cache := writable.LoadFingerprintCache()
	sweep(t, writable, cache)
	if err := writable.SaveFingerprintCache(cache); err != nil {
		t.Fatalf("SaveFingerprintCache: %v", err)
	}
	if err := writable.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	denyWrites(t, libRoot)

	s, err := NewShelf(&ShelfConf{LibRoot: libRoot, ReadOnly: true})
	if err != nil {
		t.Fatalf("NewShelf: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	if err := s.WaitReady(context.Background()); err != nil {
		t.Fatalf("WaitReady: %v", err)
	}

	readOnlyCache := s.LoadFingerprintCache()
	if len(readOnlyCache.Entries) == 0 {
		t.Error("a read-only shelf could not read an existing fingerprint cache")
	}
	if len(readOnlyCache.Index) == 0 {
		t.Errorf("a read-only shelf read no index entries for book %s", bookID)
	}

	if err := s.SaveFingerprintCache(readOnlyCache); !errors.Is(err, fsutil.ErrReadOnly) {
		t.Errorf("SaveFingerprintCache on a read-only shelf = %v, want %v", err, fsutil.ErrReadOnly)
	}
}

// The index key is the one place two IDs are joined into a string, and both
// halves are validated path segments, so the join cannot be ambiguous.
func TestFingerprintKeyNamesBookAndSource(t *testing.T) {
	key := FingerprintKey("3f2a91bc", "20260315-083000")
	if key != "3f2a91bc/20260315-083000" {
		t.Errorf("FingerprintKey = %q", key)
	}
	bookID, sourceID, ok := strings.Cut(key, "/")
	if !ok || bookID != "3f2a91bc" || sourceID != "20260315-083000" {
		t.Errorf("key %q does not split back into its two halves", key)
	}
}

// A book that holds no source yet is not a damaged book. It contributes nothing
// to fingerprint and must not be reported as a failure.
func TestFingerprintTargetsSkipsABookWithNoSources(t *testing.T) {
	s := newTestShelf(t, &ShelfConf{LibRoot: path.Join(t.TempDir(), "shelf")})

	if _, err := s.NewBook(nil, "Empty Book"); err != nil {
		t.Fatalf("NewBook: %v", err)
	}

	targets, failed, err := s.FingerprintTargets()
	if err != nil {
		t.Fatalf("FingerprintTargets: %v", err)
	}
	if len(targets) != 0 {
		t.Errorf("targets = %+v, want none", targets)
	}
	if len(failed) != 0 {
		t.Errorf("a book with no sources was reported as unlistable: %v", failed)
	}
}
