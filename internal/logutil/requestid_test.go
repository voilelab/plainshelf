package logutil

import (
	"context"
	"strings"
	"testing"
)

// confusables are the characters a request ID must never contain, because it is
// read aloud and copied by hand: 0 against O, and 1 against I and L. Crockford
// base32 already drops I, L and O; the digits are this package's own exclusion,
// and losing either half of a pair puts the ambiguity straight back.
const confusables = "0O1IL"

func TestRequestIDAlphabetHasNoConfusableCharacters(t *testing.T) {
	for _, c := range confusables {
		if strings.ContainsRune(requestIDAlphabet, c) {
			t.Errorf("alphabet %q contains %q, which a user cannot reliably tell from its twin", requestIDAlphabet, c)
		}
	}

	// U is Crockford's own exclusion, kept because an accidental obscenity in a
	// number the user has to read out is worse than the entropy it costs.
	if strings.ContainsRune(requestIDAlphabet, 'U') {
		t.Errorf("alphabet %q contains U, which Crockford base32 excludes", requestIDAlphabet)
	}

	seen := map[rune]bool{}
	for _, c := range requestIDAlphabet {
		if seen[c] {
			t.Errorf("alphabet %q repeats %q", requestIDAlphabet, c)
		}
		seen[c] = true
	}
}

// The alphabet is only half the promise: the generator has to be the thing that
// never emits a confusable, whatever it draws from the random source.
func TestNewRequestIDEmitsOnlyAlphabetCharacters(t *testing.T) {
	for range 2000 {
		id := NewRequestID()

		if len(id) != requestIDLength {
			t.Fatalf("NewRequestID() = %q, want %d characters", id, requestIDLength)
		}
		for _, c := range id {
			if strings.ContainsRune(confusables, c) {
				t.Fatalf("NewRequestID() = %q contains the confusable %q", id, c)
			}
			if !strings.ContainsRune(requestIDAlphabet, c) {
				t.Fatalf("NewRequestID() = %q contains %q, which is outside the alphabet", id, c)
			}
		}
	}
}

// Two requests must not answer with the same number, or a bug report points at
// the wrong log line. Repeats within a small sample mean the generator is not
// drawing from the whole alphabet.
func TestNewRequestIDDoesNotRepeat(t *testing.T) {
	const draws = 5000

	seen := make(map[string]struct{}, draws)
	for range draws {
		id := NewRequestID()
		if _, dup := seen[id]; dup {
			t.Fatalf("NewRequestID() returned %q twice in %d draws", id, draws)
		}
		seen[id] = struct{}{}
	}
}

// Every symbol has to be reachable: an off-by-one in the rejection bound would
// silently shrink the alphabet and the entropy with it.
func TestNewRequestIDReachesEverySymbol(t *testing.T) {
	seen := map[rune]bool{}
	for range 20000 {
		for _, c := range NewRequestID() {
			seen[c] = true
		}
	}

	for _, c := range requestIDAlphabet {
		if !seen[c] {
			t.Errorf("no generated ID ever contained %q", c)
		}
	}
}

func TestRequestIDRoundTripsThroughContext(t *testing.T) {
	id := NewRequestID()

	if got := RequestIDFrom(WithRequestID(context.Background(), id)); got != id {
		t.Errorf("RequestIDFrom = %q, want %q", got, id)
	}

	// A context that never met the middleware - a background job, or a handler
	// the desktop client calls directly - has no ID rather than a wrong one.
	if got := RequestIDFrom(context.Background()); got != "" {
		t.Errorf("RequestIDFrom(plain context) = %q, want empty", got)
	}
	if got := RequestIDFrom(nil); got != "" { //nolint:staticcheck // a nil context is what a zero-value caller passes
		t.Errorf("RequestIDFrom(nil) = %q, want empty", got)
	}
}
