package main

import (
	"context"
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
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
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

	native := &wailsNative{}
	app := NewReaderApp(native, logger, launch.shelfID, launch.section)

	if launch.bookPath != "" {
		if _, err := app.Library().Open(launch.bookPath); err != nil {
			log.Println("failed to open the book package given on the command line:", err)
		}
	}

	api := readerapi.NewHandler(app.Library(), version.Version)
	spa := readerapi.NewSPAHandler(frontend.WebFS, app.BootConfig)
	appMenu, err := toWailsMenu(applicationMenu(app))
	if err != nil {
		log.Fatal(err)
	}

	wailsApp := application.New(application.Options{
		Name:     "PlainShelf Reader",
		Services: []application.Service{application.NewService(app)},
		Assets: application.AssetOptions{
			Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if strings.HasPrefix(r.URL.Path, "/api/") || r.URL.Path == "/api" {
					api.ServeHTTP(w, r)
					return
				}
				spa.ServeHTTP(w, r)
			}),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
		OnShutdown: func() { app.Shutdown(context.Background()) },
	})
	wailsApp.Menu.SetApplicationMenu(appMenu)

	window := wailsApp.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:  "PlainShelf Reader",
		Width:  1100,
		Height: 900,
		URL:    "/",
	})
	native.attach(wailsApp, window)
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
