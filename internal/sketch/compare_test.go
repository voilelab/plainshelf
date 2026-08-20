package sketch

import (
	"math"
	"strings"
	"testing"
)

// jaccardTolerance is the accuracy the default k is chosen to deliver. The
// thresholds this package feeds are graded, so roughly +/-0.05 is enough.
const jaccardTolerance = 0.06

func TestJaccardIsExactForCompleteSketches(t *testing.T) {
	testCases := []struct {
		name string
		a    string
		b    string
	}{
		{name: "identical", a: "the shelf is the source of truth", b: "the shelf is the source of truth"},
		{name: "disjoint", a: "aaaaaaaaaaaaaaaa", b: "bbbbbbbbbbbbbbbb"},
		{name: "overlapping", a: "abcdefghij", b: "defghijklm"},
		{name: "prefix", a: "書架上的每一本書都是純文字檔案", b: "書架上的每一本書"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			a, b := exactSketch(testCase.a), exactSketch(testCase.b)
			if !a.complete() || !b.complete() {
				t.Fatal("fixture sketches are not complete")
			}

			want := bruteForceJaccard(testCase.a, testCase.b, DefaultN)
			if got := Jaccard(a, b); math.Abs(got-want) > 1e-9 {
				t.Errorf("got %v, want the exact %v", got, want)
			}
		})
	}
}

func TestJaccardRejectsIncomparableSketches(t *testing.T) {
	text := strings.Join(randomWords(newRNG(21), 2000), " ")

	if got := Jaccard(Build(text, 4, DefaultK), Build(text, 5, DefaultK)); got != 0 {
		t.Errorf("different shingle widths compared as %v, want 0", got)
	}
	if got := Jaccard(BuildDefault(text), BuildDefault("")); got != 0 {
		t.Errorf("an empty sketch compared as %v, want 0", got)
	}
}

// TestJaccardAccuracyAtDefaultK checks the accuracy the default k buys. A
// bottom-k estimate is a sample, so it is stated over a run of fixtures rather
// than as a bound on one of them: at k=128 a single pair can land about 0.08
// away from the truth, while the average error stays near 0.03.
func TestJaccardAccuracyAtDefaultK(t *testing.T) {
	const trials = 25

	for _, fraction := range []float64{0, 0.02, 0.1, 0.3, 0.6, 1} {
		var totalError, worstError float64

		for seed := uint64(1); seed <= trials; seed++ {
			rng := newRNG(seed * 7919)
			base := randomWords(rng, 2500)
			original := strings.Join(base, " ")
			variant := strings.Join(replaceWords(rng, base, fraction), " ")

			want := bruteForceJaccard(original, variant, DefaultN)
			got := Jaccard(BuildDefault(original), BuildDefault(variant))

			difference := math.Abs(got - want)
			totalError += difference
			worstError = math.Max(worstError, difference)
		}

		meanError := totalError / trials
		t.Logf("replaced %.0f%%: mean error %.4f, worst %.4f", fraction*100, meanError, worstError)

		if meanError > jaccardTolerance {
			t.Errorf("replaced %.0f%%: mean error %.4f exceeds %.2f", fraction*100, meanError, jaccardTolerance)
		}
		if worstError > 2*jaccardTolerance {
			t.Errorf("replaced %.0f%%: worst error %.4f exceeds %.2f", fraction*100, worstError, 2*jaccardTolerance)
		}
	}
}

func TestContainmentIsAlgebraic(t *testing.T) {
	rng := newRNG(99)
	whole := strings.Join(randomWords(rng, 800), " ")
	part := whole[:len(whole)*3/4]

	a, b := exactSketch(whole), exactSketch(part)
	if !a.complete() || !b.complete() {
		t.Fatal("fixture sketches are not complete")
	}

	inWhole, inPart := Containment(a, b)

	if math.Abs(inPart-1) > 1e-9 {
		t.Errorf("the truncation is not reported as fully contained: got %v", inPart)
	}
	if inWhole >= 1 || inWhole <= 0.5 {
		t.Errorf("the whole document should be partly contained in its truncation, got %v", inWhole)
	}
}

