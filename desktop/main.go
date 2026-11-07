package main

import (
	"context"
	"log"
	"runtime"

	"github.com/voilelab/plainshelf/frontend"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

func main() {
	native := &wailsNative{}
	app := NewDesktopApp(native)
	app.Startup(context.Background())

	wailsApp := application.New(application.Options{
		Name:     "PlainShelf",
		Services: []application.Service{application.NewService(app)},
		Assets: application.AssetOptions{
			Handler:    application.AssetFileServerFS(frontend.WebFS),
			Middleware: assetMiddleware(frontend.WebFS, app.GetAPIHandler),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
		OnShutdown: app.Shutdown,
	})
	appMenu, err := toWailsMenu(applicationMenu(app, runtime.GOOS))
	if err != nil {
		log.Fatal(err)
	}
	wailsApp.Menu.SetApplicationMenu(appMenu)

	window := wailsApp.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:                 "PlainShelf",
		Width:                 1600,
		Height:                1200,
		URL:                   "/",
		FullscreenButtonState: application.ButtonEnabled,
	})
	native.attach(wailsApp, window)
	app.watchLANShare()
	window.RegisterHook(events.Common.WindowClosing, func(event *application.WindowEvent) {
		if app.beforeClose(context.Background()) {
			event.Cancel()
		}
	})
	window.OnWindowEvent(events.Common.WindowRuntimeReady, func(*application.WindowEvent) {
		app.DomReady(context.Background())
	})

	if err := wailsApp.Run(); err != nil {
		log.Fatal(err)
	}
}
