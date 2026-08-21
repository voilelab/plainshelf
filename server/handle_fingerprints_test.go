package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/voilelab/plainshelf/internal/sketch"
	"github.com/voilelab/plainshelf/shelf"
)

func TestClassifyRelation(t *testing.T) {
	cases := []struct {
		name           string
		sameNormMD5    bool
		jaccard        float64
		maxContainment float64
		want           string
	}{
		{"identical content wins over everything", true, 1, 1, relationIdentical},
		{"identical even when the sketches barely overlap", true, 0.10, 0.10, relationIdentical},
		{"subset at the containment boundary", false, 0.50, subsetContainmentThreshold, relationSubset},
		{"subset just under the jaccard ceiling", false, 0.89, 0.99, relationSubset},
		{"jaccard at the ceiling is near-identical, not subset", false, subsetJaccardCeiling, 1, relationNearIdentical},
		{"near-identical at its boundary", false, nearIdenticalThreshold, 0.10, relationNearIdentical},
		{"just under near-identical falls to same source", false, 0.84, 0.10, relationSameSource},
		{"containment under the boundary is not a subset", false, 0.50, 0.94, relationSameSource},
		{"a bare match above the floor is same source", false, 0.30, 0.40, relationSameSource},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := classifyRelation(tc.sameNormMD5, tc.jaccard, tc.maxContainment); got != tc.want {
				t.Errorf("classifyRelation(%v, %v, %v) = %q, want %q",
					tc.sameNormMD5, tc.jaccard, tc.maxContainment, got, tc.want)
			}
		})
	}
}

func TestBuildSimilarPairs(t *testing.T) {
	common := sketch.BuildDefault(strings.Repeat("alpha beta gamma delta epsilon zeta ", 30))
	other := sketch.BuildDefault(strings.Repeat("mille nulla porta quisque vivamus lorem ", 30))

	// b2 and a1 hold the same content under the same normalized hash; c3 is
	// unrelated. The input order is deliberately unsorted.
	prints := []bookSketch{
		{bookID: "b2", normMD5: "same", normChars: 900, sketch: common},
		{bookID: "a1", normMD5: "same", normChars: 900, sketch: common},
		{bookID: "c3", normMD5: "diff", normChars: 950, sketch: other},
	}

	got := buildSimilarPairs(prints, defaultSimilarFloor)
	if len(got) != 1 {
		t.Fatalf("got %d pairs, want only the identical pair: %+v", len(got), got)
	}

	pair := got[0]
	if pair.A != "a1" || pair.B != "b2" {
		t.Errorf("pair order = (%q, %q), want the IDs sorted (a1, b2)", pair.A, pair.B)
	}
	if pair.Jaccard < 0.99 {
		t.Errorf("jaccard = %v, want ~1 for identical content", pair.Jaccard)
	}
	if pair.Relation != relationIdentical {
		t.Errorf("relation = %q, want %q for a shared norm_md5", pair.Relation, relationIdentical)
	}
	if pair.NormCharsA != 900 || pair.NormCharsB != 900 {
		t.Errorf("norm chars = (%d, %d), want (900, 900)", pair.NormCharsA, pair.NormCharsB)
	}

	// Dropping the floor to zero admits the unrelated pairs too, so every one of
	// the three combinations appears exactly once - never a book against itself,
	// never a pair twice.
	all := buildSimilarPairs(prints, 0)
	if len(all) != 3 {
		t.Fatalf("got %d pairs at floor 0, want all three combinations: %+v", len(all), all)
	}
	seen := map[string]bool{}
	for _, p := range all {
		if p.A >= p.B {
			t.Errorf("pair (%q, %q) is not in ascending order", p.A, p.B)
		}
		key := p.A + "|" + p.B
		if seen[key] {
			t.Errorf("pair %s appears more than once", key)
		}
		seen[key] = true
	}
}

// The count returned rises monotonically as the floor widens: a stricter floor
// can only ever be a subset of a looser one, which is the property the page's
// tier switching relies on.
func TestBuildSimilarPairsMonotonicInFloor(t *testing.T) {
	prints := []bookSketch{
		{bookID: "a", normMD5: "1", normChars: 900, sketch: sketch.BuildDefault(strings.Repeat("alpha beta gamma delta ", 40))},
		{bookID: "b", normMD5: "2", normChars: 900, sketch: sketch.BuildDefault(strings.Repeat("alpha beta gamma omega ", 40))},
		{bookID: "c", normMD5: "3", normChars: 900, sketch: sketch.BuildDefault(strings.Repeat("kappa lambda mu nu ", 40))},
	}

	wide := len(buildSimilarPairs(prints, 0.10))
	narrow := len(buildSimilarPairs(prints, 0.85))
	if narrow > wide {
		t.Errorf("a stricter floor returned more pairs (%d) than a looser one (%d)", narrow, wide)
	}
}

func TestParseSimilarFloor(t *testing.T) {
	cases := []struct {
		query  string
		want   float64
		wantOK bool
	}{
		{"", defaultSimilarFloor, true},
		{"?floor=0.5", 0.5, true},
		{"?floor=0", 0, true},
		{"?floor=1", 1, true},
		{"?floor=abc", 0, false},
		{"?floor=1.5", 0, false},
		{"?floor=-0.1", 0, false},
		{"?floor=NaN", 0, false},
		{"?floor=Inf", 0, false},
		{"?floor=-Inf", 0, false},
	}

	for _, tc := range cases {
		req := httptest.NewRequest(http.MethodGet, "/api/shelves/s/books/similar"+tc.query, nil)
		got, ok := parseSimilarFloor(req)
		if ok != tc.wantOK || (ok && got != tc.want) {
			t.Errorf("parseSimilarFloor(%q) = (%v, %v), want (%v, %v)", tc.query, got, ok, tc.want, tc.wantOK)
		}
	}
}

// A shelf past the limit is answered 202 with the counts, not run through the
// O(n^2) sweep to a timeout.
func TestFindSimilarBooksOverLimitReturns202(t *testing.T) {
	app := newTestApp(t)

	shelfData, ok := app.ShelfManager().GetShelf("default_shelf")
	if !ok {
		t.Fatal("default shelf not found")
	}
	for i := 0; i < 2; i++ {
		if _, err := shelfData.NewBookWith(nil, "Book "+strconv.Itoa(i), func(b *shelf.Book) error {
			source, err := b.NewSource(strings.NewReader("content number " + strconv.Itoa(i)))
			if err != nil {
				return err
			}
			return b.SetCurrentSource(source.ID())
		}); err != nil {
			t.Fatalf("NewBookWith: %v", err)
		}
	}

	restore := similarBookLimit
	similarBookLimit = 1
	defer func() { similarBookLimit = restore }()

	req := httptest.NewRequest(http.MethodGet, "/api/shelves/default_shelf/books/similar", nil)
	rec := httptest.NewRecorder()
	app.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202; body %s", rec.Code, rec.Body.String())
	}

	var body similarTooLarge
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decoding 202 body %q: %v", rec.Body.String(), err)
	}
	if body.Status != "too_large" || body.Total != 2 || body.Limit != 1 {
		t.Errorf("body = %+v, want too_large with total 2 and limit 1", body)
	}
}
