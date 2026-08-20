package textnorm

import (
	"strings"
	"testing"
)

// wrapAt re-flows a paragraph at width runes per line, the way two converters
// configured with different column counts would.
func wrapAt(paragraph string, width int, lineBreak string) string {
	var b strings.Builder
	for i, r := range []rune(paragraph) {
		if i > 0 && i%width == 0 {
			b.WriteString(lineBreak)
		}
		b.WriteRune(r)
	}

	return b.String()
}

// TestNormalizeIgnoresLayout is the property the whole package exists for: the
// same text, laid out four different ways, has to reduce to one string.
func TestNormalizeIgnoresLayout(t *testing.T) {
	paragraphs := []string{
		"話說天下大勢，分久必合，合久必分。",
		"周末七國分爭，并入於秦。",
	}

	// Wrapped narrow, indented with an ideographic space, blank line between
	// paragraphs, LF.
	var narrow strings.Builder
	for _, p := range paragraphs {
		narrow.WriteString(wrapAt("　"+p, 8, "\n"))
		narrow.WriteString("\n\n")
	}

	// Wrapped wide, no indent, CRLF, no trailing blank line.
	var wide strings.Builder
	for i, p := range paragraphs {
		if i > 0 {
			wide.WriteString("\r\n")
		}
		wide.WriteString(wrapAt(p, 17, "\r\n"))
	}

	// One line per paragraph, tab-indented.
	var flat strings.Builder
	for _, p := range paragraphs {
		flat.WriteString("\t" + p + "\n")
	}

	// Every rune on its own line, the pathological case.
	var shredded strings.Builder
	for _, p := range paragraphs {
		shredded.WriteString(wrapAt(p, 1, "\n"))
	}

	want := Normalize(narrow.String())
	if want == "" {
		t.Fatal("Normalize dropped the entire passage")
	}

	for _, tc := range []struct {
		name string
		text string
	}{
		{"wide wrap with CRLF", wide.String()},
		{"one line per paragraph", flat.String()},
		{"one rune per line", shredded.String()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := Normalize(tc.text); got != want {
				t.Errorf("Normalize(%s) = %q, want %q", tc.name, got, want)
			}
		})
	}
}

// TestNormalizeIgnoresQuoteAndPunctuationStyle covers the substitutions a
// converter makes on its own, without the author touching the text.
func TestNormalizeIgnoresQuoteAndPunctuationStyle(t *testing.T) {
	variants := []string{
		"他說：「這是一本書。」",
		"他說：“這是一本書。”",
		"他說:『這是一本書』",
		"他說 ── 「這是一本書」……",
	}

	want := Normalize(variants[0])
	if want != "他說這是一本書" {
		t.Fatalf("Normalize kept punctuation: %q", want)
	}

	for _, v := range variants[1:] {
		if got := Normalize(v); got != want {
			t.Errorf("Normalize(%q) = %q, want %q", v, got, want)
		}
	}
}

