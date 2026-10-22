package jsonopt_test

import (
	"encoding/json/v2"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/voilelab/plainshelf/internal/jsonopt"
)

const marshalRounds = 20

var unsortedWideKeys = []string{"zeta", "alpha", "mu", "beta", "omega", "delta", "kappa", "gamma"}

func unsortedMap() map[string]int {
	m := make(map[string]int, len(unsortedWideKeys))
	for i, key := range unsortedWideKeys {
		m[key] = i
	}
	return m
}

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

func keyOrder(t *testing.T, encoded string) []string {
	t.Helper()

	positions := make(map[string]int, len(unsortedWideKeys))
	for _, key := range unsortedWideKeys {
		index := strings.Index(encoded, fmt.Sprintf("%q", key))
		if index < 0 {
			t.Fatalf("key %q missing from %s", key, encoded)
		}
		positions[key] = index
	}

	names := slices.Clone(unsortedWideKeys)
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
