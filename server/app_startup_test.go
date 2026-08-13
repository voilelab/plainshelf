package server

import (
	"path/filepath"
	"testing"

	badger "github.com/dgraph-io/badger/v4"
	"github.com/voilelab/plainshelf/internal/logutil"
	"github.com/voilelab/plainshelf/server/store"
	"github.com/voilelab/plainshelf/shelf"
)

func TestNewAppDeletesLegacyBookmarks(t *testing.T) {
	storePath := filepath.Join(t.TempDir(), "store")
	raw, err := badger.Open(badger.DefaultOptions(storePath).WithLogger(nil))
	if err != nil {
		t.Fatalf("open seed store: %v", err)
	}
	if err := raw.Update(func(txn *badger.Txn) error {
		if err := txn.Set([]byte("bookmark:default_shelf:book-1"), []byte(`{"char_offset":42}`)); err != nil {
			return err
		}
		return txn.Set([]byte("setting:keep"), []byte("true"))
	}); err != nil {
		raw.Close()
		t.Fatalf("seed store: %v", err)
	}
	if err := raw.Close(); err != nil {
		t.Fatalf("close seed store: %v", err)
	}

	app, err := NewApp(&AppConf{
		Shelves: []*shelf.ShelfConfWithID{{
			ID:        "default_shelf",
			ShelfConf: shelf.ShelfConf{LibRoot: t.TempDir()},
		}},
		StorePath: storePath,
		Security:  &SecurityConf{Mode: SecurityModeNone},
	})
	if err != nil {
		t.Fatalf("NewApp: %v", err)
	}
	if err := app.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	raw, err = badger.Open(badger.DefaultOptions(storePath).WithLogger(nil))
	if err != nil {
		t.Fatalf("reopen store: %v", err)
	}
	defer raw.Close()
	if err := raw.View(func(txn *badger.Txn) error {
		if _, err := txn.Get([]byte("bookmark:default_shelf:book-1")); err != badger.ErrKeyNotFound {
			t.Fatalf("legacy bookmark lookup error = %v, want key not found", err)
		}
		if _, err := txn.Get([]byte("setting:keep")); err != nil {
			t.Fatalf("unrelated setting was removed: %v", err)
		}
		return nil
	}); err != nil {
		t.Fatalf("verify store: %v", err)
	}
}

// Badger locks the store directory until the handle is closed, so a startup
// that fails after opening it must still release the lock.
func TestNewAppReleasesStoreWhenLaterStepFails(t *testing.T) {
	storePath := filepath.Join(t.TempDir(), "store")

	conf := &AppConf{
		Shelves: []*shelf.ShelfConfWithID{
			{
				ID:        "default_shelf",
				ShelfConf: shelf.ShelfConf{LibRoot: t.TempDir()},
			},
		},
		StorePath: storePath,
		// The worker logger is built after the store is opened, so an invalid
		// level here fails startup at exactly the point being tested.
		Worker: &WorkerConf{
			Logger: logutil.LogConf{Level: "not-a-real-level"},
		},
	}

	app, err := NewApp(conf)
	if err == nil {
		app.Close()
		t.Fatal("NewApp succeeded, want failure from the invalid worker log level")
	}

	reopened, err := store.New(storePath)
	if err != nil {
		t.Fatalf("store still locked after failed startup: %v", err)
	}
	if err := reopened.Close(); err != nil {
		t.Fatalf("close reopened store: %v", err)
	}
}
