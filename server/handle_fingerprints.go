package server

import (
	"math"
	"net/http"
	"sort"
	"strconv"

	"github.com/voilelab/plainshelf/internal/sketch"
	"github.com/voilelab/plainshelf/server/task"
)

// fingerprintHandlers answers the two read-only endpoints the similarity page
// needs: the pairwise comparison and the coverage count. Both read the
// fingerprint cache under app/ and never a source.txt, so neither is gated
// behind the read-only check - a read-only shelf still compares whatever
// fingerprints it already holds.
type fingerprintHandlers struct {
	*apiCore
}

// defaultSimilarFloor is the Jaccard similarity below which a pair is not worth
// returning. The endpoint returns every pair at or above it in one response and
// the frontend narrows from there, so this is the widest tier the UI offers:
// wide enough that two independent transcripts of one audiobook still surface,
// far enough above the ~0.02 background of two unrelated novels that the noise
// does not.
const defaultSimilarFloor = 0.15

// similarBookLimit caps how many books the synchronous comparison will scan.
// The comparison is O(n^2) in the book count; at a few thousand books that is
// still well under a second, but past this a request could outlast its
// deadline, so the endpoint answers 202 instead of timing out. It is a var, not
// a const, so a test can lower it without standing up thousands of books.
var similarBookLimit = 3200

// The relation names describe how two sources are alike. The server decides
// this rather than the frontend so the classification lives in one place, and a
// future pCloud reader that computes similarity on its own can share the same
// vocabulary instead of reinventing the thresholds.
const (
	relationIdentical     = "identical_after_normalize"
	relationSubset        = "subset"
	relationNearIdentical = "near_identical"
	relationSameSource    = "same_source"
)

const (
	// subsetContainmentThreshold is how much of one source must lie inside the
	// other for the pair to read as one being an edited-down version of it.
	subsetContainmentThreshold = 0.95

	// subsetJaccardCeiling keeps a pair that is already near-identical overall
	// out of the subset bucket: containment is ~1 in both directions there, and
	// "one is a subset of the other" is only informative when they differ in
	// size.
	subsetJaccardCeiling = 0.9

	// nearIdenticalThreshold is the Jaccard at which two sources are the same
	// text bar small edits - a format conversion, a truncation, a few scattered
	// deletions.
	nearIdenticalThreshold = 0.85
)

// similarPair is one comparison the similarity page renders. A and B are book
// IDs in a stable order (A < B) so a pair is reported once. ContainmentA is how
// much of A lies inside B; for a subset relation one of the two is close to 1.
type similarPair struct {
	A string `json:"a"`
	B string `json:"b"`

	Jaccard      float64 `json:"jaccard"`
	ContainmentA float64 `json:"containment_a"`
	ContainmentB float64 `json:"containment_b"`

	NormCharsA int `json:"norm_chars_a"`
	NormCharsB int `json:"norm_chars_b"`

	Relation string `json:"relation"`
}

// similarTooLarge is the 202 body for a shelf past similarBookLimit: the
// synchronous comparison was declined rather than run past the request's
// deadline. It reports the counts so the caller can explain the refusal.
type similarTooLarge struct {
	Status string `json:"status"`
	Total  int    `json:"total"`
	Limit  int    `json:"limit"`
}

// bookSketch is a book paired with the decoded fingerprint of its current
// source, the unit the pairwise comparison works over.
type bookSketch struct {
	bookID    string
	normMD5   string
	normChars int
	sketch    sketch.Sketch
}

// classifyRelation names how two sources relate, checked most-specific first:
// identical normalized content wins over everything; then a near-total
// containment that is not yet near-identical, which is one source edited down
// from the other; then a high overall similarity; and otherwise a bare match
// above the floor.
func classifyRelation(sameNormMD5 bool, jaccard, maxContainment float64) string {
	switch {
	case sameNormMD5:
		return relationIdentical
	case maxContainment >= subsetContainmentThreshold && jaccard < subsetJaccardCeiling:
		return relationSubset
	case jaccard >= nearIdenticalThreshold:
		return relationNearIdentical
	default:
		return relationSameSource
	}
}

