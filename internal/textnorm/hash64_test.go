package textnorm

import (
	"testing"

	"github.com/cespare/xxhash/v2"
)

func hash64(b []byte) uint64 { return xxhash.Sum64(b) }

func hash64String(s string) uint64 { return xxhash.Sum64String(s) }

var goldenHashes = []struct {
	name  string
	input string
	want  uint64
}{
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
				t.Errorf("hash64(%q) = %#016x, want %#016x; do not regenerate the golden values: a new hash needs a new Hash64Version, which invalidates every stored fingerprint", tc.input, got, tc.want)
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

func TestHash64TreatsNilAndEmptyAlike(t *testing.T) {
	if hash64(nil) != hash64([]byte{}) {
		t.Error("hash64(nil) and hash64([]byte{}) disagree")
	}
}

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
