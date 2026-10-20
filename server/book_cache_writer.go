package server

import (
	"crypto/rand"
	"encoding/hex"

	"github.com/voilelab/plainshelf/internal/util"
	"github.com/voilelab/plainshelf/server/store"
)

// settingKeyBookCacheWriterID stores the identity this installation stamps onto
// the book cache it exports into every shelf's app/ folder.
const settingKeyBookCacheWriterID = "book_cache_writer_id"

// bookCacheWriterIDBytes is the length of a generated ID before hex encoding.
// Eight bytes is far more than enough to keep two machines sharing a shelf from
// colliding, and short enough to keep the file name readable.
const bookCacheWriterIDBytes = 8

// resolveBookCacheWriterID returns this installation's book cache writer ID,
// generating and persisting one on first use. It is random rather than
// hardware-derived: a MAC-based ID survives a reinstall but is unstable with
// several interfaces, virtual adapters or containers, and would put a hardware
// identifier into files users sync to the cloud. A stale entry after a reinstall
// is cheaper, and the export prunes entries no longer written. The store lives
// outside the shelf (store_path, or the desktop data directory), so the ID names
// the installation and every shelf it opens shares it.
func resolveBookCacheWriterID(storeDB *store.DB) (string, error) {
	value, found, err := storeDB.GetSetting(settingKeyBookCacheWriterID)
	if err != nil {
		return "", util.Errorf("%w", err)
	}
	if found && len(value) > 0 {
		return string(value), nil
	}

	buf := make([]byte, bookCacheWriterIDBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", util.Errorf("%w", err)
	}
	writerID := hex.EncodeToString(buf)

	if err := storeDB.SetSetting(settingKeyBookCacheWriterID, []byte(writerID)); err != nil {
		return "", util.Errorf("%w", err)
	}
	return writerID, nil
}
