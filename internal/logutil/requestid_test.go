package logutil

import (
	"context"
	"strings"
	"testing"
)

const confusables = "0O1IL"

func TestRequestIDAlphabetHasNoConfusables(t *testing.T) {
	for _, c := range confusables {
		if strings.ContainsRune(requestIDAlphabet, c) {
			t.Errorf("alphabet %q contains %q, which a user cannot reliably tell from its twin", requestIDAlphabet, c)
		}
	}

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

func TestRequestIDUsesOnlyAlphabet(t *testing.T) {
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

	if got := RequestIDFrom(context.Background()); got != "" {
		t.Errorf("RequestIDFrom(plain context) = %q, want empty", got)
	}
	if got := RequestIDFrom(nil); got != "" { //nolint:staticcheck // a nil context is what a zero-value caller passes
		t.Errorf("RequestIDFrom(nil) = %q, want empty", got)
	}
}
