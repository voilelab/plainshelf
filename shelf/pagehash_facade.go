package shelf

import (
	"github.com/voilelab/plainshelf/internal/appcache"
	"github.com/voilelab/plainshelf/shelf/pagehash"
)

type (
	PageHashCache      = pagehash.Cache
	PageHashCacheStats = pagehash.Stats
)

func (s *Shelf) OpenPageHashCache() *PageHashCache {
	return pagehash.Open(pagehash.Config{
		Store:     appcache.NewFSStore(s.dbRoot, appFolder),
		Logger:    s.Logger,
		LiveBooks: s.liveBookIDs,
	})
}
