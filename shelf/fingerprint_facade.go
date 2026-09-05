package shelf

import (
	"github.com/voilelab/plainshelf/internal/appcache"
	"github.com/voilelab/plainshelf/internal/util"
	"github.com/voilelab/plainshelf/shelf/fingerprint"
)

// The similarity fingerprint cache lives in shelf/fingerprint so it can evolve
// on its own - a new algorithm, a forced rebuild, a future read from the pcloud
// client - without being tied to the shelf's lifecycle or its scan-ready state.
// It holds no *Shelf: everything it needs arrives through fingerprint.Config.
// These aliases and facade methods keep the symbols call sites name reachable as
// shelf.X.

// Fingerprint cache types.
type (
	FingerprintCache      = fingerprint.Cache
	FingerprintAlgo       = fingerprint.Algo
	FingerprintEntry      = fingerprint.Entry
	FingerprintCacheStats = fingerprint.Stats
	FingerprintCoverage   = fingerprint.Coverage

	// FingerprintBookSource names one book and the source of it a coverage
	// count asks about. The caller assembles the list, which is what keeps this
	// shelf out of the question of which books a given request may count.
	FingerprintBookSource = fingerprint.BookSource
)

// ErrIncompleteFingerprintAlgo is raised by OpenFingerprintCache for an
// algorithm that does not name all of its rules.
var ErrIncompleteFingerprintAlgo = fingerprint.ErrIncompleteAlgo

// OpenFingerprintCache loads the fingerprint cache for algo, wiring it to this
// shelf: its file under app/, the shelf's live book set for pruning, and the
// shelf-owned repair of a source's stale md5_hash. It reports
// ErrIncompleteFingerprintAlgo for an algorithm that cannot describe itself, and
// otherwise starts from empty whenever the file on disk cannot be used - a miss
// the run about to happen rebuilds, never an error.
func (s *Shelf) OpenFingerprintCache(algo FingerprintAlgo) (*FingerprintCache, error) {
	cache, err := fingerprint.Open(fingerprint.Config{
		Store:      appcache.NewFSStore(s.dbRoot, appFolder),
		Algo:       algo,
		Logger:     s.Logger,
		LiveBooks:  s.liveBookIDs,
		RepairHash: repairSourceContentHash,
	})
	if err != nil {
		return nil, util.Errorf("%w", err)
	}
	return cache, nil
}

// FingerprintCoverageFor reports how many of these books already have a
// fingerprint for the named source under algo, reading only
// app/fingerprint-cache.json and what is already in memory - never a
// source.txt. A cache built under different rules answers for none, which is
// what tells the UI a rebuild is due.
//
// The books are the caller's to choose rather than the whole shelf's listing,
// because a count is a listing in miniature: a total taken over books the
// requester cannot see would report that they exist. Which books a request may
// count is a server-side question the shelf has no business answering.
func (s *Shelf) FingerprintCoverageFor(algo FingerprintAlgo, books []FingerprintBookSource) (FingerprintCoverage, error) {
	cache, err := s.OpenFingerprintCache(algo)
	if err != nil {
		return FingerprintCoverage{}, util.Errorf("%w", err)
	}

	return cache.CoverageFor(books), nil
}

// repairSourceContentHash is the write the fingerprint cache delegates back to
// the shelf: when a source's content hashes to something its meta.json does not
// record, the stored hash is stale and repaired here. Keeping the write on this
// side is what lets fingerprint.Cache stay pure computation over its own file -
// it computes and caches, the shelf owns the source data. See
// Source.RepairContentHash.
func repairSourceContentHash(source *Source, contentMD5 string) (bool, error) {
	return source.RepairContentHash(contentMD5)
}

// liveBookIDs reports the books the shelf currently holds, for the cache to
// prune records of books that are gone, and false when the shelf has not
// finished its first scan and cannot answer - in which case nothing is pruned.
func (s *Shelf) liveBookIDs() (map[string]struct{}, bool) {
	if !s.IsReady() {
		return nil, false
	}

	s.bookCache.RLock()
	defer s.bookCache.RUnlock()

	ids := make(map[string]struct{}, len(s.bookCache.cache))
	for bookID := range s.bookCache.cache {
		ids[bookID] = struct{}{}
	}
	return ids, true
}
