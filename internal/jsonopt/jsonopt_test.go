package jsonopt

import (
	jsonv1 "encoding/json"
	"encoding/json/jsontext"
	jsonv2 "encoding/json/v2"
	"strings"
	"testing"
)

// repeats is how many times a value is marshaled before a set is called
// deterministic. Go randomizes map iteration, so a payload that comes back
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

// nested is shaped like the payloads PlainShelf writes: maps reached through a
// struct field and through another map, not only at the top level. Testing the
// option here rather than once per on-disk type is the whole point of having
// one place to set it — Deterministic sorts every map in the value, so a
// per-payload repeat of this assertion would re-test the standard library.
type nested struct {
	Name    string                     `json:"name"`
	Entries map[string]map[string]int  `json:"entries"`
	ByKey   map[string]struct{ N int } `json:"by_key"`
}

func nestedValue() nested {
	value := nested{Name: "shelf", Entries: map[string]map[string]int{}, ByKey: map[string]struct{ N int }{}}
	for key, n := range wideMap() {
		// A narrow inner map on purpose: twelve of them randomize together
		// just as reliably as twelve wide ones, and keep a failure readable.
		value.Entries[key] = map[string]int{"a": n, "b": n + 1, "c": n + 2, "d": n + 3}
		value.ByKey[key] = struct{ N int }{N: n}
	}
	return value
}

// TestOptionSetsSortMapKeys is the assertion the whole package exists for: each
// exported set marshals the same value to the same bytes, at every depth.
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
			want, err := jsonv2.Marshal(nestedValue(), tc.opts)
			if err != nil {
				t.Fatalf("Failed to marshal: %v", err)
			}
			for i := range repeats {
				got, err := jsonv2.Marshal(nestedValue(), tc.opts)
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

// TestOptionsWithoutDeterministicVaryTheOrder is the control for the assertion
// above: it marshals the same payload without the option and fails if the bytes
// never move, which would mean TestOptionSetsSortMapKeys passes for a reason
// other than the one it claims. If this starts failing because a later Go
// release sorts maps by default, the option still has to stay — the shelf
// outlives the toolchain that wrote it.
func TestOptionsWithoutDeterministicVaryTheOrder(t *testing.T) {
	first, err := jsonv2.Marshal(nestedValue())
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}
	for range repeats {
		got, err := jsonv2.Marshal(nestedValue())
		if err != nil {
			t.Fatalf("Failed to marshal: %v", err)
		}
		if string(got) != string(first) {
			return
		}
	}
	t.Fatalf("Marshaling the payload %d times without Deterministic never changed the order; TestOptionSetsSortMapKeys proves nothing", repeats)
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

// TestStrictUnmarshalDefaults records what a decoder does with no options: v2's
// strict defaults, which [Read] exists to hold back until PSW-99 adopts them.
func TestStrictUnmarshalDefaults(t *testing.T) {
	if err := jsonv2.Unmarshal([]byte(`{"a":1,"a":2}`), new(map[string]int)); err == nil {
		t.Error("Expected a duplicate object member to be rejected")
	}
	if err := jsonv2.Unmarshal([]byte("{\"a\":\"\xff\"}"), new(map[string]string)); err == nil {
		t.Error("Expected invalid UTF-8 to be rejected")
	}
}

// readCompat is shaped like the shelf's own on-disk types: a snake_case member
// that a hand edit could plausibly write with a dash, and one whose only
// difference from the tag is capitalization.
type readCompat struct {
	SchemaVersion int    `json:"schema_version"`
	Title         string `json:"title"`
}

// TestReadMatchesV1 is the assertion [Read] exists for, and it is written
// against the v1 package rather than against a table of remembered behaviors on
// purpose: the first version of this option set reached for
// MatchCaseInsensitiveNames alone and was *looser* than v1, because v2 folds
// away underscores and dashes as well as case. A stray "schema-version" that v1
// ignored would have bound to SchemaVersion, and a book.json whose
// schema_version reads as a future one refuses every write. Only v1 itself can
// say what v1 did.
func TestReadMatchesV1(t *testing.T) {
	for _, input := range []string{
		// Case alone: v1 matched these, and so must Read — a hand-edited
		// "Title" that stops being read is deleted by the next whole-file write.
		`{"title":"kept"}`,
		`{"Title":"kept"}`,
		`{"TITLE":"kept"}`,
		`{"SCHEMA_VERSION":3}`,
		`{"Schema_Version":3}`,
		// Delimiters: v1 ignored all of these, and so must Read.
		`{"schema-version":999}`,
		`{"schemaversion":999}`,
		`{"schema__version":999}`,
		// Tolerances v1 had that v2 dropped.
		`{"title":"first","title":"second"}`,
		"{\"title\":\"\xff\"}",
	} {
		t.Run(input, func(t *testing.T) {
			var v1, v2 readCompat
			v1Err := jsonv1.Unmarshal([]byte(input), &v1)
			v2Err := jsonv2.Unmarshal([]byte(input), &v2, Read())

			if (v1Err == nil) != (v2Err == nil) {
				t.Fatalf("v1 error %v, Read() error %v; the two must agree on acceptance", v1Err, v2Err)
			}
			if v1 != v2 {
				t.Errorf("v1 decoded %+v, Read() decoded %+v", v1, v2)
			}
		})
	}
}
