package store

import (
	"encoding/json"

	badger "github.com/dgraph-io/badger/v4"
	"github.com/voilelab/plainshelf/internal/util"
	"github.com/voilelab/plainshelf/shelf"
)

var shelfKey = "shelves:"

// shelf: "shelves:{shelfID}" -> JSON-encoded shelf.ShelfConfWithID

func (db *DB) GetShelves() ([]*shelf.ShelfConfWithID, error) {
	var shelves []*shelf.ShelfConfWithID
	err := db.db.View(func(txn *badger.Txn) error {
		prefix := []byte(shelfKey)
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()

		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			value, err := item.ValueCopy(nil)
			if err != nil {
				return util.Errorf("%w", err)
			}

			var conf shelf.ShelfConfWithID
			err = json.Unmarshal(value, &conf)
			if err != nil {
				return util.Errorf("%w", err)
			}
			shelves = append(shelves, &conf)
		}
		return nil
	})
	if err != nil {
		return nil, util.Errorf("%w", err)
	}
	return shelves, nil
}

func (db *DB) AddShelf(conf *shelf.ShelfConfWithID) error {
	value, err := json.Marshal(conf)
	if err != nil {
		return util.Errorf("%w", err)
	}

	err = db.db.Update(func(txn *badger.Txn) error {
		key := []byte(shelfKey + conf.ID)
		if _, err := txn.Get(key); err == nil {
			return util.Errorf("shelf with ID %q already exists", conf.ID)
		} else if err != badger.ErrKeyNotFound {
			return util.Errorf("%w", err)
		}

		err = txn.Set(key, value)
		if err != nil {
			return util.Errorf("%w", err)
		}
		return nil
	})
	if err != nil {
		return util.Errorf("%w", err)
	}
	return nil
}

func (db *DB) DeleteShelf(id string) error {
	err := db.db.Update(func(txn *badger.Txn) error {
		key := []byte(shelfKey + id)
		if err := txn.Delete(key); err != nil {
			return util.Errorf("%w", err)
		}
		return nil
	})
	if err != nil {
		return util.Errorf("%w", err)
	}
	return nil
}
