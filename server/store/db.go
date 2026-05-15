package store

import (
	badger "github.com/dgraph-io/badger/v4"
	"github.com/voilelab/plainshelf/internal/util"
)

type DB struct {
	readHistoryLimit int

	db *badger.DB
}

func New(dbPath string, readHistoryLimit int) (*DB, error) {
	opts := badger.DefaultOptions(dbPath).WithLogger(nil)
	db, err := badger.Open(opts)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}
	return &DB{db: db, readHistoryLimit: readHistoryLimit}, nil
}

func (db *DB) Close() error {
	err := db.db.Close()
	if err != nil {
		return util.Errorf("%w", err)
	}
	return nil
}
