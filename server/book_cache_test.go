package server

import (
	"encoding/json/v2"
	"testing"

	"github.com/voilelab/plainshelf/internal/testutil"
	"github.com/voilelab/plainshelf/shelf"
)

// The writer ID identifies the installation, so it has to outlive a restart:
// two runs against the same store must keep writing the same file rather than
// leaving a new one behind on every start.
//
// This asks nothing of the HTTP surface — the ID is minted from the store and
// handed to the shelf at open time — so it is pinned here rather than in
// server/contract, where it would only be a slower way to call the same
// methods. The runtime half of the same promise, a shelf added after startup
// getting the ID too, does go through the API and is pinned there.
func TestBookCacheWriterIDIsStableAcrossRestarts(t *testing.T) {
	storePath := t.TempDir()
	libRoot := t.TempDir()

	newRun := func() string {
		app, err := NewApp(&AppConf{
			Shelves: []*shelf.ShelfConfWithID{
				{
					ID:        "default_shelf",
					ShelfConf: shelf.ShelfConf{LibRoot: libRoot},
				},
			},
			StorePath: storePath,
		})
		if err != nil {
			t.Fatalf("NewApp: %v", err)
		}
		defer func() {
			if err := app.Close(); err != nil {
				t.Fatalf("Close app: %v", err)
			}
		}()

		shelfData, ok := app.ShelfManager().GetShelf("default_shelf")
		if !ok {
			t.Fatal("default_shelf missing")
		}
		if err := shelfData.WaitReady(t.Context()); err != nil {
			t.Fatalf("WaitReady: %v", err)
		}
		if _, err := shelfData.ExportBookCache(); err != nil {
			t.Fatalf("ExportBookCache: %v", err)
		}

		var cache shelf.BookCacheFile
		if err := json.Unmarshal(testutil.WaitForExportedBookCache(t, libRoot), &cache); err != nil {
			t.Fatalf("decode exported book cache: %v", err)
		}
		return cache.WriterID
	}

	first := newRun()
	second := newRun()
	if first == "" || first != second {
		t.Fatalf("writer ID changed across restarts: %q then %q", first, second)
	}
	// WaitForExportedBookCache fails when a second file appears, so reaching
	// here also proves the restart did not orphan the first run's cache.
}
