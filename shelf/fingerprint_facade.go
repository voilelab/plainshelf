package shelf

import (
	"github.com/voilelab/plainshelf/internal/appcache"
	"github.com/voilelab/plainshelf/internal/util"
	"github.com/voilelab/plainshelf/shelf/fingerprint"
)

type (
	FingerprintCache      = fingerprint.Cache
	FingerprintAlgo       = fingerprint.Algo
	FingerprintEntry      = fingerprint.Entry
	FingerprintCacheStats = fingerprint.Stats
	FingerprintCoverage   = fingerprint.Coverage

	FingerprintBookSource = fingerprint.BookSource
)

var ErrIncompleteFingerprintAlgo = fingerprint.ErrIncompleteAlgo

func (s *Shelf) OpenFingerprintCache(algo FingerprintAlgo) (*FingerprintCache, error) {
	cache, err := fingerprint.Open(fingerprint.Config{
		Store:      appcache.NewFSStore(s.dbRoot, appFolder),
		Algo:       algo,
		Logger:     s.Logger,
		LiveBooks:  s.liveBookIDs,
		RepairHash: (*Source).RepairContentHash,
	})
	if err != nil {
		return nil, util.Errorf("%w", err)
	}
	return cache, nil
}

func (s *Shelf) FingerprintCoverageFor(algo FingerprintAlgo, books []FingerprintBookSource) (FingerprintCoverage, error) {
	cache, err := s.OpenFingerprintCache(algo)
	if err != nil {
		return FingerprintCoverage{}, util.Errorf("%w", err)
	}

	return cache.CoverageFor(books), nil
}

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
