package store

import (
	"testing"

	badger "github.com/dgraph-io/badger/v4"
)

func TestDeleteLegacyBookmarks(t *testing.T) {
	db, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Fatalf("Close: %v", err)
		}
	})

	err = db.db.Update(func(txn *badger.Txn) error {
		if err := txn.Set([]byte("bookmark:shelf-1:book-1"), []byte(`{"char_offset":42}`)); err != nil {
			return err
		}
		return txn.Set([]byte("setting:keep"), []byte("true"))
	})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}

	if err := db.DeleteLegacyBookmarks(); err != nil {
		t.Fatalf("DeleteLegacyBookmarks: %v", err)
	}

	err = db.db.View(func(txn *badger.Txn) error {
		if _, err := txn.Get([]byte("bookmark:shelf-1:book-1")); err != badger.ErrKeyNotFound {
			t.Fatalf("legacy bookmark lookup error = %v, want key not found", err)
		}
		if _, err := txn.Get([]byte("setting:keep")); err != nil {
			t.Fatalf("unrelated setting was removed: %v", err)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
}
