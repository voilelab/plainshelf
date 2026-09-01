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

// ScanCacheMismatch reports one directory the scan cache's snapshot describes
// wrongly while its modification time claims nothing has changed - the state a
// walk cannot detect or recover from on its own. See scancache.Mismatch.
type ScanCacheMismatch = scancache.Mismatch

// scanOptions tunes one walk of the shelf tree.
type scanOptions struct {
	// checkScanCache lists every directory the scan cache would have skipped and
	// collects the ones whose snapshot no longer matches the directory, turning
	// "the new book never appears" into something the user can be shown.
	//
	// It gives up the cache's entire saving for that walk, so it belongs only on
	// a walk a user asked for and never on the refresh behind a listing. It is
	// also a no-op when scan_cache is off: there is then no snapshot to disagree
	// with, and reporting one would be inventing a fault.
	checkScanCache bool
}

// newScanCache builds the shelf's scan cache over its app/ directory. Open loads
// the previous run's snapshot, so the first walk is already cheap.
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
