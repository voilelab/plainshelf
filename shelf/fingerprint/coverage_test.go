package fingerprint

import (
	"testing"
	"time"
)

func TestLookupUsesLoadedCacheWithoutStat(t *testing.T) {
	ts := newTestShelf(t)
	book := ts.addBook("dune.bookpkg", "book-dune", "Dune", "the spice must flow", -time.Hour)

	builder := &fakeFingerprint{label: "v1"}
	cache := openCache(t, ts, ts.base, testAlgo)
	want := resolveFingerprint(t, cache, ts.base, book.PackagePath(), builder)

	saveCache(t, cache)
	counting := ts.countSourceReads()
	cache = openCache(t, ts, counting, testAlgo)

	got, ok := cache.LookupRecorded(book.ID(), book.CurrentSource())
	if !ok {
		t.Fatalf("LookupRecorded(%q, %q) missed a source loaded from disk", book.ID(), book.CurrentSource())
	}
	if got != want {
		t.Errorf("LookupRecorded returned %+v, want %+v", got, want)
	}

	if _, ok := cache.LookupRecorded(book.ID(), "no-such-source"); ok {
		t.Error("LookupRecorded found a source the cache has no record of")
	}
	if _, ok := cache.LookupRecorded("no-such-book", book.CurrentSource()); ok {
		t.Error("LookupRecorded found a book the cache has no record of")
	}
	if got := counting.stats.Load(); got != 0 {
		t.Errorf("LookupRecorded statted %d paths, want 0", got)
	}
	if got := counting.opens.Load(); got != 0 {
		t.Errorf("LookupRecorded opened source.txt %d times, want 0", got)
	}
}

func TestCoverageForReadsNoSources(t *testing.T) {
	ts := newTestShelf(t)
	dune := ts.addBook("dune.bookpkg", "book-dune", "Dune", "the spice must flow", -time.Hour)
	solaris := ts.addBook("solaris.bookpkg", "book-solaris", "Solaris", "the ocean thinks in shapes", -time.Hour)

	builder := &fakeFingerprint{label: "v1"}
	cache := openCache(t, ts, ts.base, testAlgo)
	resolveFingerprint(t, cache, ts.base, dune.PackagePath(), builder)
	saveCache(t, cache)

	counting := ts.countSourceReads()
	fresh := openCache(t, ts, counting, testAlgo)

	coverage := fresh.CoverageFor([]BookSource{
		{BookID: dune.ID(), SourceID: dune.CurrentSource()},
		{BookID: solaris.ID(), SourceID: solaris.CurrentSource()},
	})

	if got := counting.opens.Load(); got != 0 {
		t.Errorf("CoverageFor opened source.txt %d times, want 0", got)
	}
	if coverage.Total != 2 || coverage.Fingerprinted != 1 || coverage.Missing != 1 {
		t.Errorf("coverage = %+v, want total 2, fingerprinted 1, missing 1", coverage)
	}
	if coverage.Algo != testAlgo {
		t.Errorf("coverage algo = %+v, want %+v", coverage.Algo, testAlgo)
	}
}

func TestCoverageForOtherRulesIsZero(t *testing.T) {
	ts := newTestShelf(t)
	dune := ts.addBook("dune.bookpkg", "book-dune", "Dune", "the spice must flow", -time.Hour)

	builder := &fakeFingerprint{label: "v1"}
	cache := openCache(t, ts, ts.base, testAlgo)
	resolveFingerprint(t, cache, ts.base, dune.PackagePath(), builder)
	saveCache(t, cache)

	other := Algo{Normalize: "other-v9", Shingle: "char-7gram", Hash: "xxhash64", K: 64}
	otherCache := openCache(t, ts, ts.base, other)

	coverage := otherCache.CoverageFor([]BookSource{
		{BookID: dune.ID(), SourceID: dune.CurrentSource()},
	})

	if coverage.Total != 1 || coverage.Fingerprinted != 0 || coverage.Missing != 1 {
		t.Errorf("coverage = %+v, want the book counted as missing under new rules", coverage)
	}
	if coverage.Algo != other {
		t.Errorf("coverage algo = %+v, want the requested %+v", coverage.Algo, other)
	}
}
