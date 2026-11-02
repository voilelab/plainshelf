package main

import (
	"context"
	"log"
	"runtime"

	"github.com/voilelab/plainshelf/frontend"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
)

func main() {
	native := &wailsNative{}
	app := NewDesktopApp(native)
	appMenu, err := toWailsMenu(applicationMenu(app, runtime.GOOS))
	if err != nil {
		log.Fatal(err)
	}

	err = wails.Run(&options.App{
		Title:  "PlainShelf",
		Width:  1600,
		Height: 1200,
		AssetServer: &assetserver.Options{
			Assets:     frontend.WebFS,
			Middleware: assetMiddleware(frontend.WebFS, app.GetAPIHandler),
		},
		Menu: appMenu,
		OnStartup: func(ctx context.Context) {
			native.attach(ctx)
			app.Startup(ctx)
		},
		// Dialogs wait for DomReady: from OnStartup the window is not ready and
		// MessageDialog panics on Windows (wailsapp/wails#1660).
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
