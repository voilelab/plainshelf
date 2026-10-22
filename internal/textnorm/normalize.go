package textnorm

import (
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

const NormalizeVersion = "nfkc-strip-space-punct-v2"

func Normalize(s string) string {
	return normalizeToFixpoint(s, maxNormalizeRounds)
}

const maxNormalizeRounds = 4

func normalizeToFixpoint(s string, maxRounds int) string {
	for range maxRounds {
		stripped := stripLayout(norm.NFKC.String(s))
		if stripped == s {
			break
		}
		s = stripped
	}

	return s
}

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

func isLayout(r rune) bool {
	if unicode.IsSpace(r) {
		return true
	}

	return unicode.In(r, unicode.P, unicode.S, unicode.Z)
}
