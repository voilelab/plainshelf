package contract_test

import (
	"net/http"
	"testing"
)

// The similarity endpoints share the fingerprint-task helpers in
// api_source_fingerprints_contract_test.go: a pair only appears once the task
// has written the fingerprints these read.

type similarPairResponse struct {
	A string `json:"a"`
	B string `json:"b"`

	Jaccard      float64 `json:"jaccard"`
	ContainmentA float64 `json:"containment_a"`
	ContainmentB float64 `json:"containment_b"`

	NormCharsA int `json:"norm_chars_a"`
	NormCharsB int `json:"norm_chars_b"`

	Relation string `json:"relation"`
}

type fingerprintStatusResponse struct {
	Total         int `json:"total"`
	Fingerprinted int `json:"fingerprinted"`
	Missing       int `json:"missing"`
	Algo          struct {
		Normalize string `json:"normalize"`
		Shingle   string `json:"shingle"`
		Hash      string `json:"hash"`
		K         int    `json:"k"`
	} `json:"algo"`
}

func similarURL() string {
	return shelfURL("books", "similar")
}

func fingerprintStatusURL() string {
	return shelfURL("fingerprints", "status")
}

// sharedProse is long enough to yield many distinct 5-gram shingles, so two
// books built on top of it overlap heavily while a third built on other words
// does not.
const sharedProse = "the quick brown fox jumps over the lazy dog while the industrious " +
	"bee gathers nectar from a thousand summer flowers and the river winds " +
	"slowly toward the distant glimmering sea beneath an indifferent sky " +
	"the quick brown fox jumps over the lazy dog while the industrious " +
	"bee gathers nectar from a thousand summer flowers and the river winds " +
	"slowly toward the distant glimmering sea beneath an indifferent sky "

func sortedPair(a, b string) (string, string) {
	if a <= b {
		return a, b
	}
	return b, a
}

// Two books that share most of their text surface as one similar pair; a third,
// unrelated book does not pair with either at the default floor.
func TestAPISimilarBooksContract(t *testing.T) {
	env := newAPITestEnv(t)

	left := importTextBook(t, env, "Left", "", "left.txt", sharedProse+"and a unique closing passage belonging only to the left book")
	right := importTextBook(t, env, "Right", "", "right.txt", sharedProse+"and a different closing passage belonging only to the right book")
	stranger := importTextBook(t, env, "Stranger", "", "stranger.txt",
		"utterly unrelated vocabulary weaving mercury zephyr quartz lantern across every improbable clause")

	for _, book := range []string{left.Meta.ID, right.Meta.ID, stranger.Meta.ID} {
		backdateSources(t, env, book)
	}
	runFingerprintSources(t, env)

	pairs := getJSON[[]similarPairResponse](t, env, similarURL())
	if len(pairs) != 1 {
		t.Fatalf("got %d pairs, want exactly the left/right pair: %+v", len(pairs), pairs)
	}

	pair := pairs[0]
	wantA, wantB := sortedPair(left.Meta.ID, right.Meta.ID)
	if pair.A != wantA || pair.B != wantB {
		t.Errorf("pair = (%q, %q), want the sorted left/right pair (%q, %q)", pair.A, pair.B, wantA, wantB)
	}
	if pair.Jaccard < defaultSimilarFloorContract {
		t.Errorf("jaccard = %v, want >= the default floor %v", pair.Jaccard, defaultSimilarFloorContract)
	}
	if !knownRelation(pair.Relation) {
		t.Errorf("relation = %q, want one of the four defined values", pair.Relation)
	}
	if pair.NormCharsA <= 0 || pair.NormCharsB <= 0 {
		t.Errorf("norm chars = (%d, %d), want both positive", pair.NormCharsA, pair.NormCharsB)
	}
	if pair.A == stranger.Meta.ID || pair.B == stranger.Meta.ID {
		t.Errorf("the unrelated book appears in a pair: %+v", pair)
	}
}

// The result count only grows as the floor widens: a stricter floor is always a
// subset of a looser one, which is what lets the page switch tiers in memory.
func TestAPISimilarBooksFloorContract(t *testing.T) {
	env := newAPITestEnv(t)

	left := importTextBook(t, env, "Left", "", "left.txt", sharedProse+"unique left tail")
	right := importTextBook(t, env, "Right", "", "right.txt", sharedProse+"unique right tail")
	for _, book := range []string{left.Meta.ID, right.Meta.ID} {
		backdateSources(t, env, book)
	}
	runFingerprintSources(t, env)

	wide := getJSON[[]similarPairResponse](t, env, similarURL()+"?floor=0")
	narrow := getJSON[[]similarPairResponse](t, env, similarURL()+"?floor=1")
	if len(narrow) > len(wide) {
		t.Errorf("floor=1 returned %d pairs, more than floor=0's %d", len(narrow), len(wide))
	}

	// A floor that is not a number in [0, 1] is a client error, not a silent
	// default.
	for _, bad := range []string{"abc", "2", "-0.5"} {
		if rec := env.get(similarURL() + "?floor=" + bad); rec.Code != http.StatusBadRequest {
			t.Errorf("floor=%s gave status %d, want 400", bad, rec.Code)
		}
	}
}

// Before anything is fingerprinted every book is missing; after the task runs,
// none is. The endpoint reports the algorithm the count was taken under.
func TestAPIFingerprintStatusContract(t *testing.T) {
	env := newAPITestEnv(t)

	first := importTextBook(t, env, "First", "", "first.txt", "the spice must flow")
	second := importTextBook(t, env, "Second", "", "second.txt", "the ocean thinks in shapes")

	before := getJSON[fingerprintStatusResponse](t, env, fingerprintStatusURL())
	if before.Total != 2 || before.Fingerprinted != 0 || before.Missing != 2 {
		t.Errorf("before = %+v, want two books all missing", before)
	}
	if before.Algo.Shingle != "char-5gram" || before.Algo.K != 128 {
		t.Errorf("algo = %+v, want the char-5gram/k=128 ruleset", before.Algo)
	}
	if before.Algo.Normalize == "" || before.Algo.Hash == "" {
		t.Errorf("algo = %+v, want the normalize and hash versions named", before.Algo)
	}

	for _, book := range []string{first.Meta.ID, second.Meta.ID} {
		backdateSources(t, env, book)
	}
	runFingerprintSources(t, env)

	after := getJSON[fingerprintStatusResponse](t, env, fingerprintStatusURL())
	if after.Total != 2 || after.Fingerprinted != 2 || after.Missing != 0 {
		t.Errorf("after = %+v, want both books fingerprinted", after)
	}
}

// defaultSimilarFloorContract mirrors the server's default floor; the contract
// package cannot see the unexported server constant.
const defaultSimilarFloorContract = 0.15

func knownRelation(relation string) bool {
	switch relation {
	case "identical_after_normalize", "subset", "near_identical", "same_source":
		return true
	default:
		return false
	}
}
