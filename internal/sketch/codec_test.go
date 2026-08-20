package sketch

import (
	"encoding/base64"
	"slices"
	"strings"
	"testing"
)

func TestEncodeDecodeRoundTrip(t *testing.T) {
	rng := newRNG(1234)

	testCases := map[string]Sketch{
		"empty":     BuildDefault(""),
		"short":     BuildDefault(strings.Join(randomWords(rng, 40), " ")),
		"truncated": BuildDefault(strings.Join(randomWords(rng, 4000), " ")),
		"cjk":       BuildDefault(strings.Repeat("書架上的每一本書都是純文字檔案，", 500)),
		"custom n":  Build(strings.Join(randomWords(rng, 4000), " "), 8, 37),
	}

	for name, want := range testCases {
		t.Run(name, func(t *testing.T) {
			got, err := Decode(want.Encode())
			if err != nil {
				t.Fatalf("Decode returned an error: %v", err)
			}

			if got.N != want.N || got.Distinct != want.Distinct || !slices.Equal(got.Values, want.Values) {
				t.Fatalf("round trip changed the sketch:\n got N=%d distinct=%d values=%d\nwant N=%d distinct=%d values=%d",
					got.N, got.Distinct, len(got.Values), want.N, want.Distinct, len(want.Values))
			}

			if similarity := Jaccard(got, want); similarity != 1 && want.Distinct > 0 {
				t.Errorf("a sketch does not match its own round trip: %v", similarity)
			}
		})
	}
}

// TestEncodeIsStable pins the encoded form. The hash and the layout are part of
// the stored fingerprint, so changing either invalidates every sketch already
// written to disk; this test is here to make that a deliberate decision.
func TestEncodeIsStable(t *testing.T) {
	const want = "AQUAAAAbAAAAAAAAAAUAAACKCECE7YyXB7ULKZ8cf0kMMviiXlG5Aw/wioinKWDSHkMHjDt7dzwi"

	got := Build("the shelf is the source of truth", DefaultN, 5).Encode()
	if got != want {
		t.Errorf("encoded form changed:\n got %s\nwant %s", got, want)
	}
}

func TestDecodeRejectsMalformedInput(t *testing.T) {
	valid := Build("the shelf is the source of truth", DefaultN, 5)
	raw, err := base64.StdEncoding.DecodeString(valid.Encode())
	if err != nil {
		t.Fatalf("the fixture is not valid base64: %v", err)
	}

	corrupt := func(mutate func(raw []byte) []byte) string {
		return base64.StdEncoding.EncodeToString(mutate(slices.Clone(raw)))
	}

	testCases := map[string]string{
		"not base64": "not base64 at all!",
		"empty":      "",
		"header only": corrupt(func(raw []byte) []byte {
			return raw[:headerLen-1]
		}),
		"unknown version": corrupt(func(raw []byte) []byte {
			raw[0] = codecVersion + 1

			return raw
		}),
		"zero shingle width": corrupt(func(raw []byte) []byte {
			raw[1], raw[2], raw[3], raw[4] = 0, 0, 0, 0

			return raw
		}),
		"more hashes than distinct shingles": corrupt(func(raw []byte) []byte {
			raw[5] = 1

			return raw
		}),
		"truncated values": corrupt(func(raw []byte) []byte {
			return raw[:len(raw)-3]
		}),
		"unsorted values": corrupt(func(raw []byte) []byte {
			for i := range 8 {
				raw[headerLen+i], raw[headerLen+8+i] = raw[headerLen+8+i], raw[headerLen+i]
			}

			return raw
		}),
	}

	for name, encoded := range testCases {
		t.Run(name, func(t *testing.T) {
			if _, err := Decode(encoded); err == nil {
				t.Error("Decode accepted a malformed sketch")
			}
		})
	}
}
