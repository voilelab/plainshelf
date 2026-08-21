package main

import (
	"log"
	"net/http"
	"strings"

	"github.com/voilelab/plainshelf/frontend"
	"github.com/voilelab/plainshelf/internal/logutil"
	"github.com/voilelab/plainshelf/internal/version"
	"github.com/voilelab/plainshelf/reader/readerapi"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/menu/keys"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
)

func main() {
	// Warnings about a book.json from a newer build are worth seeing when the
	// app is started from a terminal, and cost nothing when it is not.
	logger, err := logutil.NewLogger(&logutil.LogConf{
		Level:   "info",
		Format:  "text",
		LogFile: logutil.LogFileConf{Type: logutil.LogFileTypeStderr},
	})
	if err != nil {
		log.Fatal(err)
	}

	app := NewReaderApp(logger)
	api := readerapi.NewHandler(app.Library(), version.Version)
	spa := readerapi.NewSPAHandler(frontend.WebFS, app.BootConfig)

	err = wails.Run(&options.App{
		Title:  "PlainShelf Reader",
		Width:  1100,
		Height: 900,
		AssetServer: &assetserver.Options{
			// Handler rather than Assets: the reader serves the embedded
			// frontend itself so it can inject the open book into index.html
			// and answer in-app routes with it.
			Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if strings.HasPrefix(r.URL.Path, "/api/") || r.URL.Path == "/api" {
					api.ServeHTTP(w, r)
					return
				}
				spa.ServeHTTP(w, r)
			}),
		},
		Menu:       newApplicationMenu(app),
		OnStartup:  app.Startup,
		OnDomReady: app.DomReady,
		OnShutdown: app.Shutdown,
		Bind:       []any{app},
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

// newApplicationMenu keeps the reader's menu to what a reader can do: open
// another book, and the editing/window items macOS users expect to exist.
func newApplicationMenu(app *ReaderApp) *menu.Menu {
	root := menu.NewMenu()
	root.Append(menu.AppMenu())

	fileMenu := root.AddSubmenu("File")
	fileMenu.AddText("Open Book…", keys.CmdOrCtrl("o"), func(*menu.CallbackData) {
		if _, err := app.OpenBookPackage(); err != nil {
			log.Println("failed to open a book package:", err)
		}
	})

	root.Append(menu.EditMenu())
	return root
}
