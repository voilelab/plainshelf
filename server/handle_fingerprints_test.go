package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/voilelab/plainshelf/internal/sketch"
	"github.com/voilelab/plainshelf/server/task"
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

// fingerprintTestApp builds a started app on a caller-known LibRoot. The other
// server tests let newTestApp pick a temp dir, but these need the path so they
// can backdate source files: the fingerprint cache refuses to index a file
// written moments ago, and findSimilarBooks reads only indexed fingerprints.
func fingerprintTestApp(t *testing.T, libRoot string) *App {
	t.Helper()

	app, err := NewApp(&AppConf{
		Shelves: []*shelf.ShelfConfWithID{
			{ID: "default_shelf", ShelfConf: shelf.ShelfConf{LibRoot: libRoot}},
		},
		StorePath:  t.TempDir(),
		CoverToJPG: false,
	})
	if err != nil {
		t.Fatalf("NewApp: %v", err)
	}
	t.Cleanup(func() {
		if err := app.Close(); err != nil {
			t.Fatalf("Close app: %v", err)
		}
	})
	if err := app.Start(); err != nil {
		t.Fatalf("Start app: %v", err)
	}
	waitForShelves(t, app)

	return app
}

// makeBook creates a book with one source and backdates that source, so the
// fingerprint cache will index it. An un-backdated source still fingerprints but
// never lands in the index findSimilarBooks reads from (see server/task's
// addFingerprintBook for the same dance).
func makeBook(t *testing.T, shelfData *shelf.ShelfData, libRoot, title, content string) *shelf.Book {
	t.Helper()

	book, err := shelfData.NewBookWith(nil, title, func(b *shelf.Book) error {
		source, err := b.NewSource(strings.NewReader(content))
		if err != nil {
			return err
		}
		return b.SetCurrentSource(source.ID())
	})
	if err != nil {
		t.Fatalf("NewBookWith(%q): %v", title, err)
	}

	source, err := book.GetSource(book.CurrentSource())
	if err != nil {
		t.Fatalf("GetSource: %v", err)
	}
	old := time.Now().Add(-time.Hour)
	sourcePath := path.Join(libRoot, source.FolderPath(), shelf.SourceFile)
	if err := os.Chtimes(sourcePath, old, old); err != nil {
		t.Fatalf("backdating %s: %v", sourcePath, err)
	}
	return book
}

// fingerprintBooks resolves and persists each book's current-source fingerprint,
// so findSimilarBooks' own cache Open finds them on record. Books passed to
// makeBook but not here stay unfingerprinted and are skipped by the sweep.
func fingerprintBooks(t *testing.T, shelfData *shelf.ShelfData, books ...*shelf.Book) {
	t.Helper()

	cache, err := shelfData.OpenFingerprintCache(task.FingerprintAlgo())
	if err != nil {
		t.Fatalf("OpenFingerprintCache: %v", err)
	}
	for _, book := range books {
		source, err := book.GetSource(book.CurrentSource())
		if err != nil {
			t.Fatalf("GetSource: %v", err)
		}
		if _, err := cache.Resolve(book, source, task.BuildFingerprint); err != nil {
			t.Fatalf("Resolve fingerprint: %v", err)
		}
	}
	if err := cache.Save(); err != nil {
		t.Fatalf("Save fingerprint cache: %v", err)
	}
}

// variedText builds a string of distinct tokens, so its sketch keeps roughly one
// shingle per token rather than collapsing to the few a repeated phrase yields.
// Under sketch.ExactShingleLimit runes the sketch retains every shingle, which is
// what makes a "short" shelf cost more per pair than a book count would suggest.
func variedText(tokens int) string {
	var b strings.Builder
	for i := range tokens {
		b.WriteString("term")
		b.WriteString(strconv.Itoa(i))
		b.WriteByte(' ')
	}
	return b.String()
}

func decodeTooLarge(t *testing.T, rec *httptest.ResponseRecorder) similarTooLarge {
	t.Helper()

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", rec.Code, rec.Body.String())
	}
	var body similarTooLarge
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decoding rejection body %q: %v", rec.Body.String(), err)
	}
	return body
}

func getSimilar(t *testing.T, app *App) *httptest.ResponseRecorder {
	t.Helper()
	return getSimilarWithQuery(t, app, "")
}