// buildSimilarPairs compares every pair of fingerprints once and keeps those at
// or above floor. The books are sorted by ID first so a pair is emitted in a
// stable order and only once, and MaxJaccard rules out a length-mismatched pair
// with a single integer comparison before the sketch intersection is computed.
func buildSimilarPairs(prints []bookSketch, floor float64) []similarPair {
	sorted := append([]bookSketch(nil), prints...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].bookID < sorted[j].bookID })

	pairs := []similarPair{}
	for i := range sorted {
		for j := i + 1; j < len(sorted); j++ {
			a, b := sorted[i], sorted[j]

			if sketch.MaxJaccard(a.sketch.Distinct, b.sketch.Distinct) < floor {
				continue
			}

			jaccard := sketch.Jaccard(a.sketch, b.sketch)
			if jaccard < floor {
				continue
			}

			containmentA, containmentB := sketch.Containment(a.sketch, b.sketch)
			pairs = append(pairs, similarPair{
				A:            a.bookID,
				B:            b.bookID,
				Jaccard:      jaccard,
				ContainmentA: containmentA,
				ContainmentB: containmentB,
				NormCharsA:   a.normChars,
				NormCharsB:   b.normChars,
				Relation:     classifyRelation(a.normMD5 == b.normMD5, jaccard, max(containmentA, containmentB)),
			})
		}
	}
	return pairs
}

// parseSimilarFloor reads the floor query parameter, defaulting to
// defaultSimilarFloor when it is absent and rejecting a value that is not a
// number in [0, 1].
func parseSimilarFloor(r *http.Request) (float64, bool) {
	raw := r.URL.Query().Get("floor")
	if raw == "" {
		return defaultSimilarFloor, true
	}
	floor, err := strconv.ParseFloat(raw, 64)
	// NaN slips past both range comparisons (every comparison against it is
	// false), and a NaN floor then passes every later "jaccard < floor" test too,
	// so it would return every pair and run the full O(n^2) sweep. Reject any
	// non-finite value outright.
	if err != nil || math.IsNaN(floor) || math.IsInf(floor, 0) || floor < 0 || floor > 1 {
		return 0, false
	}
	return floor, true
}

// GET /api/shelves/{shelf_id}/books/similar
func (h *fingerprintHandlers) findSimilarBooks(w http.ResponseWriter, r *http.Request) {
	shelfData, ok := h.resolveShelf(w, r)
	if !ok {
		return
	}

	floor, ok := parseSimilarFloor(r)
	if !ok {
		http.Error(w, "invalid floor", http.StatusBadRequest)
		return
	}

	books, err := shelfData.ListBooks()
	if err != nil {
		h.writeErr(w, err, "failed to list books")
		return
	}

	// A library past the limit would spend too long in the O(n^2) sweep to
	// answer within the request's deadline, so it is told to come back for an
	// asynchronous result rather than left to time out.
	if len(books) > similarBookLimit {
		h.writeJSON(w, http.StatusAccepted, similarTooLarge{Status: "too_large", Total: len(books), Limit: similarBookLimit})
		return
	}

	cache, err := shelfData.OpenFingerprintCache(task.FingerprintAlgo())
	if err != nil {
		h.writeErr(w, err, "failed to open the fingerprint cache")
		return
	}

	prints := make([]bookSketch, 0, len(books))
	for _, book := range books {
		entry, ok := cache.Lookup(book.ID(), book.CurrentSource())
		if !ok {
			// A book without a fingerprint yet is skipped, not failed: the sweep
			// still compares every book that does have one, which is what lets the
			// page show results while some books remain unfingerprinted.
			continue
		}

		decoded, err := sketch.Decode(entry.Sketch)
		if err != nil {
			// A stored sketch this build cannot decode is one book's problem, not
			// the whole comparison's; log it and leave that book out.
			h.Warn("skipping a book whose stored sketch could not be decoded",
				"shelf_id", shelfData.ID, "book_id", book.ID(), "error", err)
			continue
		}

		prints = append(prints, bookSketch{
			bookID:    book.ID(),
			normMD5:   entry.NormMD5,
			normChars: entry.NormChars,
			sketch:    decoded,
		})
	}

	h.writeJSON(w, http.StatusOK, buildSimilarPairs(prints, floor))
}

// GET /api/shelves/{shelf_id}/fingerprints/status
func (h *fingerprintHandlers) getFingerprintStatus(w http.ResponseWriter, r *http.Request) {
	shelfData, ok := h.resolveShelf(w, r)
	if !ok {
		return
	}

	status, err := shelfData.FingerprintStatus(task.FingerprintAlgo())
	if err != nil {
		h.writeErr(w, err, "failed to read fingerprint status")
		return
	}

	h.writeJSON(w, http.StatusOK, status)
}
