package textnorm

import (
	"strings"
	"testing"

	"github.com/cespare/xxhash/v2"
)

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

func TestNormalizeIgnoresLayout(t *testing.T) {
	paragraphs := []string{
		"話說天下大勢，分久必合，合久必分。",
		"周末七國分爭，并入於秦。",
	}

	var narrow strings.Builder
	for _, p := range paragraphs {
		narrow.WriteString(wrapAt("　"+p, 8, "\n"))
		narrow.WriteString("\n\n")
	}

	var wide strings.Builder
	for i, p := range paragraphs {
		if i > 0 {
			wide.WriteString("\r\n")
		}
		wide.WriteString(wrapAt(p, 17, "\r\n"))
	}

	var flat strings.Builder
	for _, p := range paragraphs {
		flat.WriteString("\t" + p + "\n")
	}

	var shredded strings.Builder
	for _, p := range paragraphs {
		shredded.WriteString(wrapAt(p, 1, "\n"))
	}

	const want = "話說天下大勢分久必合合久必分周末七國分爭并入於秦"

	for _, tc := range []struct {
		name string
		text string
	}{
		{"narrow wrap with LF and ideographic indent", narrow.String()},
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

	if Normalize("a note book") != Normalize("a notebook") {
		t.Error("expected the spacing collision this version accepts")
	}
}

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
			if xxhash.Sum64String(gotSplit) != xxhash.Sum64String(gotWhole) {
				t.Errorf("layout changed the hash: %#016x vs %#016x",
					xxhash.Sum64String(gotSplit), xxhash.Sum64String(gotWhole))
			}
		})
	}
}

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

func TestNormalizeIsIdempotent(t *testing.T) {
	inputs := []string{
		"",
		"話說天下大勢，分久必合。",
		"　ＰｌａｉｎＳｈｅｌｆ　v1.0 ─ the shelf\r\n",
		"㌀ﬁ㍿",
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

// TestNormalizeKeepsSupplementaryRunes pins the x/text v0.42.0 fix behind v2:
// older NFKC truncated a supplementary rune in its composition lookup, so
// U+10041 plus an acute came out as "Á".
func TestNormalizeKeepsSupplementaryRunes(t *testing.T) {
	const in = "\U00010041\u0301"
	if got := Normalize(in); got != in {
		t.Errorf("Normalize(%q) = %q, want it unchanged", in, got)
	}
}

func TestNormalizeVersionIsSet(t *testing.T) {
	if NormalizeVersion != "nfkc-strip-space-punct-v2" {
		t.Errorf("NormalizeVersion = %q; changing it invalidates every cache built on it", NormalizeVersion)
	}
}

func TestNormalizeGoldenOutputsBelongToNormalizeVersion(t *testing.T) {
	const goldenVersion = "nfkc-strip-space-punct-v2"
	if NormalizeVersion != goldenVersion {
		t.Fatalf("NormalizeVersion is %q but this table was recorded under %q; regenerate it for the new version", NormalizeVersion, goldenVersion)
	}
	for _, tc := range []struct{ input, want string }{
		{"Hello, World!", "HelloWorld"},
		{"三國演義　第一回\r\n", "三國演義第一回"},
		{"ｆｕｌｌ　ｗｉｄｔｈ", "fullwidth"},
		{"“quoted” — text…", "quotedtext"},
		{"é", "é"},
		{"a\tb\nc", "abc"},
	} {
		if got := Normalize(tc.input); got != tc.want {
			t.Errorf("Normalize(%q) = %q, want %q; the output changed, so bump NormalizeVersion and regenerate this table", tc.input, got, tc.want)
		}
	}
}
