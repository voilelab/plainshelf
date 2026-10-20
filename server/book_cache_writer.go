package server

import (
	"crypto/rand"
	"encoding/hex"

	"github.com/voilelab/plainshelf/internal/util"
	"github.com/voilelab/plainshelf/server/store"
)

const settingKeyBookCacheWriterID = "book_cache_writer_id"

const bookCacheWriterIDBytes = 8

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
