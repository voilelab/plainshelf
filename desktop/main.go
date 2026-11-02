package main

import (
	"context"
	"log"

	"github.com/voilelab/plainshelf/frontend"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
)

func main() {
	app := NewDesktopApp()

	err := wails.Run(&options.App{
		Title:  "PlainShelf",
		Width:  1600,
		Height: 1200,
		AssetServer: &assetserver.Options{
			Assets:     frontend.WebFS,
			Middleware: assetMiddleware(frontend.WebFS, app.GetAPIHandler),
		},
		Menu:          newApplicationMenu(app),
		OnStartup:     app.Startup,
		OnDomReady:    app.DomReady,
		OnBeforeClose: app.beforeClose,
		OnShutdown: func(context.Context) {
			app.Shutdown()
		},

		Bind: []any{app},
		Mac: &mac.Options{
			Preferences: &mac.Preferences{
				FullscreenEnabled: mac.Enabled,
			},
		},
	})

	if err != nil {
		log.Fatal(err)
	}
}
