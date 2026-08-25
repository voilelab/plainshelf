package fingerprint

import (
	"testing"
	"time"
)

// Lookup answers from the loaded index and entries alone: a source that was
// resolved is found, and anything the cache has no record of is a clean miss.
func TestFingerprintCacheLookup(t *testing.T) {
	ts := newTestShelf(t)
	book := ts.addBook("dune.bookpkg", "book-dune", "Dune", "the spice must flow", -time.Hour)

	builder := &fakeFingerprint{label: "v1"}
	cache := openCache(t, ts, ts.base, testAlgo)
	want := resolveFingerprint(t, cache, ts.base, book.PackagePath(), builder)

	got, ok := cache.Lookup(book.ID(), book.CurrentSource())
	if !ok {
		t.Fatalf("Lookup(%q, %q) missed a source that was just resolved", book.ID(), book.CurrentSource())
	}
	if got != want {
		t.Errorf("Lookup returned %+v, want %+v", got, want)
	}

	if _, ok := cache.Lookup(book.ID(), "no-such-source"); ok {
		t.Error("Lookup found a source the cache has no record of")
	}
	if _, ok := cache.Lookup("no-such-book", book.CurrentSource()); ok {
		t.Error("Lookup found a book the cache has no record of")
	}
}

// CoverageFor counts coverage from the loaded cache and the book list it is
// handed without opening a single source.txt - the property the maintenance
// page's status bar depends on, since it loads on every visit.
func TestCoverageForReadsNoSources(t *testing.T) {
	ts := newTestShelf(t)
	dune := ts.addBook("dune.bookpkg", "book-dune", "Dune", "the spice must flow", -time.Hour)
	solaris := ts.addBook("solaris.bookpkg", "book-solaris", "Solaris", "the ocean thinks in shapes", -time.Hour)

	// Fingerprint one of the two books so the count has both a hit and a miss.
	builder := &fakeFingerprint{label: "v1"}
	cache := openCache(t, ts, ts.base, testAlgo)
	resolveFingerprint(t, cache, ts.base, dune.PackagePath(), builder)
	saveCache(t, cache)

	// From here on, any source.txt open is one this coverage read caused.
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

// A cache built under different rules answers for nothing: every book reads as
// missing, which is what tells the UI to offer a full rebuild.
func TestCoverageForUnderDifferentRulesCountsNothing(t *testing.T) {
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
