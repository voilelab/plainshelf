// Package fingerprint stores the similarity fingerprint of a source's content
// under app/, so computing it - a full read plus a pass over every character -
// happens once per distinct content rather than once per run.
//
// It knows how to store an answer, when to trust it, and how to merge two
// machines' files; it does not know how a fingerprint is computed or how a shelf
// is scanned. Everything from the outside arrives through Config, which is why
// it holds no *Shelf.
package fingerprint

import (
	"github.com/voilelab/plainshelf/internal/util"
)

/*
The file has two levels of key, and neither works alone.

  index    {bookID}/{sourceID} -> the size, mtime and MD5 of that source.txt

    Cheap: one Stat decides whether the record still describes the file, so an
    unchanged source is answered without being opened. A cache that has to read
    the file to learn whether it may skip reading the file saves nothing, which
    is the deadlock this level breaks.

    Keyed on the book ID rather than the path, because a book ID survives a
    retitle and a move between layers. Moving a bookpkg with a file manager
    must not throw its fingerprints away; a path key would.

  entries  MD5 of source.txt -> the fingerprint of that exact content

    Immutable, so an entry is never updated, only added. Two copies of one book
    share an entry, and merging two machines' caches is a set union that cannot
    conflict. Keyed only this way, though, a book copied in or restored from a
    backup would be fingerprinted from scratch.

It lives under app/ rather than in book.json or meta.json for three reasons: a
fingerprint belongs to a source's content, not to whichever source the book
currently points at; an entry is ~1.4 kB of base64, in files meant to stay
readable in a text editor; and changing the normalization or sketch parameters
invalidates every fingerprint at once - one file to delete here, a full-library
rewrite there, which docs/concepts/data-format-versioning.md refuses to do.

So a missing, unreadable, too-new or differently-parameterized file is a cache
miss and nothing more.
*/

// Like the other files under app/ there is no migration path and none is
// needed: anything this build cannot read is discarded and recomputed.
const schemaVersion = 1

// Not named per installation the way the exported book cache is: a fingerprint
// is a pure function of the content, so every machine computes the same answer
// and they can all share one file. See mergeIndex for how they combine.
const cacheFileName = "fingerprint-cache.json"

// ErrIncompleteAlgo requires every field: a cache that cannot describe its own
// algorithm cannot tell a stale entry from a usable one.
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

// Builder computes the fingerprint of one source. The cache calls it only when
// neither level of key could answer, and hands it the content it has already
// read so the builder does not read the file a second time.
type Builder func(content []byte) (Entry, error)

// Stats records what one run of a fingerprint task cost.
type Stats struct {
	// StatHits is how many sources were answered from the index, without being
	// opened at all, and ContentHits how many had to be read but were then
	// recognized by content - a copied, restored or re-imported book.
	StatHits    int
	ContentHits int

	// Built is how many fingerprints had to be computed, and Repaired how many
	// sources had an md5_hash in meta.json that disagreed with their content.
	Built    int
	Repaired int
}

// Coverage is how much of a set of books the cache can already answer for, as
// the maintenance UI reports it before offering to build the rest.
type Coverage struct {
	// Fingerprinted counts the books with a fingerprint on record for their
	// current source; Missing is the rest, which the next build has to read.
	Total         int `json:"total"`
	Fingerprinted int `json:"fingerprinted"`
	Missing       int `json:"missing"`

	// Algo is the ruleset the count was taken under, so a caller can tell "no
	// book is fingerprinted" from "the cache was built with other rules and
	// discarded": both leave Fingerprinted at zero, but only the algo says why.
	Algo Algo `json:"algo"`
}
