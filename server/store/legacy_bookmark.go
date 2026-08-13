package store

import "github.com/voilelab/plainshelf/internal/util"

// DeleteLegacyBookmarks removes reading positions written by builds that kept
// per-device progress in the server store. New builds never read or write this
// prefix; cleanup runs before the HTTP server starts accepting requests.
func (db *DB) DeleteLegacyBookmarks() error {
	if err := db.db.DropPrefix([]byte("bookmark:")); err != nil {
		return util.Errorf("%w", err)
	}
	return nil
}
