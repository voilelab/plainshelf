package sketch

import "math"

// Jaccard estimates the Jaccard similarity of the two documents, that is the
// size of the intersection of their shingle sets over the size of their union.
//
// It returns 0 for sketches that cannot be compared: sketches built with
// different shingle widths, and empty sketches.
//
// The estimate is exact when both sketches are complete. Otherwise it is read
// off the merged sketches below the smaller of their two largest retained
// hashes, because below that threshold both sketches still list their
// document's shingles exhaustively and membership is decided, not guessed.
func Jaccard(a, b Sketch) float64 {
	if a.N != b.N || len(a.Values) == 0 || len(b.Values) == 0 {
		return 0
	}

	limit := uint64(math.MaxUint64)
	if !a.complete() {
		limit = min(limit, a.Values[len(a.Values)-1])
	}
	if !b.complete() {
		limit = min(limit, b.Values[len(b.Values)-1])
	}

	var union, intersection int
	i, j := 0, 0
	for (i < len(a.Values) && a.Values[i] <= limit) || (j < len(b.Values) && b.Values[j] <= limit) {
		switch {
		case j >= len(b.Values) || b.Values[j] > limit:
			i++
		case i >= len(a.Values) || a.Values[i] > limit:
			j++
		case a.Values[i] == b.Values[j]:
			i++
			j++
			intersection++
		case a.Values[i] < b.Values[j]:
			i++
		default:
			j++
		}
		union++
	}

	if union == 0 {
		return 0
	}

	return float64(intersection) / float64(union)
}

// Containment reports how much of a lies inside b, and how much of b lies
// inside a. A value of 1 means the document is entirely covered by the other.
//
// Given the Jaccard similarity J and the two exact distinct shingle counts, the
// size of the intersection follows algebraically from
// J = inter / (na + nb - inter), so this adds no estimation of its own beyond
// the error already in J.
func Containment(a, b Sketch) (float64, float64) {
	if a.Distinct <= 0 || b.Distinct <= 0 {
		return 0, 0
	}
	return ContainmentFromJaccard(Jaccard(a, b), a.Distinct, b.Distinct)
}

// ContainmentFromJaccard is Containment given an already-computed Jaccard
// similarity, so a caller that has just merged the two sketches to get J - as
// the similarity sweep does - derives containment from that J rather than
// merging them a second time inside Containment. aDistinct and bDistinct are the
// two sketches' exact distinct shingle counts. Containment is this with J and
// the counts read off the sketches for you.
func ContainmentFromJaccard(jaccard float64, aDistinct, bDistinct int) (float64, float64) {
	if aDistinct <= 0 || bDistinct <= 0 || jaccard <= 0 {
		return 0, 0
	}

	intersection := jaccard * float64(aDistinct+bDistinct) / (1 + jaccard)

	return clamp01(intersection / float64(aDistinct)),
		clamp01(intersection / float64(bDistinct))
}

// MaxJaccard is the largest Jaccard similarity two documents holding na and nb
// distinct shingles can possibly have, because their intersection cannot exceed
// the smaller set and their union cannot be smaller than the larger one.
//
// It is a hard bound on the true similarity, not on an estimate of it. One
// integer comparison rules out a whole pair, which matters for a library whose
// documents vary widely in length.
func MaxJaccard(na, nb int) float64 {
	if na < 0 {
		na = 0
	}
	if nb < 0 {
		nb = 0
	}
	if na == 0 && nb == 0 {
		return 1
	}

	return float64(min(na, nb)) / float64(max(na, nb))
}

func clamp01(value float64) float64 {
	return min(max(value, 0), 1)
}