func getSimilarWithQuery(t *testing.T, app *App, query string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, "/api/shelves/default_shelf/books/similar"+query, nil)
	rec := httptest.NewRecorder()
	app.Handler().ServeHTTP(rec, req)
	return rec
}

type deadlineRecorder struct {
	*httptest.ResponseRecorder
	writeDeadline time.Time
}

func (r *deadlineRecorder) SetWriteDeadline(deadline time.Time) error {
	r.writeDeadline = deadline
	return nil
}

// A confirmed scan can legitimately outlive the server's ordinary 60-second
// WriteTimeout, so the handler must move the server-side deadline as well as the
// frontend moving its fetch deadline.
func TestFindSimilarBooksConfirmExtendsServerWriteDeadline(t *testing.T) {
	libRoot := t.TempDir()
	app := fingerprintTestApp(t, libRoot)

	start := time.Now()
	rec := &deadlineRecorder{ResponseRecorder: httptest.NewRecorder()}
	req := httptest.NewRequest(http.MethodGet, "/api/shelves/default_shelf/books/similar?confirm=1", nil)
	app.Handler().ServeHTTP(rec, req)

	if rec.writeDeadline.IsZero() {
		t.Fatal("confirmed request did not set a server write deadline")
	}
	minimum := start.Add(confirmedSimilarTimeout - time.Second)
	maximum := time.Now().Add(confirmedSimilarTimeout + time.Second)
	if rec.writeDeadline.Before(minimum) || rec.writeDeadline.After(maximum) {
		t.Errorf("write deadline = %v, want approximately now + %v", rec.writeDeadline, confirmedSimilarTimeout)
	}
}

// A shelf whose fingerprints cost more than the budget is declined with 200 and
// the too_large body, not run through the sweep to a timeout. This is the old
// over-limit test, now measuring the work budget instead of a book count.
func TestFindSimilarBooksOverBudgetReturns200(t *testing.T) {
	libRoot := t.TempDir()
	app := fingerprintTestApp(t, libRoot)

	shelfData, ok := app.ShelfManager().GetShelf("default_shelf")
	if !ok {
		t.Fatal("default shelf not found")
	}
	books := []*shelf.Book{
		makeBook(t, shelfData, libRoot, "Book 0", "content number zero"),
		makeBook(t, shelfData, libRoot, "Book 1", "content number one"),
	}
	fingerprintBooks(t, shelfData, books...)

	restore := similarWorkBudget
	similarWorkBudget = 1
	defer func() { similarWorkBudget = restore }()

	body := decodeTooLarge(t, getSimilar(t, app))
	if body.Status != "too_large" || body.Budget != 1 || body.Work <= body.Budget {
		t.Errorf("body = %+v, want too_large with budget 1 and work over budget", body)
	}
	if body.Total != 2 || body.Fingerprinted != 2 || body.Pairs != 1 || body.Seconds < 1 {
		t.Errorf("estimate = %+v, want 2 total, 2 fingerprinted, 1 pair, and a positive duration", body)
	}
}

// Confirmation only releases the budget gate: it must run the same sweep with
// the same floor and return the same pair array that a higher budget would.
func TestFindSimilarBooksConfirmBypassesBudgetWithoutChangingPairs(t *testing.T) {
	libRoot := t.TempDir()
	app := fingerprintTestApp(t, libRoot)

	shelfData, ok := app.ShelfManager().GetShelf("default_shelf")
	if !ok {
		t.Fatal("default shelf not found")
	}
	content := variedText(200)
	books := []*shelf.Book{
		makeBook(t, shelfData, libRoot, "Book A", content),
		makeBook(t, shelfData, libRoot, "Book B", content),
	}
	fingerprintBooks(t, shelfData, books...)

	restore := similarWorkBudget
	defer func() { similarWorkBudget = restore }()
	similarWorkBudget = 1

	if body := decodeTooLarge(t, getSimilarWithQuery(t, app, "?confirm=0")); body.Work <= body.Budget {
		t.Fatalf("unconfirmed body = %+v, want an over-budget estimate", body)
	}

	decodePairs := func(rec *httptest.ResponseRecorder) []similarPair {
		t.Helper()
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body %s", rec.Code, rec.Body.String())
		}
		var pairs []similarPair
		if err := json.Unmarshal(rec.Body.Bytes(), &pairs); err != nil {
			t.Fatalf("decoding pairs %q: %v", rec.Body.String(), err)
		}
		return pairs
	}

	confirmed := decodePairs(getSimilarWithQuery(t, app, "?confirm=1"))
	similarWorkBudget = 1 << 30
	unlimited := decodePairs(getSimilar(t, app))
	if !reflect.DeepEqual(confirmed, unlimited) {
		t.Errorf("confirmed pairs = %+v, higher-budget pairs = %+v", confirmed, unlimited)
	}
}

