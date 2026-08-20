package textnorm

import "github.com/cespare/xxhash/v2"

// Hash64Version names the hash Hash64 computes. Like NormalizeVersion it belongs
// in the algo block of any cache holding these values, and it must change
// whenever the algorithm does.
const Hash64Version = "xxhash64-v1"

// Hash64 hashes b to 64 bits with xxHash64 under its fixed default seed.
//
// The fixed seed is the requirement, not the choice of algorithm. A PlainShelf
// shelf is regularly opened from more than one machine, and a sketch is a set of
// hash values compared across the shelves that produced them. Seed the hash per
// process — hash/maphash does exactly this, and so does Python's hash() for
// strings — and two machines describe the same book with two disjoint sets of
// numbers. The similarity comes out as zero, no error is raised anywhere, and
// the book simply stops matching itself. Anything used here must be a pure
// function of its input on every machine and every build, so do not reach for a
// randomly seeded hash, and do not derive a value from map iteration order.
//
// The golden tests in this package exist to catch a substitution: they pin known
// inputs to known outputs, which is the only thing that fails loudly when the
// algorithm underneath changes.
func Hash64(b []byte) uint64 {
	return xxhash.Sum64(b)
}

// Hash64String is Hash64 over a string. Callers shingling normalized text hash
// many overlapping substrings of one string; this spares each of them a copy.
func Hash64String(s string) uint64 {
	return xxhash.Sum64String(s)
}
