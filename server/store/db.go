package store

import (
	badger "github.com/dgraph-io/badger/v4"
	"github.com/voilelab/plainshelf/internal/util"
)

type DB struct {
	db *badger.DB
}

func New(dbPath string) (*DB, error) {
	opts := badger.DefaultOptions(dbPath).WithLogger(nil)
	db, err := badger.Open(opts)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}
	return &DB{db: db}, nil
}

// NewInMemory opens a store that keeps everything in memory and never touches
// the disk, for a server that must not write anything at all — the standalone
// reader binary, which is pointed at a shelf and given nowhere of its own to
// write to.
//
// Badger requires both directories to be empty when it runs in memory, which is
// what DefaultOptions("") gives it.
func NewInMemory() (*DB, error) {
	opts := badger.DefaultOptions("").WithInMemory(true).WithLogger(nil)
	db, err := badger.Open(opts)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}
	return &DB{db: db}, nil
}

func (db *DB) Close() error {
	err := db.db.Close()
	if err != nil {
		return util.Errorf("%w", err)
	}
	return nil
}
