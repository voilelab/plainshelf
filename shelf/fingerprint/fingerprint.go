// Package fingerprint stores the similarity fingerprint of a source's content
// under app/, so computing it - a full read plus a pass over every character -
// happens once per distinct content rather than once per run.
//
// The package is deliberately independent of the shelf that owns it. It knows
// how to store an answer, when to trust it, and how to merge two machines'
// files; it does not know how a fingerprint is computed, how a shelf is scanned,
// or when that scan is ready. Everything it needs from the outside arrives
// through Config: a Store for its cache file under app/, the set of live book
// IDs to prune against, and a hook for the one write it must not own - repairing
// a source's stale md5_hash. That is why the cache no longer holds a *Shelf.
package fingerprint

import (
	"github.com/voilelab/plainshelf/internal/util"
)

/*
The fingerprint cache is what makes similarity detection affordable to run more
than once: computing a source's fingerprint costs a full read plus a pass over
every character of it, and the answer never changes for the same bytes.

The file has two levels of key, and each one answers a question the other
cannot.

  index    {bookID}/{sourceID} -> the size, mtime and MD5 of that source.txt

    Cheap. One Stat decides whether the record still describes the file, so a
    source that has not changed is answered without opening it at all - which
    is the whole point, because a cache that has to read the file to find out
    whether it can skip reading the file saves nothing.

    Keyed on the book ID rather than on the path because a book ID survives a
    retitle and a move between layers, while the folder name behind it does
    not. Moving a bookpkg with a file manager must not throw its fingerprints
    away; a path key would.

  entries  MD5 of source.txt -> the fingerprint of that exact content

    Immutable. The same bytes always produce the same fingerprint, so an entry
    is never updated, only added. Two copies of the same book therefore share
    one entry, and merging two machines' caches is a set union that cannot
    conflict.

Neither level works alone. Keyed only by content, a lookup would have to read
the whole file to learn the MD5 it needs to look the fingerprint up by - the
deadlock the index level exists to break. Keyed only by stat, a book copied in
or restored from a backup would be fingerprinted again from scratch.

Why this is not stored in book.json or meta.json, which are the files that
already describe a source:

  - Wrong level. A fingerprint belongs to one source's content, and one that
    lived in book.json would have to be re-derived whenever the current source
    pointer moved.
  - Wrong shape. An entry is ~1.4 kB of base64 per source. meta.json is meant
    to stay readable in a text editor.
  - Wrong lifetime. Changing the normalization or the sketch parameters
    invalidates every fingerprint at once. Under app/ that is one file to
    delete; in the books themselves it would be a full-library rewrite, which
    is exactly what docs/concepts/data-format-versioning.md refuses to do.

So this lives under app/ with the other rebuildable state: a missing,
unreadable, too-new or differently-parameterized file is a cache miss and
nothing more.
*/

// schemaVersion is the format this build writes. Like the other files under
// app/ there is no migration path and none is needed: anything this build
// cannot read is discarded and recomputed.
const schemaVersion = 1

// cacheFileName is the cache under app/. It is not named per installation the
// way the exported book cache is: a fingerprint is a pure function of the
// content, so every machine computes the same answer and they can all share one
// file. See mergeIndex for how they combine.
const cacheFileName = "fingerprint-cache.json"

// ErrIncompleteAlgo is returned when a caller opens the cache without saying
// which rules its fingerprints were produced by. Every field is required: a
// cache that cannot describe its own algorithm cannot tell a stale entry from a
// usable one.
var ErrIncompleteAlgo = util.NewError("fingerprint algorithm must name a normalization, a shingling, a hash and a k")

// Algo names the rules behind every entry in the file.
//
// It is supplied by the caller rather than defined here so that this package
// stays independent of the fingerprinting code: it knows how to store an answer
// and when to trust it, not how to compute one.
//
// Any difference at all discards the whole file. There is deliberately no
// field-level migration - a changed normalization invalidates the sketch built
// on top of it, so keeping "the parts that did not change" would mean keeping
// entries whose comparability nobody can vouch for.
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

// Entry is one source's fingerprint, as stored under its content hash. It is
// written once and never modified: the content it describes cannot change
// without changing the key it is stored under.
type Entry struct {
	// NormMD5 identifies the normalized text the sketch was built from. Two
	// sources whose raw bytes differ only in layout share it, which is what
	// makes an exact-duplicate check possible without comparing sketches.
	NormMD5 string `json:"norm_md5"`

	// NormChars is the length of the normalized text in runes, and Shingles the
	// number of distinct shingles it held. Both are kept because a similarity
	// report reads them - a 3% overlap means something different between two
	// novels than between a novel and its blurb.
	NormChars int `json:"norm_chars"`
	Shingles  int `json:"shingles"`

	// Sketch is the fingerprint itself, in the encoding the sketch package
	// produces. Opaque here: this package stores it and never interprets it.
	Sketch string `json:"sketch"`
}

// Builder computes the fingerprint of one source. The cache calls it only when
// neither level of key could answer, and hands it the content it has already
// read so the builder does not read the file a second time.
type Builder func(content []byte) (Entry, error)

// Stats records what one run of a fingerprint task cost, so the effect of the
// cache is observable rather than only inferable from wall time.
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
// the maintenance UI reports it before offering to build the rest. It is a pure
// read of the loaded cache; see Cache.CoverageFor and the shelf's
// FingerprintStatus facade.
type Coverage struct {
	// Total is how many books were counted, Fingerprinted how many have a
	// fingerprint on record for their current source, and Missing the
	// difference - the books the next build would still have to read.
	Total         int `json:"total"`
	Fingerprinted int `json:"fingerprinted"`
	Missing       int `json:"missing"`

	// Algo is the ruleset the count was taken under, so a caller can tell "no
	// book is fingerprinted" from "the cache was built with other rules and
	// discarded": both leave Fingerprinted at zero, but only the algo says why.
	Algo Algo `json:"algo"`
}
