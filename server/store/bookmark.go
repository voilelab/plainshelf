package store

import (
	"encoding/json"
	"fmt"

	badger "github.com/dgraph-io/badger/v4"
	"github.com/voilelab/plainshelf/internal/util"
)

var bookmarkKey = "bookmark:"

type Bookmark struct {
	CharOffset int `json:"char_offset"`
}

func (db *DB) SetBookmark(shelfID, bookID string, mark Bookmark) error {
	bs, err := json.Marshal(mark)
	if err != nil {
		return util.Errorf("%w", err)
	}

	err = db.db.Update(func(txn *badger.Txn) error {
		key := fmt.Sprintf("%s:%s:%s", bookmarkKey, shelfID, bookID)
		return txn.Set([]byte(key), bs)
	})
	if err != nil {
		return util.Errorf("%w", err)
	}
	return nil
}

func (db *DB) GetBookmark(shelfID, bookID string) (Bookmark, error) {
	var mark Bookmark
	err := db.db.View(func(txn *badger.Txn) error {
		key := fmt.Sprintf("%s:%s:%s", bookmarkKey, shelfID, bookID)
		item, err := txn.Get([]byte(key))
		if err != nil {
			return err
		}
		return item.Value(func(val []byte) error {
			return json.Unmarshal(val, &mark)
		})
	})
	if err == badger.ErrKeyNotFound {
		return Bookmark{}, nil
	}

	if err != nil {
		return Bookmark{}, util.Errorf("%w", err)
	}
	return mark, nil
}
