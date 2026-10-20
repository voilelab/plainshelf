package server

import (
	"cmp"
	"errors"
	"math"
	"net/http"
	"slices"
	"strconv"
	"time"

	"github.com/voilelab/plainshelf/internal/sketch"
	"github.com/voilelab/plainshelf/server/task"
	"github.com/voilelab/plainshelf/shelf"
)

type fingerprintHandlers struct {
	*apiCore
}

const defaultSimilarFloor = 0.15

var similarWorkBudget = 1 << 30

const similarMergeStepsPerSecond = 1 << 25

const confirmedSimilarTimeout = 5 * time.Minute

const (
	relationIdentical     = "identical_after_normalize"
	relationSubset        = "subset"
	relationNearIdentical = "near_identical"
	relationSameSource    = "same_source"
)

const (
	subsetContainmentThreshold = 0.95

	subsetJaccardCeiling = 0.9

	nearIdenticalThreshold = 0.85
)

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

type similarTooLarge struct {
	Status        string `json:"status"`
	Total         int    `json:"total"`
	Fingerprinted int    `json:"fingerprinted"`
	Pairs         int    `json:"pairs"`
	Work          int    `json:"work"`
	Seconds       int    `json:"seconds"`
	Budget        int    `json:"budget"`
}

type bookSketch struct {
	bookID    string
	normMD5   string
	normChars int
	sketch    sketch.Sketch
}

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

func buildSimilarPairs(prints []bookSketch, floor float64) []similarPair {
	sorted := slices.Clone(prints)
	slices.SortFunc(sorted, func(a, b bookSketch) int { return cmp.Compare(a.bookID, b.bookID) })

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

			containmentA, containmentB := sketch.ContainmentFrom(a.sketch, b.sketch, jaccard)
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

func parseSimilarFloor(r *http.Request) (float64, bool) {
	raw := r.URL.Query().Get("floor")
	if raw == "" {
		return defaultSimilarFloor, true
	}
	floor, err := strconv.ParseFloat(raw, 64)

	if err != nil || math.IsNaN(floor) || math.IsInf(floor, 0) || floor < 0 || floor > 1 {
		return 0, false
	}
	return floor, true
}

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

	confirmed := r.URL.Query().Get("confirm") == "1"
	if confirmed {
		// http.Server.WriteTimeout installs a connection deadline before this
		// handler runs. Move it for this request only; extending the browser's
		// timeout cannot keep a server-side 60-second deadline alive.
		if err := http.NewResponseController(w).SetWriteDeadline(time.Now().Add(confirmedSimilarTimeout)); err != nil && !errors.Is(err, http.ErrNotSupported) {
			h.Warn("failed to extend the confirmed similarity response deadline", "error", err)
		}
	}

	books, err := h.visibility(shelfData).listBooks()
	if err != nil {
		h.writeErr(w, r, err, "failed to list books")
		return
	}

	cache, err := shelfData.OpenFingerprintCache(task.FingerprintAlgo())
	if err != nil {
		h.writeErr(w, r, err, "failed to open the fingerprint cache")
		return
	}

	prints := make([]bookSketch, 0, len(books))
	sumValues := 0
	for _, listing := range books {
		book := listing.Book
		entry, ok := cache.Lookup(book.ID(), book.CurrentSource())
		if !ok {

			continue
		}

		decoded, err := sketch.Decode(entry.Sketch)
		if err != nil {

			h.Warn("skipping a book whose stored sketch could not be decoded",
				"shelf_id", shelfData.ID, "book_id", book.ID(), "error", err)
			continue
		}

		sumValues += len(decoded.Values)
		prints = append(prints, bookSketch{
			bookID:    book.ID(),
			normMD5:   entry.NormMD5,
			normChars: entry.NormChars,
			sketch:    decoded,
		})
	}

	if work := (len(prints) - 1) * sumValues; work > similarWorkBudget && !confirmed {
		h.writeJSON(w, http.StatusOK, similarTooLarge{
			Status:        "too_large",
			Total:         len(books),
			Fingerprinted: len(prints),
			Pairs:         len(prints) * (len(prints) - 1) / 2,
			Work:          work,
			Seconds:       (work + similarMergeStepsPerSecond - 1) / similarMergeStepsPerSecond,
			Budget:        similarWorkBudget,
		})
		return
	}

	h.writeJSON(w, http.StatusOK, buildSimilarPairs(prints, floor))
}

func (h *fingerprintHandlers) getFingerprintStatus(w http.ResponseWriter, r *http.Request) {
	shelfData, ok := h.resolveShelf(w, r)
	if !ok {
		return
	}

	books, err := h.visibility(shelfData).listBooks()
	if err != nil {
		h.writeErr(w, r, err, "failed to list books")
		return
	}

	refs := make([]shelf.FingerprintBookSource, 0, len(books))
	for _, listing := range books {
		refs = append(refs, shelf.FingerprintBookSource{
			BookID:   listing.Book.ID(),
			SourceID: listing.Book.CurrentSource(),
		})
	}

	status, err := shelfData.FingerprintCoverageFor(task.FingerprintAlgo(), refs)
	if err != nil {
		h.writeErr(w, r, err, "failed to read fingerprint status")
		return
	}

	h.writeJSON(w, http.StatusOK, status)
}
