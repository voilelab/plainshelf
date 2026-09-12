package server

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/voilelab/plainshelf/shelf"
)

/*
App-wide read_only is not only an HTTP gate. Refusing the requests that ask for
a write is not the same as not writing: a shelf writes on its own account too —
it creates its folders, clears app/tmp/, takes the lock file and exports the
book cache on a timer — and none of that has a request behind it for the gate to
see. What turns those writes off is the setting reaching ShelfConf, which is
what the two tests below pin.

Neither issues a request, so neither belongs in server/contract. The HTTP side —
that a read-only server still serves a whole round of reads and leaves the shelf
untouched — is pinned in server/contract/crosscut/read_only_test.go.
*/

// newReadOnlyTestApp builds a started app whose app-wide read_only is on. It
// takes the conf the long way rather than through newTestApp because read_only
// has to be set before the shelves are opened, which is the whole point.
func newReadOnlyTestApp(t *testing.T) *App {
	t.Helper()

	app, err := NewApp(&AppConf{
		Shelves: []*shelf.ShelfConfWithID{
			{
				ID:        "default_shelf",
				ShelfConf: shelf.ShelfConf{LibRoot: t.TempDir()},
			},
		},
		StorePath: t.TempDir(),
		ReadOnly:  true,
	})
	if err != nil {
		t.Fatalf("NewApp: %v", err)
	}
	t.Cleanup(func() {
		if err := app.Close(); err != nil {
			t.Fatalf("Close app: %v", err)
		}
	})

	if err := app.Start(); err != nil {
		t.Fatalf("Start app: %v", err)
	}
	waitForShelves(t, app)

	return app
}

// A read-only server opens every shelf read-only, including one added after
// startup through the desktop "add shelf" flow.
func TestReadOnlyServerOpensEveryShelfReadOnly(t *testing.T) {
	app := newReadOnlyTestApp(t)

	shelfData, ok := app.ShelfManager().GetShelf("default_shelf")
	if !ok {
		t.Fatal("default_shelf missing")
	}
	if !shelfData.ReadOnly() {
		t.Error("configured shelf ReadOnly() = false, want the app-wide read_only to reach it")
	}

	addedRoot := t.TempDir()
	if err := app.AddShelf(shelf.ShelfConfWithID{
		ID:        "added_later",
		Name:      "Added Later",
		ShelfConf: shelf.ShelfConf{LibRoot: addedRoot},
	}); err != nil {
		t.Fatalf("AddShelf: %v", err)
	}

	added, ok := app.ShelfManager().GetShelf("added_later")
	if !ok {
		t.Fatal("added_later missing from the shelf manager")
	}
	if !added.ReadOnly() {
		t.Error("added shelf ReadOnly() = false, want the app-wide read_only to reach it too")
	}

	// The writer ID is what enables the exported book cache, and exporting is a
	// write. A read-only server must not hand one out, so the export is refused
	// as read-only rather than reported as unconfigured.
	if _, err := added.ExportBookCache(); err == nil {
		t.Error("ExportBookCache on a read-only server succeeded, want a refusal")
	}
}

// A read-only server must not create the shelf either — the same promise
// ShelfConf.ReadOnly makes, reached through the app-wide setting.
func TestReadOnlyServerDoesNotCreateTheShelf(t *testing.T) {
	libRoot := filepath.Join(t.TempDir(), "missing-shelf")

	app, err := NewApp(&AppConf{
		Shelves: []*shelf.ShelfConfWithID{
			{
				ID:        "default_shelf",
				ShelfConf: shelf.ShelfConf{LibRoot: libRoot},
			},
		},
		StorePath: t.TempDir(),
		ReadOnly:  true,
	})
	if err == nil {
		app.Close()
		t.Fatal("NewApp succeeded on a missing lib_root, want a failure rather than a created shelf")
	}
	if _, statErr := os.Stat(libRoot); !os.IsNotExist(statErr) {
		t.Errorf("stat %s = %v, want the path still missing", libRoot, statErr)
	}
}
