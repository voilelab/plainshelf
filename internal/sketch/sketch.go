// Package sketch reduces a normalized document to a fixed-size fingerprint and
// compares fingerprints.
//
// The fingerprint is a bottom-k MinHash sketch: the k smallest 64-bit hashes of
// the document's distinct character n-grams ("shingles"). Cohen and Kaplan
// (2007) showed that keeping the k smallest values of one hash function is both
// more accurate and cheaper than the textbook construction of k independent
// hash functions.
//
// Every function here is pure, and the only thing it needs beyond the standard
// library is the generic heap in internal/container, so a sketch can be built,
// stored and compared anywhere.
package sketch

import (
	"slices"
	"unicode/utf8"

	"github.com/voilelab/plainshelf/internal/container"
)

const (
	// DefaultN is the shingle width, in runes. Wider shingles separate related
	// documents from unrelated ones more sharply but weaken the signal itself;
	// 5 is the measured balance point for book-length prose.
	DefaultN = 5

	// DefaultK is the number of hashes a sketch retains. It bounds the stored
	// size only: the cost of building a sketch is dominated by the shingle
	// scan, not by k. At 128 the Jaccard error is roughly +/-0.05.
	DefaultK = 128

	// ExactShingleLimit is the document length, in runes, below which
	// BuildDefault retains every shingle instead of a bottom-k sample. Below
	// this size the sampling noise starts to swamp the signal, and the document
	// is small enough that comparing it exactly costs nothing.
	ExactShingleLimit = 5000
)

// Sketch is the fingerprint of one document.
//
// Values holds the smallest distinct shingle hashes in ascending order, at most
// k of them. Distinct is the exact number of distinct shingles in the document,
// which is what makes ContainmentFrom algebra rather than estimation; deriving it
// from Values instead (KMV cardinality estimation) costs up to 16% at k=128.
//
// Two sketches are only comparable when they were built with the same N.
// A sketch whose Values hold every shingle it counted, meaning
// len(Values) == Distinct, compares exactly rather than approximately.
type Sketch struct {
	N        int
	Distinct int
	Values   []uint64
}

// complete reports whether Values lists every distinct shingle of the document,
// in which case membership below any threshold is known exactly.
func (s Sketch) complete() bool {
	return len(s.Values) == s.Distinct
}

// BuildDefault builds a sketch with the project defaults, retaining every
// shingle of a short document so that it is compared exactly. See Build.
func BuildDefault(normalized string) Sketch {
	k := DefaultK
	if size := utf8.RuneCountInString(normalized); size < ExactShingleLimit {
		// A document of size runes has fewer than size shingles, so this keeps
		// all of them without capping.
		k = size + 1
	}

	return Build(normalized, DefaultN, k)
}

// Build reduces normalized to the bottom-k sketch of its n-rune shingles.
//
// It streams: the text is scanned once and only the k smallest hashes are kept,
// so the sketch itself stays at O(k) whatever the document size. The exact
// distinct count cannot be obtained in sublinear space, so the scan does keep a
// set of the hashes it has already seen; that set is transient and is an order
// of magnitude smaller than the normalized text it is derived from.
//
// n and k below 1 fall back to DefaultN and DefaultK.
func Build(normalized string, n, k int) Sketch {
	if n < 1 {
		n = DefaultN
	}
	if k < 1 {
		k = DefaultK
	}

	seen := map[uint64]struct{}{}
	// bottom is a max-heap of the k smallest hashes seen so far, so the hash to
	// evict is always at the root. Cap the initial allocation: k may be the
	// document length for a short document, or an arbitrary caller value.
	bottom := container.HeapFrom(make([]uint64, 0, min(k, 1024)), container.Greater[uint64])

	eachShingle(normalized, n, func(hash uint64) {
		if _, duplicate := seen[hash]; duplicate {
			return
		}
		seen[hash] = struct{}{}

		if bottom.Len() < k {
			bottom.Push(hash)

			return
		}
		if largest, _ := bottom.Peek(); hash < largest {
			bottom.Replace(hash)
		}
	})

	values := bottom.Items()
	slices.Sort(values)

	return Sketch{N: n, Distinct: len(seen), Values: values}
}