// TestNormalizeAppliesNFKC pins the compatibility folding, which is what makes
// fullwidth Latin and its ASCII spelling one string.
func TestNormalizeAppliesNFKC(t *testing.T) {
	testCases := []struct {
		name  string
		input string
		want  string
	}{
		{"fullwidth latin folds to ascii", "ＰｌａｉｎＳｈｅｌｆ", "PlainShelf"},
		{"fullwidth digits fold to ascii", "第１２３章", "第123章"},
		{"ideographic space is layout", "第一章　開始", "第一章開始"},
		{"ligature decomposes", "ﬁle", "file"},
		{"squared abbreviation decomposes to letters", "㌀", "アパート"},
		{"halfwidth kana composes", "ｶﾞ", "ガ"},
		{"combining marks survive", "é", "é"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Normalize(tc.input); got != tc.want {
				t.Errorf("Normalize(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

// TestNormalizeRunsLatinWordsTogether documents the accepted cost of dropping
// whitespace. Latin word boundaries are gone, so two texts that differ only in
// spacing compare equal — including CJK prose whose embedded Latin was spaced
// differently by two translators.
func TestNormalizeRunsLatinWordsTogether(t *testing.T) {
	if got := Normalize("the dog"); got != "thedog" {
		t.Errorf("Normalize(%q) = %q, want %q", "the dog", got, "thedog")
	}

	spaced := Normalize("他讀了 The Great Gatsby 。")
	tight := Normalize("他讀了The Great Gatsby。")
	if spaced != tight {
		t.Errorf("Latin spacing changed the result: %q vs %q", spaced, tight)
	}
	if spaced != "他讀了TheGreatGatsby" {
		t.Errorf("unexpected mixed-script result: %q", spaced)
	}

	// The flip side: a collision Normalize cannot see through. This is the
	// price of the rule above, not a bug to fix in v1.
	if Normalize("a note book") != Normalize("a notebook") {
		t.Error("expected the spacing collision this version accepts")
	}
}

// TestNormalizeComposesAcrossRemovedLayout covers the case that makes stripping
// layout and normalizing a single step rather than two: a line break, or a space
// a converter left behind, can sit between a base character and its combining
// mark. NFKC cannot compose them while it is there, so the pair has to be
// normalized again once the layout is gone — otherwise a wrapped text and the
// same text unwrapped end up as different strings, and hash to different values.
func TestNormalizeComposesAcrossRemovedLayout(t *testing.T) {
	testCases := []struct {
		name  string
		split string
		whole string
	}{
		{"newline before a combining acute", "e\n\u0301", "é"},
		{"space before a combining acute", "e \u0301", "é"},
		{"line break inside a decomposed word", "caf\u0065\n\u0301 au lait", "café au lait"},
		{"halfwidth voiced mark split from its kana", "\uFF76\n\uFF9E", "ガ"},
		{"two marks parted from one base", "\uFF43\u0327 \u0301", "ḉ"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			gotSplit, gotWhole := Normalize(tc.split), Normalize(tc.whole)
			if gotSplit != gotWhole {
				t.Errorf("Normalize(%q) = %q, want %q from Normalize(%q)",
					tc.split, gotSplit, gotWhole, tc.whole)
			}
			if Hash64String(gotSplit) != Hash64String(gotWhole) {
				t.Errorf("layout changed the hash: %#016x vs %#016x",
					Hash64String(gotSplit), Hash64String(gotWhole))
			}
		})
	}
}

// TestNormalizeKeepsChineseVariantsApart pins the deliberate absence of a
// traditional/simplified conversion. Adding one later invalidates every cached
// fingerprint, so it has to arrive with a version bump rather than by accident.
func TestNormalizeKeepsChineseVariantsApart(t *testing.T) {
	traditional := Normalize("三國演義　羅貫中")
	simplified := Normalize("三国演义　罗贯中")

	if traditional == simplified {
		t.Fatalf("Normalize converted between Chinese variants: both gave %q", traditional)
	}
	if traditional != "三國演義羅貫中" {
		t.Errorf("traditional text = %q", traditional)
	}
	if simplified != "三国演义罗贯中" {
		t.Errorf("simplified text = %q", simplified)
	}
}

// TestNormalizeKeepsFormatRunes states a known limit of this version: Unicode
// Cf runes are neither whitespace nor punctuation to Unicode, and NFKC leaves
// them alone, so a byte-order mark or a zero-width space still separates two
// otherwise identical texts. Removing them is a rule change and needs a bump of
// NormalizeVersion.
func TestNormalizeKeepsFormatRunes(t *testing.T) {
	for _, tc := range []struct {
		name  string
		input string
	}{
		{"byte-order mark", "\ufeff一二三"},
		{"zero-width space", "一\u200b二三"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := Normalize(tc.input); got == "一二三" {
				t.Errorf("Normalize(%q) now strips format runes; bump NormalizeVersion and update this test", tc.input)
			}
		})
	}
}

func TestNormalizeEdgeCases(t *testing.T) {
	testCases := []struct {
		name  string
		input string
		want  string
	}{
		{"empty", "", ""},
		{"only layout", "　 \t\r\n「」，。……—", ""},
		{"symbols are dropped", "價格 $100 → ￥700", "價格100700"},
		{"digits and letters survive", "ISBN 978-4-16-790000-0", "ISBN9784167900000"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Normalize(tc.input); got != tc.want {
				t.Errorf("Normalize(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

// TestNormalizeIsIdempotent guards the property a cache depends on: re-running
// Normalize over stored normalized text must not move it.
func TestNormalizeIsIdempotent(t *testing.T) {
	inputs := []string{
		"",
		"話說天下大勢，分久必合。",
		"　ＰｌａｉｎＳｈｅｌｆ　v1.0 ─ the shelf\r\n",
		"㌀ﬁ㍿",
		// A combining mark parted from its base by layout. Removing the break
		// lets NFKC compose the pair, which it could not do while the break was
		// between them, so a single pass would leave this decomposed.
		"e\n\u0301",
		"\uFF43\u0327 \u0301",
	}

	for _, in := range inputs {
		once := Normalize(in)
		if twice := Normalize(once); twice != once {
			t.Errorf("Normalize(%q) is not idempotent: %q then %q", in, once, twice)
		}
	}
}

func TestNormalizeVersionIsSet(t *testing.T) {
	if NormalizeVersion != "nfkc-strip-space-punct-v1" {
		t.Errorf("NormalizeVersion = %q; changing it invalidates every cache built on it", NormalizeVersion)
	}
}
