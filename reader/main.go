package main

import (
	"log"
	"net/http"
	"os"
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

	// -book opens that package instead of prompting for one:
	// `just run-reader path/to/book.bookpkg`, and `open -a PlainShelfReader
	// --args -book path/to/book.bookpkg`. A path that cannot be opened falls
	// through to the folder dialog rather than taking the window down — the
	// user is already looking at an app that can ask.
	if bookPath := bookPathFromArgs(os.Args[1:]); bookPath != "" {
		if _, err := app.Library().Open(bookPath); err != nil {
			log.Println("failed to open the book package given on the command line:", err)
		}
	}

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

// bookPathFromArgs reads the -book argument the app was launched with.
//
// Scanned rather than parsed with the flag package, and named rather than
// positional, because the app does not own its whole command line: `wails dev`
// passes its own flags through to the binary, so an unknown flag must not stop
// the parse and a bare value like the "debug" in "-loglevel debug" must not be
// mistaken for a path.
func bookPathFromArgs(args []string) string {
	for i, arg := range args {
		if value, found := strings.CutPrefix(arg, "-book="); found {
			return value
		}
		if value, found := strings.CutPrefix(arg, "--book="); found {
			return value
		}
		if (arg == "-book" || arg == "--book") && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
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
