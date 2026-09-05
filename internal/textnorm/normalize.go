package textnorm

import (
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// NormalizeVersion names the rules Normalize applies. Write it into the algo
// block of every cache derived from normalized text, and bump it whenever
// Normalize's output can change for any input — a mixed cache is worse than no
// cache, because nothing about it looks wrong.
const NormalizeVersion = "nfkc-strip-space-punct-v1"

// Normalize reduces text to the characters that carry its content: NFKC, then
// every whitespace, punctuation, symbol and separator rune dropped, repeated
// until the string stops changing. Two layouts of one text therefore normalize
// alike — a chapter rewrapped, corner brackets traded for curly quotes, an
// ideographic space indenting every paragraph. Format conversions are weaker: an
// EPUB rendered to text tends to gain a table of contents, and the extra
// headings are content. Exact equality here is a zero-false-positive fast path,
// not a replacement for a similarity sketch.
//
// Three consequences a caller comparing results will meet:
//
// Latin words run together — "the dog" and "thedog" normalize alike. Dropping
// whitespace is what makes CJK reflowing invisible, and the cost lands on Latin,
// where word boundaries are recoverable from the letters anyway.
//
// Traditional and simplified Chinese stay apart: 「三國」 and 「三国」 differ.
// Converting needs a dictionary, and a wrong conversion would silently merge two
// books that are not the same edition.
//
// Zero-width and other format runes (Unicode Cf, a byte-order mark included)
// survive this version: NFKC does not remove them and no Unicode category calls
// them whitespace. Stripping them is a rule change, so it belongs to a version
// bump rather than to this function.
//
// Takes the whole text at once so it shares the single read the caller already
// makes for the other content metrics.
func Normalize(s string) string {
	// NFKC runs before the filter, because a compatibility form can expand into
	// punctuation only the filter knows to drop — and after it, because removing
	// a rune can leave a base character next to a combining mark a line break had
	// kept apart. Without the second round, "e\n" + U+0301 and a precomposed "é"
	// normalize differently, which is exactly the difference this erases.
	//
	// One extra round settles every input we know of; the loop makes that a
	// guarantee. It terminates because after the first round each further round
	// only composes, and composing never lengthens the string.
	for range maxNormalizeRounds {
		stripped := stripLayout(norm.NFKC.String(s))
		if stripped == s {
			break
		}
		s = stripped
	}

	return s
}

// maxNormalizeRounds bounds the loop above so a rune we have not thought of
// cannot spin it. Reaching the bound costs idempotence, not determinism:
// Normalize stays a pure function of its input, which is the property every
// cached fingerprint depends on.
const maxNormalizeRounds = 4

// stripLayout removes every rune that describes presentation rather than
// content.
func stripLayout(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if isLayout(r) {
			continue
		}
		b.WriteRune(r)
	}

	return b.String()
}

// isLayout reports whether r describes how text is presented rather than what it
// says. unicode.IsSpace covers the separator categories plus the ASCII control
// runes that end a line, which no category does.
func isLayout(r rune) bool {
	if unicode.IsSpace(r) {
		return true
	}

	return unicode.In(r, unicode.P, unicode.S, unicode.Z)
}
