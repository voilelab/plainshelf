package server

import (
	"path/filepath"
	"testing"

	"github.com/voilelab/plainshelf/internal/logutil"
	"github.com/voilelab/plainshelf/server/store"
	"github.com/voilelab/plainshelf/shelf"
)

// NewApp opens the Badger store partway through startup, and Badger holds a
// lock on that directory until the handle is closed. A startup that fails after
// that point must still release it, or the failure becomes permanent: every
// later attempt to open the same store is refused by the lock left behind.
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

	// The real assertion: the store must be openable again.
	reopened, err := store.New(storePath)
	if err != nil {
		t.Fatalf("store still locked after failed startup: %v", err)
	}
	if err := reopened.Close(); err != nil {
		t.Fatalf("close reopened store: %v", err)
	}
}
