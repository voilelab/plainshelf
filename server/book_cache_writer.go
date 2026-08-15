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
// generating and persisting one the first time it is asked.
//
// It is deliberately random rather than derived from the hardware. A MAC-based
// ID would survive a reinstall, but it is unstable in exactly the environments
// PlainShelf runs in — several interfaces, virtual adapters, containers — and it
// would write a hardware identifier into a file that users sync to cloud
// storage. A stale entry after a reinstall is the cheaper problem, and the
// export prunes ones that stop being written.
//
// The store lives outside the shelf (the server's store_path, the desktop app's
// data directory), so the ID identifies the installation rather than the shelf,
// and every shelf that installation opens shares it.
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
