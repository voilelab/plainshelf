package fingerprint

import (
	"github.com/voilelab/plainshelf/internal/util"
)

const schemaVersion = 1

const cacheFileName = "fingerprint-cache.json"

var ErrIncompleteAlgo = util.NewError("fingerprint algorithm must name a normalization, a shingling, a hash and a k")

// Algo names the rules behind every entry in the file. It is supplied by the
// caller so this package stays independent of the fingerprinting code.
//
// Any difference at all discards the whole file: a changed normalization
// invalidates the sketch built on top of it, so a field-level migration would
// keep entries whose comparability nobody can vouch for.
type Algo struct {
	// Normalize is the canonical-form version, e.g. textnorm.NormalizeVersion.
	Normalize string `json:"normalize"`

	// Shingle describes what was hashed, e.g. "char-5gram".
	Shingle string `json:"shingle"`

	// Hash is the shingle hash version, e.g. textnorm.Hash64Version.
	Hash string `json:"hash"`

	// K is how many hashes a sketch retains.
	K int `json:"k"`
}

func (a Algo) complete() bool {
	return a.Normalize != "" && a.Shingle != "" && a.Hash != "" && a.K > 0
}

// Entry is written once and never modified: the content it describes cannot
// change without changing the key it is stored under.
type Entry struct {
	// NormMD5 identifies the normalized text the sketch was built from. Two
	// sources whose raw bytes differ only in layout share it, which is what
	// makes an exact-duplicate check possible without comparing sketches.
	NormMD5 string `json:"norm_md5"`

	// NormChars is in runes. Both are kept because a similarity report reads
	// them: 3% overlap means something different between two novels than
	// between a novel and its blurb.
	NormChars int `json:"norm_chars"`
	Shingles  int `json:"shingles"`

	// Sketch is opaque here: stored in the sketch package's encoding, never
	// interpreted.
	Sketch string `json:"sketch"`
}

type Builder func(content []byte) (Entry, error)

type Stats struct {
	StatHits    int
	ContentHits int

	Built    int
	Repaired int
}

type Coverage struct {
	Total         int `json:"total"`
	Fingerprinted int `json:"fingerprinted"`
	Missing       int `json:"missing"`

	Algo Algo `json:"algo"`
}
