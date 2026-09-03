package textnorm

import (
	"testing"

	"github.com/cespare/xxhash/v2"
)

// hash64 and hash64String are the golden table's subject. Production hashes
// shingles in internal/sketch, which calls xxhash.Sum64String directly; these
// call the same dependency function so that the table below pins what a
// fingerprint's Hash64Version actually stands for. They live here rather than
// in the package because nothing outside a test ever needed a second spelling
// of them.
func hash64(b []byte) uint64 { return xxhash.Sum64(b) }

func hash64String(s string) uint64 { return xxhash.Sum64String(s) }

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
			if got := hash64([]byte(tc.input)); got != tc.want {
				t.Errorf("hash64(%q) = %#016x, want %#016x", tc.input, got, tc.want)
			}
		})
	}
}

func TestHash64StringMatchesHash64(t *testing.T) {
	for _, tc := range goldenHashes {
		t.Run(tc.name, func(t *testing.T) {
			if got := hash64String(tc.input); got != tc.want {
				t.Errorf("hash64String(%q) = %#016x, want %#016x", tc.input, got, tc.want)
			}
		})
	}
}

// TestHash64TreatsNilAndEmptyAlike keeps the boundary case from depending on how
// a caller happened to build its slice.
func TestHash64TreatsNilAndEmptyAlike(t *testing.T) {
	if hash64(nil) != hash64([]byte{}) {
		t.Error("hash64(nil) and hash64([]byte{}) disagree")
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
		h := hash64String(s)
		if other, ok := seen[h]; ok {
			t.Errorf("hash64String(%q) collides with %q at %#016x", s, other, h)
		}
		seen[h] = s
	}
}

func TestHash64VersionIsSet(t *testing.T) {
	if Hash64Version != "xxhash64-v1" {
		t.Errorf("Hash64Version = %q; changing it invalidates every cache built on it", Hash64Version)
	}
}
