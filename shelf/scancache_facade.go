package shelf

import (
	"github.com/voilelab/plainshelf/internal/appcache"
	"github.com/voilelab/plainshelf/internal/fsutil"
	"github.com/voilelab/plainshelf/internal/logutil"
	"github.com/voilelab/plainshelf/shelf/scancache"
)

type scanStats = scancache.Stats

const scanCacheFileName = scancache.FileName

func newScanCache(dbRoot fsutil.ReadFS, enabled bool, logger logutil.Logger) *scancache.Cache {
	return scancache.Open(scancache.Config{
		Store:     appcache.NewFSStore(dbRoot, appFolder),
		Enabled:   enabled,
		Logger:    logger,
		KeepFiles: []string{FolderMetaFile},
	})
}
