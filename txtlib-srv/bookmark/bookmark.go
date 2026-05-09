package bookmark

import (
	"encoding/json"

	badger "github.com/dgraph-io/badger/v4"
	"github.com/voilelab/plainshelf/internal/util"
)

type Mark struct {
	CharOffset int `json:"char_offset"`
}

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

func (db *DB) Close() error {
	err := db.db.Close()
	if err != nil {
		return util.Errorf("%w", err)
	}
	return nil
}

func (db *DB) Set(bookID string, mark Mark) error {
	bs, err := json.Marshal(mark)
	if err != nil {
		return util.Errorf("%w", err)
	}

	err = db.db.Update(func(txn *badger.Txn) error {
		return txn.Set([]byte(bookID), bs)
	})
	if err != nil {
		return util.Errorf("%w", err)
	}
	return nil
}

func (db *DB) Get(bookID string) (Mark, error) {
	var mark Mark
	err := db.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(bookID))
		if err != nil {
			return err
		}
		return item.Value(func(val []byte) error {
			return json.Unmarshal(val, &mark)
		})
	})
	if err == badger.ErrKeyNotFound {
		return Mark{}, nil
	}

	if err != nil {
		return Mark{}, util.Errorf("%w", err)
	}
	return mark, nil
}