// TestContainmentAtDefaultK covers the same claim through sampled sketches.
// Containment is algebra, but it is algebra over an estimated Jaccard, so a
// single pair inherits that estimate's spread; the average stays on the true
// value because errors above 1 are clamped away.
func TestContainmentAtDefaultK(t *testing.T) {
	const trials = 25

	var total, worst float64
	worst = 1

	for seed := uint64(1); seed <= trials; seed++ {
		rng := newRNG(seed * 104729)
		words := randomWords(rng, 2500)
		whole := strings.Join(words, " ")
		part := strings.Join(words[:len(words)*3/4], " ")

		_, inPart := Containment(BuildDefault(whole), BuildDefault(part))
		total += inPart
		worst = math.Min(worst, inPart)
	}

	mean := total / trials
	t.Logf("truncation at 25%%: mean containment %.4f, worst %.4f", mean, worst)

	if mean < 0.98 {
		t.Errorf("mean containment %.4f, want at least 0.98", mean)
	}
	if worst < 0.9 {
		t.Errorf("worst containment %.4f, want at least 0.90", worst)
	}
}

func TestContainmentEdgeCases(t *testing.T) {
	text := strings.Join(randomWords(newRNG(5), 1000), " ")

	if a, b := Containment(BuildDefault(text), BuildDefault("")); a != 0 || b != 0 {
		t.Errorf("an empty sketch gave containment (%v, %v), want (0, 0)", a, b)
	}

	unrelatedA := strings.Join(randomWords(newRNG(6), 1000), " ")
	unrelatedB := strings.Join(randomWords(newRNG(7), 1000), " ")
	if a, b := Containment(exactSketch(unrelatedA), exactSketch(unrelatedB)); a > 0.2 || b > 0.2 {
		t.Errorf("unrelated documents gave containment (%v, %v)", a, b)
	}
}

func TestMaxJaccardBoundsTheTrueJaccard(t *testing.T) {
	rng := newRNG(4242)
	base := randomWords(rng, 2000)

	for _, length := range []int{20, 200, 2000} {
		for _, fraction := range []float64{0, 0.25, 0.75, 1} {
			original := strings.Join(base[:length], " ")
			variant := strings.Join(replaceWords(rng, base, fraction), " ")

			a, b := exactSketch(original), exactSketch(variant)
			bound := MaxJaccard(a.Distinct, b.Distinct)
			actual := Jaccard(a, b)

			if bound < actual {
				t.Errorf("length %d, replaced %.0f%%: bound %.4f is below the actual %.4f",
					length, fraction*100, bound, actual)
			}
		}
	}
}

func TestMaxJaccardDegenerateInputs(t *testing.T) {
	testCases := []struct {
		name   string
		na, nb int
		want   float64
	}{
		{name: "both empty", na: 0, nb: 0, want: 1},
		{name: "one empty", na: 0, nb: 500, want: 0},
		{name: "equal", na: 500, nb: 500, want: 1},
		{name: "short against long", na: 100, nb: 1000, want: 0.1},
		{name: "negative", na: -5, nb: 100, want: 0},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := MaxJaccard(testCase.na, testCase.nb); math.Abs(got-testCase.want) > 1e-9 {
				t.Errorf("got %v, want %v", got, testCase.want)
			}
		})
	}
}

// bruteForceJaccard compares the two full shingle sets directly.
func bruteForceJaccard(a, b string, n int) float64 {
	setA, setB := map[uint64]struct{}{}, map[uint64]struct{}{}
	eachShingle(a, n, func(hash uint64) { setA[hash] = struct{}{} })
	eachShingle(b, n, func(hash uint64) { setB[hash] = struct{}{} })

	intersection := 0
	for hash := range setA {
		if _, shared := setB[hash]; shared {
			intersection++
		}
	}

	union := len(setA) + len(setB) - intersection
	if union == 0 {
		return 0
	}

	return float64(intersection) / float64(union)
}
