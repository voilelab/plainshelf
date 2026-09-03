package jsonopt

import (
	"encoding/json/jsontext"
	jsonv2 "encoding/json/v2"
	"strings"
	"testing"
)

// repeats is how many times a value is marshaled before a set is called
// deterministic. Go randomizes map iteration, so a 12-key map that comes back
// byte-identical this many times in a row is sorted, not lucky.
const repeats = 64

// wideMap has more keys than a Go map holds in one bucket, so its iteration
// order is randomized rather than incidentally stable.
func wideMap() map[string]int {
	return map[string]int{
		"alpha": 1, "bravo": 2, "charlie": 3, "delta": 4,
		"echo": 5, "foxtrot": 6, "golf": 7, "hotel": 8,
		"india": 9, "juliett": 10, "kilo": 11, "lima": 12,
	}
}

func TestOptionSetsSortMapKeys(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts jsonv2.Options
	}{
		{"Disk", Disk()},
		{"DiskCompact", DiskCompact()},
		{"API", API()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			want, err := jsonv2.Marshal(wideMap(), tc.opts)
			if err != nil {
				t.Fatalf("Failed to marshal: %v", err)
			}
			for i := range repeats {
				got, err := jsonv2.Marshal(wideMap(), tc.opts)
				if err != nil {
					t.Fatalf("Failed to marshal on attempt %d: %v", i, err)
				}
				if string(got) != string(want) {
					t.Fatalf("Attempt %d changed the bytes:\n want %s\n  got %s", i, want, got)
				}
			}
		})
	}
}

// TestOptionsWithoutDeterministicVaryTheOrder is the control for every
// determinism test in the repository: it fails if marshaling an unsorted map
// happens to be stable anyway, which would mean those tests pass without
// proving anything. If this one starts failing because a later Go release
// sorts maps by default, the assertions elsewhere are free — but the option
// still has to stay, because the shelf outlives the toolchain that wrote it.
func TestOptionsWithoutDeterministicVaryTheOrder(t *testing.T) {
	first, err := jsonv2.Marshal(wideMap())
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}
	for range repeats {
		got, err := jsonv2.Marshal(wideMap())
		if err != nil {
			t.Fatalf("Failed to marshal: %v", err)
		}
		if string(got) != string(first) {
			return
		}
	}
	t.Fatalf("Marshaling a %d-key map %d times without Deterministic never changed the order; the determinism assertions elsewhere prove nothing", len(wideMap()), repeats)
}

func TestDiskIndentsAndOthersDoNot(t *testing.T) {
	value := map[string]int{"b": 2, "a": 1}

	indented, err := jsonv2.Marshal(value, Disk())
	if err != nil {
		t.Fatalf("Failed to marshal with Disk: %v", err)
	}
	if want := "{\n  \"a\": 1,\n  \"b\": 2\n}"; string(indented) != want {
		t.Errorf("Disk wrote %q, want %q", indented, want)
	}

	for name, opts := range map[string]jsonv2.Options{"DiskCompact": DiskCompact(), "API": API()} {
		got, err := jsonv2.Marshal(value, opts)
		if err != nil {
			t.Fatalf("Failed to marshal with %s: %v", name, err)
		}
		if want := `{"a":1,"b":2}`; string(got) != want {
			t.Errorf("%s wrote %q, want %q", name, got, want)
		}
	}
}

// TestAcceptedV2DefaultsStayDefault pins the v2 defaults the option sets
// deliberately do not override, so that adopting one later — to chase a golden
// fixture, say — is a visible decision rather than a quiet argument added to
// JoinOptions. See docs/development/json-encoding.md for the reasoning behind
// each row. The assertions are on substrings so that they hold whether or not
// the set indents.
func TestAcceptedV2DefaultsStayDefault(t *testing.T) {
	type payload struct {
		Tags    []string          `json:"tags"`
		IDs     map[string]string `json:"ids"`
		Comment string            `json:"comment"`
	}

	// A nil slice and map encode as [] and {}, not null, and & < > reach the
	// file as themselves: the shelf's selling point is that a text editor shows
	// what the reader typed.
	wants := []string{`"tags": []`, `"ids": {}`, `"comment": "Tom & Jerry <b>"`}

	for name, opts := range map[string]jsonv2.Options{"Disk": Disk(), "DiskCompact": DiskCompact(), "API": API()} {
		t.Run(name, func(t *testing.T) {
			got, err := jsonv2.Marshal(payload{Comment: `Tom & Jerry <b>`}, opts)
			if err != nil {
				t.Fatalf("Failed to marshal: %v", err)
			}
			// Compare without the indentation the set may have added.
			compact := jsontext.Value(got)
			if err := compact.Format(jsontext.SpaceAfterColon(true)); err != nil {
				t.Fatalf("Failed to reformat: %v", err)
			}
			for _, want := range wants {
				if !strings.Contains(compact.String(), want) {
					t.Errorf("Marshaled %s, want it to contain %s", compact, want)
				}
			}
		})
	}
}

// TestStrictUnmarshalDefaults records that reading is left strict. The option
// sets are for marshaling; nothing here relaxes what a hand-edited file may
// contain, and PSW-99 owns the reporting side of that strictness.
func TestStrictUnmarshalDefaults(t *testing.T) {
	if err := jsonv2.Unmarshal([]byte(`{"a":1,"a":2}`), new(map[string]int)); err == nil {
		t.Error("Expected a duplicate object member to be rejected")
	}
	if err := jsonv2.Unmarshal([]byte("{\"a\":\"\xff\"}"), new(map[string]string)); err == nil {
		t.Error("Expected invalid UTF-8 to be rejected")
	}
}