// The gate measures sketch length, not book count: under one budget a shelf of
// two short works is compared while a shelf of two longer works - identical in
// number - is declined, because its sketches keep far more shingles.
func TestFindSimilarBooksBudgetTracksSketchLengthNotBookCount(t *testing.T) {
	restore := similarWorkBudget
	similarWorkBudget = 1500
	defer func() { similarWorkBudget = restore }()

	shelfWithTexts := func(texts ...string) *httptest.ResponseRecorder {
		t.Helper()
		libRoot := t.TempDir()
		app := fingerprintTestApp(t, libRoot)
		shelfData, ok := app.ShelfManager().GetShelf("default_shelf")
		if !ok {
			t.Fatal("default shelf not found")
		}
		books := make([]*shelf.Book, 0, len(texts))
		for i, text := range texts {
			books = append(books, makeBook(t, shelfData, libRoot, "Book "+strconv.Itoa(i), text))
		}
		fingerprintBooks(t, shelfData, books...)
		return getSimilar(t, app)
	}

	// Two short books: well under the budget, so the pairs array comes back.
	shortRec := shelfWithTexts("tiny zero text", "tiny one text")
	if shortRec.Code != http.StatusOK {
		t.Fatalf("short shelf status = %d, want 200; body %s", shortRec.Code, shortRec.Body.String())
	}
	if strings.Contains(shortRec.Body.String(), "too_large") {
		t.Errorf("short shelf was declined, want a pairs array; body %s", shortRec.Body.String())
	}

	// Two books of the same count but long, distinct text: their summed sketch
	// length pushes the merge steps past the budget, so this shelf is declined.
	longRec := shelfWithTexts("first "+variedText(400), "second "+variedText(400))
	body := decodeTooLarge(t, longRec)
	if body.Status != "too_large" || body.Work <= body.Budget {
		t.Errorf("long shelf body = %+v, want too_large with work over budget %d", body, similarWorkBudget)
	}
}

// Books without a fingerprint are skipped, so a shelf of many books but few
// fingerprints is compared - and returned 200 with pairs - rather than declined
// on its raw count.
func TestFindSimilarBooksSkipsUnfingerprintedBooks(t *testing.T) {
	libRoot := t.TempDir()
	app := fingerprintTestApp(t, libRoot)

	shelfData, ok := app.ShelfManager().GetShelf("default_shelf")
	if !ok {
		t.Fatal("default shelf not found")
	}

	// Two books share one text so they form exactly one pair; four more books
	// exist but are never fingerprinted.
	shared := "the spice must flow across the dune sea"
	a := makeBook(t, shelfData, libRoot, "A", shared)
	b := makeBook(t, shelfData, libRoot, "B", shared)
	for i := range 4 {
		makeBook(t, shelfData, libRoot, "Unfingerprinted "+strconv.Itoa(i), "unique text "+strconv.Itoa(i))
	}
	fingerprintBooks(t, shelfData, a, b)

	rec := getSimilar(t, app)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", rec.Code, rec.Body.String())
	}

	var pairs []similarPair
	if err := json.Unmarshal(rec.Body.Bytes(), &pairs); err != nil {
		t.Fatalf("decoding pairs %q: %v", rec.Body.String(), err)
	}
	if len(pairs) != 1 {
		t.Fatalf("got %d pairs, want the single fingerprinted pair: %+v", len(pairs), pairs)
	}
	if pairs[0].Relation != relationIdentical {
		t.Errorf("relation = %q, want %q for two books of identical text", pairs[0].Relation, relationIdentical)
	}
}
