package shelf

import "testing"

// Lookup answers from the loaded index and entries alone: a source that was
// resolved is found, and anything the cache has no record of is a clean miss.
func TestFingerprintCacheLookup(t *testing.T) {
	libRoot := t.TempDir()
	s := newTestShelf(t, &ShelfConf{LibRoot: libRoot, LockMode: "none"})

	book := addFingerprintBook(t, s, libRoot, "Dune", "the spice must flow")

	builder := &fakeFingerprint{label: "v1"}
	cache := openFingerprintCache(t, s, testAlgo)
	want := resolveFingerprint(t, cache, writeRootForTest(t, s), book.FolderPath(), builder)

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

// FingerprintStatus counts coverage from the cache and the in-memory book list
// without opening a single source.txt - the property the maintenance page's
// status bar depends on, since it loads on every visit.
func TestFingerprintStatusReadsNoSources(t *testing.T) {
	libRoot := t.TempDir()
	s := newTestShelf(t, &ShelfConf{LibRoot: libRoot, LockMode: "none"})

	dune := addFingerprintBook(t, s, libRoot, "Dune", "the spice must flow")
	addFingerprintBook(t, s, libRoot, "Solaris", "the ocean thinks in shapes")

	// Fingerprint one of the two books so the count has both a hit and a miss.
	builder := &fakeFingerprint{label: "v1"}
	cache := openFingerprintCache(t, s, testAlgo)
	resolveFingerprint(t, cache, writeRootForTest(t, s), dune.FolderPath(), builder)
	saveFingerprintCache(t, cache)

	// From here on, any source.txt open is one this status read caused.
	counting := countSourceReads(t, s)

	status, err := s.FingerprintStatus(testAlgo)
	if err != nil {
		t.Fatalf("FingerprintStatus: %v", err)
	}

	if got := counting.opens.Load(); got != 0 {
		t.Errorf("FingerprintStatus opened source.txt %d times, want 0", got)
	}
	if status.Total != 2 || status.Fingerprinted != 1 || status.Missing != 1 {
		t.Errorf("status = %+v, want total 2, fingerprinted 1, missing 1", status)
	}
	if status.Algo != testAlgo {
		t.Errorf("status algo = %+v, want %+v", status.Algo, testAlgo)
	}
}

// A cache built under different rules answers for nothing: every book reads as
// missing, which is what tells the UI to offer a full rebuild.
func TestFingerprintStatusUnderDifferentRulesCountsNothing(t *testing.T) {
	libRoot := t.TempDir()
	s := newTestShelf(t, &ShelfConf{LibRoot: libRoot, LockMode: "none"})

	dune := addFingerprintBook(t, s, libRoot, "Dune", "the spice must flow")

	builder := &fakeFingerprint{label: "v1"}
	cache := openFingerprintCache(t, s, testAlgo)
	resolveFingerprint(t, cache, writeRootForTest(t, s), dune.FolderPath(), builder)
	saveFingerprintCache(t, cache)

	other := FingerprintAlgo{Normalize: "other-v9", Shingle: "char-7gram", Hash: "xxhash64", K: 64}
	status, err := s.FingerprintStatus(other)
	if err != nil {
		t.Fatalf("FingerprintStatus: %v", err)
	}
	if status.Total != 1 || status.Fingerprinted != 0 || status.Missing != 1 {
		t.Errorf("status = %+v, want the book counted as missing under new rules", status)
	}
	if status.Algo != other {
		t.Errorf("status algo = %+v, want the requested %+v", status.Algo, other)
	}
}
