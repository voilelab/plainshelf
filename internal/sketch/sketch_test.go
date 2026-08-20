package sketch

import (
	"slices"
	"strings"
	"testing"
)

// deterministicRNG is a xorshift64 generator, so that every fixture in this
// package is reproducible independently of the standard library's generators.
type deterministicRNG struct {
	state uint64
}

func newRNG(seed uint64) *deterministicRNG {
	if seed == 0 {
		seed = 1
	}

	return &deterministicRNG{state: seed}
}

func (r *deterministicRNG) next() uint64 {
	r.state ^= r.state << 13
	r.state ^= r.state >> 7
	r.state ^= r.state << 17

	return r.state
}

func (r *deterministicRNG) intn(n int) int {
	return int(r.next() % uint64(n))
}

// randomWords builds a reproducible document of space-separated pseudo-words.
func randomWords(r *deterministicRNG, count int) []string {
	words := make([]string, count)
	for i := range words {
		var word strings.Builder
		for range 3 + r.intn(4) {
			word.WriteByte(byte('a' + r.intn(26)))
		}
		words[i] = word.String()
	}

	return words
}

// replaceWords returns a copy of words with the given fraction replaced, which
// is how the fixtures below reach a known, tunable amount of overlap.
func replaceWords(r *deterministicRNG, words []string, fraction float64) []string {
	changed := slices.Clone(words)
	for i := range changed {
		if float64(r.intn(1000)) < fraction*1000 {
			changed[i] = randomWords(r, 1)[0]
		}
	}

	return changed
}

// exactSketch retains every shingle, so comparisons between exact sketches are
// set operations rather than estimates.
func exactSketch(text string) Sketch {
	return Build(text, DefaultN, len(text)+1)
}

// referenceSketch is the obvious implementation the streaming one must match:
// collect every shingle hash, sort, deduplicate, keep the k smallest.
func referenceSketch(text string, n, k int) Sketch {
	var all []uint64
	eachShingle(text, n, func(hash uint64) {
		all = append(all, hash)
	})
	slices.Sort(all)
	distinct := slices.Compact(all)

	values := distinct
	if len(values) > k {
		values = values[:k]
	}

	return Sketch{N: n, Distinct: len(distinct), Values: slices.Clone(values)}
}

func TestEachShingle(t *testing.T) {
	// collect lists the n-rune windows of text the obvious way, by materializing
	// the runes, which is what the streaming ring buffer must reproduce.
	collect := func(text string, n int) []string {
		var windows []string
		runes := []rune(text)
		for i := 0; i+n <= len(runes); i++ {
			windows = append(windows, string(runes[i:i+n]))
		}

		return windows
	}

	testCases := []struct {
		name string
		text string
		n    int
	}{
		{name: "ascii", text: "abcdef", n: 3},
		{name: "cjk", text: "書架上的每一本書", n: 5},
		{name: "mixed width", text: "a書b架c上d", n: 2},
		{name: "window equals length", text: "abcde", n: 5},
		{name: "window longer than text", text: "abc", n: 5},
		{name: "single rune window", text: "書架", n: 1},
		{name: "empty", text: "", n: 5},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			want := collect(testCase.text, testCase.n)

			var got []uint64
			eachShingle(testCase.text, testCase.n, func(hash uint64) {
				got = append(got, hash)
			})

			if len(got) != len(want) {
				t.Fatalf("got %d shingles, want %d", len(got), len(want))
			}

			for i, window := range want {
				var wantHash uint64
				eachShingle(window, testCase.n, func(hash uint64) {
					wantHash = hash
				})
				if got[i] != wantHash {
					t.Errorf("shingle %d (%q): got hash %d, want %d", i, window, got[i], wantHash)
				}
			}
		})
	}
}

func TestBuildMatchesFullSetReference(t *testing.T) {
	rng := newRNG(20260820)
	long := strings.Join(randomWords(rng, 3000), " ")

	texts := map[string]string{
		"long prose":       long,
		"repeating":        strings.Repeat("the shelf is the source of truth. ", 200),
		"cjk":              strings.Repeat("書架上的每一本書都是純文字檔案，", 300),
		"shorter than n":   "abc",
		"empty":            "",
		"single character": strings.Repeat("a", 400),
	}

	for name, text := range texts {
		for _, n := range []int{3, 5, 8} {
			for _, k := range []int{1, 16, 128, 100000} {
				t.Run(name, func(t *testing.T) {
					got := Build(text, n, k)
					want := referenceSketch(text, n, k)

					if got.Distinct != want.Distinct {
						t.Errorf("n=%d k=%d: got %d distinct shingles, want %d", n, k, got.Distinct, want.Distinct)
					}
					if !slices.Equal(got.Values, want.Values) {
						t.Errorf("n=%d k=%d: got %d values, want %d; first mismatch differs",
							n, k, len(got.Values), len(want.Values))
					}
					if got.N != n {
						t.Errorf("got N=%d, want %d", got.N, n)
					}
				})
			}
		}
	}
}

func TestBuildValuesAreSortedAndBounded(t *testing.T) {
	rng := newRNG(7)
	text := strings.Join(randomWords(rng, 5000), " ")

	sketch := Build(text, DefaultN, DefaultK)

	if len(sketch.Values) != DefaultK {
		t.Fatalf("got %d values, want %d", len(sketch.Values), DefaultK)
	}
	if !slices.IsSorted(sketch.Values) {
		t.Error("values are not sorted")
	}
	if sketch.Distinct <= DefaultK {
		t.Fatalf("fixture is too small to truncate: %d distinct shingles", sketch.Distinct)
	}
	if sketch.complete() {
		t.Error("a truncated sketch reports itself complete")
	}
}

func TestBuildFallsBackToDefaults(t *testing.T) {
	text := strings.Join(randomWords(newRNG(11), 2000), " ")

	fallback := Build(text, 0, -1)
	explicit := Build(text, DefaultN, DefaultK)

	if fallback.N != DefaultN {
		t.Errorf("got N=%d, want %d", fallback.N, DefaultN)
	}
	if !slices.Equal(fallback.Values, explicit.Values) {
		t.Error("invalid n and k did not fall back to the defaults")
	}
}

func TestBuildDefaultKeepsShortDocumentsExact(t *testing.T) {
	short := strings.Join(randomWords(newRNG(3), 300), " ")
	long := strings.Join(randomWords(newRNG(4), 5000), " ")

	if size := len([]rune(short)); size >= ExactShingleLimit {
		t.Fatalf("short fixture is %d runes, expected below %d", size, ExactShingleLimit)
	}
	if size := len([]rune(long)); size < ExactShingleLimit {
		t.Fatalf("long fixture is %d runes, expected at least %d", size, ExactShingleLimit)
	}

	shortSketch := BuildDefault(short)
	if !shortSketch.complete() {
		t.Errorf("short document was sampled: %d of %d shingles retained",
			len(shortSketch.Values), shortSketch.Distinct)
	}

	longSketch := BuildDefault(long)
	if len(longSketch.Values) != DefaultK {
		t.Errorf("long document retained %d shingles, want %d", len(longSketch.Values), DefaultK)
	}
	if longSketch.N != DefaultN {
		t.Errorf("got N=%d, want %d", longSketch.N, DefaultN)
	}

	empty := BuildDefault("")
	if empty.Distinct != 0 || len(empty.Values) != 0 {
		t.Errorf("empty document produced %+v", empty)
	}
}
