package store

import (
	"encoding/json"

	badger "github.com/dgraph-io/badger/v4"
	"github.com/voilelab/plainshelf/internal/util"
)

var readHistoryKey = []byte("read_history")

func (db *DB) GetReadHistory() ([]string, error) {
	var history []string
	err := db.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(readHistoryKey)
		if err != nil {
			return err
		}
		return item.Value(func(val []byte) error {
			return json.Unmarshal(val, &history)
		})
	})
	if err == badger.ErrKeyNotFound {
		return []string{}, nil
	}
	if err != nil {
		return nil, util.Errorf("%w", err)
	}
	return history, nil
}

func (db *DB) SetReadHistory(history []string) error {
	if len(history) > db.readHistoryLimit {
		history = history[:db.readHistoryLimit]
	}

	bs, err := json.Marshal(history)
	if err != nil {
		return util.Errorf("%w", err)
	}

	err = db.db.Update(func(txn *badger.Txn) error {
		return txn.Set(readHistoryKey, bs)
	})
	if err != nil {
		return util.Errorf("%w", err)
	}
	return nil
}

func (db *DB) AddToReadHistory(bookID string) error {
	history, err := db.GetReadHistory()
	if err != nil {
		return util.Errorf("%w", err)
	}

	// Remove if already exists
	newHistory := []string{bookID}
	for _, id := range history {
		if id != bookID {
			newHistory = append(newHistory, id)
		}
	}

	// Trim to limit
	if len(newHistory) > db.readHistoryLimit {
		newHistory = newHistory[:db.readHistoryLimit]
	}

	return db.SetReadHistory(newHistory)
}
