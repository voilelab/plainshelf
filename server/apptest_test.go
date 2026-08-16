package server

import (
	"testing"

	"github.com/voilelab/plainshelf/shelf"
)

// newTestApp builds a started app on a fresh shelf and store. It exists for the
// tests that reach past the HTTP surface into the handlers themselves; tests
// that drive the API over HTTP live in server/contract, which has the request
// and assertion helpers for that.
func newTestApp(t *testing.T) *App {
	t.Helper()
	return newTestAppWithSecurity(t, nil)
}

func newTestAppWithSecurity(t *testing.T, security *SecurityConf) *App {
	t.Helper()

	app, err := NewApp(&AppConf{
		Shelves: []*shelf.ShelfConfWithID{
			{
				ID: "default_shelf",
				ShelfConf: shelf.ShelfConf{
					LibRoot: t.TempDir(),
				},
			},
		},
		StorePath:  t.TempDir(),
		CoverToJPG: false,
		Security:   security,
	})
	if err != nil {
		t.Fatalf("NewApp: %v", err)
	}
	t.Cleanup(func() {
		if err := app.Close(); err != nil {
			t.Fatalf("Close app: %v", err)
		}
	})

	// Start the background worker so handlers backed by task chains behave as
	// they do in production.
	if err := app.Start(); err != nil {
		t.Fatalf("Start app: %v", err)
	}

	waitForShelves(t, app)

	return app
}

// waitForShelves blocks until every configured shelf has finished its initial
// scan, so a test does not observe the shelf while it is still initializing.
// Endpoints that report "shelf is initializing" have their own tests; elsewhere
// it is noise.
func waitForShelves(t *testing.T, app *App) {
	t.Helper()

	for _, shelfData := range app.ShelfManager().GetAllShelves() {
		if err := shelfData.WaitReady(t.Context()); err != nil {
			t.Fatalf("WaitReady for shelf %s: %v", shelfData.ID, err)
		}
	}
}
