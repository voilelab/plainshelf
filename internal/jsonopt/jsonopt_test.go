package jsonopt_test

import (
	"encoding/json/v2"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/voilelab/plainshelf/internal/jsonopt"
)

// marshalRounds is how many times each option set is exercised. One round
// cannot tell a deterministic marshal from a lucky one: Go randomises map
// iteration per range, so a set that lost json.Deterministic would still sort
// its keys correctly about 1 time in n!. Repeating collapses that to nothing.
const marshalRounds = 20

// mapKeys is wide enough that an unordered marshal cannot plausibly agree with
// the sorted one, and the keys are deliberately not inserted in sorted order.
var mapKeys = []string{"zeta", "alpha", "mu", "beta", "omega", "delta", "kappa", "gamma"}

func unsortedMap() map[string]int {
	m := make(map[string]int, len(mapKeys))
	for i, key := range mapKeys {
		m[key] = i
	}
	return m
}

/*
Determinism is the reason this package exists, and the reason it is tested:
three mechanisms are built on "same content, same bytes" — the fingerprint
cache compares its encoded file against the one on disk before rewriting it,
the exported book cache is keyed by a digest of its own payload, and the scan
cache does the same. Dropping json.Deterministic breaks all three silently,
with no compile error and, until this file existed, no test failure. The cost
is a pointless re-upload per scan for a shelf held on pCloud or SMB.
*/
func TestEveryOptionSetMarshalsMapsDeterministically(t *testing.T) {
	sets := map[string]json.Options{
		"Disk":        jsonopt.Disk(),
		"DiskCompact": jsonopt.DiskCompact(),
		"API":         jsonopt.API(),
	}

	for name, opts := range sets {
		t.Run(name, func(t *testing.T) {
			var first string
			for round := range marshalRounds {
				bs, err := json.Marshal(unsortedMap(), opts)
				if err != nil {
					t.Fatalf("marshal: %v", err)
				}
				if round == 0 {
					first = string(bs)
					continue
				}
				if got := string(bs); got != first {
					t.Fatalf("round %d = %s, round 0 = %s; map order must not vary", round, got, first)
				}
			}

			if got := keyOrder(t, first); !isSorted(got) {
				t.Errorf("key order = %v, want it sorted", got)
			}
		})
	}
}

// The indentation is the one difference between Disk and DiskCompact, and it is
// what makes book.json and the exported cache openable in a text editor. Pinned
// so a merge of the two sets is a test failure rather than a silent reformat of
// every file on the shelf.
func TestDiskIndentsAndTheMachineOnlySetsDoNot(t *testing.T) {
	indented, err := json.Marshal(unsortedMap(), jsonopt.Disk())
	if err != nil {
		t.Fatalf("marshal with Disk: %v", err)
	}
	if !strings.Contains(string(indented), "\n  \"alpha\"") {
		t.Errorf("Disk output = %s, want two-space indented lines", indented)
	}

	for name, opts := range map[string]json.Options{
		"DiskCompact": jsonopt.DiskCompact(),
		"API":         jsonopt.API(),
	} {
		bs, err := json.Marshal(unsortedMap(), opts)
		if err != nil {
			t.Fatalf("marshal with %s: %v", name, err)
		}
		if strings.ContainsAny(string(bs), "\n ") {
			t.Errorf("%s output = %s, want it on a single line", name, bs)
		}
	}
}

// keyOrder returns the object's member names in the order they were written.
func keyOrder(t *testing.T, encoded string) []string {
	t.Helper()

	positions := make(map[string]int, len(mapKeys))
	for _, key := range mapKeys {
		index := strings.Index(encoded, fmt.Sprintf("%q", key))
		if index < 0 {
			t.Fatalf("key %q missing from %s", key, encoded)
		}
		positions[key] = index
	}

	names := slices.Clone(mapKeys)
	slices.SortFunc(names, func(a, b string) int { return positions[a] - positions[b] })
	return names
}

func isSorted(keys []string) bool {
	for i := 1; i < len(keys); i++ {
		if keys[i-1] >= keys[i] {
			return false
		}
	}
	return true
}
