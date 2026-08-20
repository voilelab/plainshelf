package textnorm

import "testing"

// goldenHashes pins known inputs to known outputs. This is the test that fails
// when someone swaps the algorithm underneath — for a randomly seeded one such
// as hash/maphash above all, whose damage is otherwise silent: every machine
// keeps hashing happily, and books simply stop matching across a shared shelf.
//
// Regenerating these values to make the test pass defeats it. A deliberate
// change of algorithm is a change of Hash64Version, and it invalidates every
// stored fingerprint.
var goldenHashes = []struct {
	name  string
	input string
	want  uint64
}{
	// The published xxHash64 vector for the empty input under seed 0. It is
	// here to pin the seed itself, which no other case can distinguish.
	{"empty", "", 0xef46db3751d8e999},
	{"single ascii letter", "a", 0xd24ec4f1a98c6e5b},
	{"ascii sentence", "Hello, World!", 0xc49aacf8080fe47f},
	{"cjk title", "三國演義", 0x9aaa5441590f6716},
	{"latin passage", "the quick brown fox jumps over the lazy dog", 0xed714233c5a9a792},
	{"normalized mixed script", "三國演義第一回TheRomanceoftheThreeKingdoms", 0x7e0a84103102dacb},
}

func TestHash64Golden(t *testing.T) {
	for _, tc := range goldenHashes {
		t.Run(tc.name, func(t *testing.T) {
			if got := Hash64([]byte(tc.input)); got != tc.want {
				t.Errorf("Hash64(%q) = %#016x, want %#016x", tc.input, got, tc.want)
			}
		})
	}
}

func TestHash64StringMatchesHash64(t *testing.T) {
	for _, tc := range goldenHashes {
		t.Run(tc.name, func(t *testing.T) {
			if got := Hash64String(tc.input); got != tc.want {
				t.Errorf("Hash64String(%q) = %#016x, want %#016x", tc.input, got, tc.want)
			}
		})
	}
}

// TestHash64IsStableAcrossCalls is the weakest form of the property the
// golden table really pins, kept because it names the failure directly: a
// per-process seed would not even survive repeated calls after a rebuild.
func TestHash64IsStableAcrossCalls(t *testing.T) {
	const input = "話說天下大勢，分久必合，合久必分。"

	first := Hash64String(input)
	for i := 0; i < 1000; i++ {
		if got := Hash64String(input); got != first {
			t.Fatalf("Hash64String is not a pure function: %#016x then %#016x", first, got)
		}
	}
}

// TestHash64TreatsNilAndEmptyAlike keeps the boundary case from depending on how
// a caller happened to build its slice.
func TestHash64TreatsNilAndEmptyAlike(t *testing.T) {
	if Hash64(nil) != Hash64([]byte{}) {
		t.Error("Hash64(nil) and Hash64([]byte{}) disagree")
	}
}

// TestHash64SeparatesNearbyInputs is a smoke test, not a quality measure: a
// sketch built on a hash that collides on one-character edits would be useless.
func TestHash64SeparatesNearbyInputs(t *testing.T) {
	seen := map[uint64]string{}
	for _, s := range []string{
		"三國演義第一回",
		"三國演義第二回",
		"三国演义第一回",
		"三國演義第一囘",
		"演義三國第一回",
		"三國演義第一回 ",
	} {
		h := Hash64String(s)
		if other, ok := seen[h]; ok {
			t.Errorf("Hash64String(%q) collides with %q at %#016x", s, other, h)
		}
		seen[h] = s
	}
}

func TestHash64VersionIsSet(t *testing.T) {
	if Hash64Version != "xxhash64-v1" {
		t.Errorf("Hash64Version = %q; changing it invalidates every cache built on it", Hash64Version)
	}
}
