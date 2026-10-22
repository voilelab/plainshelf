package sketch

import "math"

func Jaccard(a, b Sketch) float64 {
	if a.N != b.N || len(a.Values) == 0 || len(b.Values) == 0 {
		return 0
	}

	limit := exhaustiveLimit(a, b)

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

func ContainmentFrom(a, b Sketch, jaccard float64) (float64, float64) {
	if a.Distinct <= 0 || b.Distinct <= 0 {
		return 0, 0
	}
	if jaccard <= 0 {
		return 0, 0
	}

	intersection := jaccard * float64(a.Distinct+b.Distinct) / (1 + jaccard)

	return clamp01(intersection / float64(a.Distinct)),
		clamp01(intersection / float64(b.Distinct))
}

func MaxJaccard(na, nb int) float64 {
	na, nb = max(na, 0), max(nb, 0)
	if na == 0 && nb == 0 {
		return 1
	}

	return float64(min(na, nb)) / float64(max(na, nb))
}

func clamp01(value float64) float64 {
	return min(max(value, 0), 1)
}

func exhaustiveLimit(a, b Sketch) uint64 {
	limit := uint64(math.MaxUint64)
	if !a.complete() {
		limit = min(limit, a.Values[len(a.Values)-1])
	}
	if !b.complete() {
		limit = min(limit, b.Values[len(b.Values)-1])
	}

	return limit
}
