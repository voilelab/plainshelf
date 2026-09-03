package testutil

import (
	jsonv2 "encoding/json/v2"
	"testing"
)

// marshalRepeats is how many times AssertMarshalIsDeterministic re-marshals a
// value. Go randomizes map iteration, so a payload with a handful of map keys
// that comes back byte-identical this many times in a row is sorted, not lucky.
// internal/jsonopt's TestOptionsWithoutDeterministicVaryTheOrder is the control
// that keeps this number honest.
const marshalRepeats = 64

// AssertMarshalIsDeterministic fails t unless marshaling value with opts
// produces the same bytes every time.
//
// It exists because the alternative failure is invisible: a payload marshaled
// without json.Deterministic still compiles, still round-trips, and still
// passes a single-run golden test. What it does instead is defeat the two
// "unchanged content, do not rewrite" checks in the shelf — fingerprint.Cache's
// byte comparison and the exported book cache's digest — so a user whose shelf
// lives on pCloud or SMB uploads the same file again on every scan.
//
// Pass the payload as it is written to disk, not a simplified stand-in: the
// point is to cover the map that is actually there.
func AssertMarshalIsDeterministic(t *testing.T, name string, value any, opts jsonv2.Options) {
	t.Helper()

	want, err := jsonv2.Marshal(value, opts)
	if err != nil {
		t.Fatalf("Failed to marshal %s: %v", name, err)
	}
	for i := range marshalRepeats {
		got, err := jsonv2.Marshal(value, opts)
		if err != nil {
			t.Fatalf("Failed to marshal %s on attempt %d: %v", name, i, err)
		}
		if string(got) != string(want) {
			t.Fatalf("Marshaling %s twice produced different bytes on attempt %d; the options are missing json.Deterministic\n want %s\n  got %s", name, i, want, got)
		}
	}
}
