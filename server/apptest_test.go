package server

import (
	"testing"

	"github.com/voilelab/plainshelf/shelf"
)

func newTestApp(t *testing.T) *App {
	t.Helper()
	return newTestAppWithSecurity(t, nil)
}

func newTestAppWithSecurity(t *testing.T, security *SecurityConf) *App {
	t.Helper()

	app, err := NewApp(&AppConf{
		Shelves: []*shelf.ShelfConfWithID{
			{
				ID:      "default_shelf",
				LibRoot: t.TempDir(),
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

	if err := app.Start(); err != nil {
		t.Fatalf("Start app: %v", err)
	}

	waitForInitialShelfScans(t, app)

	return app
}

func waitForInitialShelfScans(t *testing.T, app *App) {
	t.Helper()

	for _, shelfData := range app.ShelfManager().GetAllShelves() {
		if err := shelfData.WaitReady(t.Context()); err != nil {
			t.Fatalf("WaitReady for shelf %s: %v", shelfData.ID, err)
		}
	}
}
