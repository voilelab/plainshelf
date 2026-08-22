package shelf

import (
	"github.com/voilelab/plainshelf/internal/appcache"
	"github.com/voilelab/plainshelf/internal/fsutil"
	"github.com/voilelab/plainshelf/internal/logutil"
	"github.com/voilelab/plainshelf/shelf/scancache"
)

// The directory scan cache lives in shelf/scancache so its mtime trust chain -
// "a parent whose listing is proven unchanged is what lets a child's recorded
// mtime be believed" - can be tested and reasoned about without a real shelf.
// It holds no *Shelf: everything it needs arrives through scancache.Config, and
// the walk reaches the filesystem through the root the shelf hands ReadDir.
// These aliases and the facade below keep every call site in this package
// unchanged by the split.

// scanStats is what one walk of the shelf tree cost; iterateShelfTree returns it
// and scanToBookCache logs it.
type scanStats = scancache.Stats

// scanCacheFileName is the snapshot under app/, aliased so the integration tests
// that plant or inspect the file keep naming it locally.
const scanCacheFileName = scancache.FileName

// newScanCache builds the shelf's directory scan cache, wiring it to this
// shelf's file under app/. Open loads the snapshot from the previous run, so the
// first walk - the one the user waits for at startup - is already cheap.
func newScanCache(dbRoot fsutil.ReadFS, enabled bool, logger logutil.Logger) *scancache.Cache {
	return scancache.Open(scancache.Config{
		Store:   appcache.NewFSStore(dbRoot, appFolder),
		Enabled: enabled,
		Logger:  logger,
	})
}

// lastScanStats reports what the most recent walk of the shelf tree cost.
func (s *Shelf) lastScanStats() scanStats {
	return s.scanCache.LastStats()
}
