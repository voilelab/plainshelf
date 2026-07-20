package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	badger "github.com/dgraph-io/badger/v4"
	"github.com/voilelab/plainshelf/internal/util"
)

const readHistoryKey = "read_history"
const readHistoryUpdateAttempts = 16

func readHistoryStoreKey(shelfID string) []byte {
	return fmt.Appendf(nil, "%s:%s", readHistoryKey, shelfID)
}

func getReadHistory(txn *badger.Txn, shelfID string) ([]string, error) {
	item, err := txn.Get(readHistoryStoreKey(shelfID))
	if errors.Is(err, badger.ErrKeyNotFound) {
		return []string{}, nil
	}
	if err != nil {
		return nil, err
	}

	var history []string
	if err := item.Value(func(val []byte) error {
		return json.Unmarshal(val, &history)
	}); err != nil {
		return nil, err
	}
	return history, nil
}

func trimReadHistory(history []string, limit int) []string {
	limit = max(limit, 0)
	if len(history) > limit {
		return history[:limit]
	}
	return history
}

func setReadHistory(txn *badger.Txn, shelfID string, history []string, limit int) error {
	bs, err := json.Marshal(trimReadHistory(history, limit))
	if err != nil {
		return err
	}
	return txn.Set(readHistoryStoreKey(shelfID), bs)
}

func (db *DB) GetReadHistory(shelfID string) ([]string, error) {
	var history []string
	err := db.db.View(func(txn *badger.Txn) error {
		var err error
		history, err = getReadHistory(txn, shelfID)
		return err
	})
	if err != nil {
		return nil, util.Errorf("%w", err)
	}
	return history, nil
}

func (db *DB) SetReadHistory(shelfID string, history []string, readHistoryLimit int) error {
	err := db.db.Update(func(txn *badger.Txn) error {
		return setReadHistory(txn, shelfID, history, readHistoryLimit)
	})
	if err != nil {
		return util.Errorf("%w", err)
	}
	return nil
}

func (db *DB) AddToReadHistory(shelfID string, bookID string, readHistoryLimit int) error {
	for attempt := range readHistoryUpdateAttempts {
		err := db.db.Update(func(txn *badger.Txn) error {
			history, err := getReadHistory(txn, shelfID)
			if err != nil {
				return err
			}

			newHistory := make([]string, 1, len(history)+1)
			newHistory[0] = bookID
			for _, id := range history {
				if id != bookID {
					newHistory = append(newHistory, id)
				}
			}

			return setReadHistory(txn, shelfID, newHistory, readHistoryLimit)
		})
		if !errors.Is(err, badger.ErrConflict) {
			if err != nil {
				return util.Errorf("%w", err)
			}
			return nil
		}
		if attempt+1 < readHistoryUpdateAttempts {
			time.Sleep(time.Duration(attempt+1) * time.Millisecond)
		}
	}

	return util.Errorf("%w", badger.ErrConflict)
}
