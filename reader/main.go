package main

import (
	"flag"
	"io"
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
	launch, err := parseLaunchArgs(os.Args[1:])
	if err != nil {
		log.Fatal("invalid reader launch arguments: ", err)
	}
	logger, err := logutil.NewLogger(&logutil.LogConf{
		Level:   "info",
		Format:  "text",
		LogFile: logutil.LogFileConf{Type: logutil.LogFileTypeStderr},
	})
	if err != nil {
		log.Fatal(err)
	}

	app := NewReaderApp(logger, launch.shelfID, launch.section)

	if launch.bookPath != "" {
		if _, err := app.Library().Open(launch.bookPath); err != nil {
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

			Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if strings.HasPrefix(r.URL.Path, "/api/") || r.URL.Path == "/api" {
					api.ServeHTTP(w, r)
					return
				}
				spa.ServeHTTP(w, r)
			}),
		},
		Menu:          newApplicationMenu(app),
		OnStartup:     app.Startup,
		OnDomReady:    app.DomReady,
		OnBeforeClose: app.beforeClose,
		OnShutdown:    app.Shutdown,
		Bind:          []any{app},
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

const noLaunchSection = -1

type launchArgs struct {
	bookPath string
	shelfID  string

	section int
}

func parseLaunchArgs(args []string) (launchArgs, error) {
	flags := flag.NewFlagSet("plainshelf-reader", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	book := flags.String("book", "", "path to the .bookpkg folder to open")
	shelf := flags.String("shelf", "", "real shelf id the book belongs to; empty uses the synthetic reader shelf")
	section := flags.Int("section", noLaunchSection, "reader section index to open at; negative opens at the restored progress")

	if err := flags.Parse(args); err != nil {
		return launchArgs{}, err
	}
	return launchArgs{bookPath: *book, shelfID: *shelf, section: *section}, nil
}

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
