package textnorm

// Hash64Version names the hash a fingerprint's shingles are reduced with:
// xxHash64 under its fixed default seed. Like NormalizeVersion it belongs in the
// algo block of any cache holding these values and must change with the
// algorithm. The hash runs in internal/sketch; the name lives here beside the
// other version a fingerprint is keyed on.
//
// The fixed seed is the requirement, not the algorithm. Sketches are compared
// across machines that open one shelf, so a per-process seed (hash/maphash,
// Python's hash()) gives the same book similarity zero with itself and no error.
// Use only a pure function of the input: no random seed, no map iteration order.
// The golden tests pin known inputs to outputs so a substitution fails loudly.
const Hash64Version = "xxhash64-v1"
