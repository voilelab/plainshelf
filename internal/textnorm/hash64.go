package textnorm

// Hash64Version names the hash a fingerprint's shingles are reduced with. Like
// NormalizeVersion it belongs in the algo block of any cache holding these
// values, and it must change whenever the algorithm does.
//
// The hash itself is computed where the shingling happens, in internal/sketch:
// this package names the algorithm because a version string belongs beside the
// other one a fingerprint is keyed on, not because it runs it. The name stands
// for xxHash64 under its fixed default seed.
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
const Hash64Version = "xxhash64-v1"
