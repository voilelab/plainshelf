package server

import (
	"slices"
	"testing"
)

func TestNormalizeBookBatchIDsPreservesFirstOccurrence(t *testing.T) {
	got, err := normalizeBookBatchIDs([]string{" b ", "a", "b"})
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}
	if !slices.Equal(got, []string{"b", "a"}) {
		t.Errorf("got %v, want [b a]", got)
	}
}
