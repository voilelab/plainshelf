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

// Normalize reduces text to the characters that carry its content: it applies
// NFKC, then drops every whitespace, punctuation, symbol and separator rune.
// What survives is letters, digits and marks.
//
// Two layouts of one text therefore normalize to one string. A chapter rewrapped
// at a different width, corner brackets traded for curly quotes, an ideographic
// space indenting every paragraph — all of it is gone before the comparison
// starts. Format conversions are a weaker case: an EPUB rendered to text tends to
// gain a table of contents or a colophon, and the extra headings are content, so
// the results stay different. Exact equality after Normalize is a narrow, zero
// false positive fast path, not a replacement for a similarity sketch.
//
// Two consequences are deliberate and worth stating, because a caller comparing
// results will meet both:
//
// Latin words run together. "the dog" and "thedog" normalize alike, and so do
// two CJK passages that differ only in where a translator put the Latin spaces.
// Dropping whitespace is what makes CJK reflowing invisible, and CJK is the
// script PlainShelf is built around; the cost lands on Latin text, where word
// boundaries are usually recoverable from the letters anyway.
//
// Traditional and simplified Chinese stay apart. 「三國」 and 「三国」 normalize
// to different strings. Converting between them is a word-level problem that
// needs a dictionary, and a wrong conversion would silently merge two books that
// are not the same edition.
//
// Zero-width and other format runes (Unicode Cf, including a byte-order mark)
// survive this version. NFKC does not remove them and they are not whitespace by
// any Unicode category, so a stray BOM still separates two otherwise identical
// texts. Stripping them is a rule change, so it belongs to a version bump rather
// than to this function.
//
// Normalize takes the whole text at once so it can share the single read the
// caller already makes for the other content metrics, rather than costing a
// second pass over a network-mounted shelf.
func Normalize(s string) string {
	folded := norm.NFKC.String(s)

	var b strings.Builder
	b.Grow(len(folded))
	for _, r := range folded {
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
