package server

import (
	"path/filepath"
	"testing"

	"github.com/voilelab/plainshelf/internal/logutil"
	"github.com/voilelab/plainshelf/server/store"
	"github.com/voilelab/plainshelf/shelf"
)

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
