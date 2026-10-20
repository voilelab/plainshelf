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
const NormalizeVersion = "nfkc-strip-space-punct-v2"

func Normalize(s string) string {
	for range maxNormalizeRounds {
		stripped := stripLayout(norm.NFKC.String(s))
		if stripped == s {
			break
		}
		s = stripped
	}

	return s
}

const maxNormalizeRounds = 4

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
