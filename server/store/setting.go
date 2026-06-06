package store

import (
	"errors"
	"fmt"

	badger "github.com/dgraph-io/badger/v4"
	"github.com/voilelab/plainshelf/internal/util"
)

var settingKey = "setting"

// GetSetting retrieves a setting value by key. It returns the value, a boolean indicating if the key exists,
// and an error if any.
func (db *DB) GetSetting(key string) ([]byte, bool, error) {
	var value []byte
	err := db.db.View(func(txn *badger.Txn) error {
		settingKey := fmt.Sprintf("%s:%s", settingKey, key)
		item, err := txn.Get([]byte(settingKey))
		if err != nil {
			return util.Errorf("%w", err)
		}
		value, err = item.ValueCopy(nil)
		if err != nil {
			return util.Errorf("%w", err)
		}
		return nil
	})

	if errors.Is(err, badger.ErrKeyNotFound) {
		return nil, false, nil
	} else if err != nil {
		return nil, false, util.Errorf("%w", err)
	}
	return value, true, nil
}

func (db *DB) SetSetting(key string, value []byte) error {
	err := db.db.Update(func(txn *badger.Txn) error {
		settingKey := fmt.Sprintf("%s:%s", settingKey, key)
		err := txn.Set([]byte(settingKey), value)
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

func (db *DB) DeleteSetting(key string) error {
	err := db.db.Update(func(txn *badger.Txn) error {
		settingKey := fmt.Sprintf("%s:%s", settingKey, key)
		err := txn.Delete([]byte(settingKey))
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
