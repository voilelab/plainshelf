package shelf

import (
	"github.com/voilelab/plainshelf/internal/appcache"
	"github.com/voilelab/plainshelf/internal/fsutil"
	"github.com/voilelab/plainshelf/internal/logutil"
	"github.com/voilelab/plainshelf/shelf/scancache"
)

// The directory scan cache lives in shelf/scancache so its mtime trust chain
// can be tested without a real shelf. These aliases keep this package's call
// sites unchanged by the split.

// scanStats is what one walk cost; iterateShelfTree returns it.
type scanStats = scancache.Stats

// scanCacheFileName is the snapshot under app/, aliased for the integration
// tests that plant or inspect the file.
const scanCacheFileName = scancache.FileName

// newScanCache builds the shelf's scan cache over its app/ directory. Open loads
// the previous run's snapshot, so the first walk is already cheap.
func newScanCache(dbRoot fsutil.ReadFS, enabled bool, logger logutil.Logger) *scancache.Cache {
	return scancache.Open(scancache.Config{
		Store:   appcache.NewFSStore(dbRoot, appFolder),
		Enabled: enabled,
		Logger:  logger,
	})
}
