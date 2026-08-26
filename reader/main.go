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

	// -shelf carries the real desktop shelf id when the desktop app shells out to
	// the reader, so the reader opens at (and writes back to) the book's existing
	// position in the desktop library. Empty (standalone launch) falls back to the
	// synthetic reader shelf. -section carries the chapter to open at (negative
	// when absent), so a desktop chapter jump lands on that chapter rather than the
	// restored progress.
	app := NewReaderApp(logger, shelfIDFromArgs(os.Args[1:]), sectionFromArgs(os.Args[1:]))

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

// bookPathFromArgs reads -book from the arguments the app was launched with,
// shelfIDFromArgs reads -shelf, and sectionFromArgs reads -section. All delegate
// to parseLaunchArgs so the flags are defined on one FlagSet: flag.Parse stops at
// the first argument it does not recognize, so a set that knew only -book would
// drop a following -shelf (and one that knew only -shelf would stop at a leading
// -book). The desktop app passes them together.
func bookPathFromArgs(args []string) string {
	return parseLaunchArgs(args).bookPath
}

func shelfIDFromArgs(args []string) string {
	return parseLaunchArgs(args).shelfID
}

func sectionFromArgs(args []string) int {
	return parseLaunchArgs(args).section
}

// noLaunchSection is the -section value that means "no chapter was requested";
// any negative value works, but the flag defaults to this one.
const noLaunchSection = -1

// launchArgs holds the reader-specific flags parsed from the launch arguments.
type launchArgs struct {
	bookPath string
	shelfID  string
	// section is the reader section index to open at, or noLaunchSection when
	// -section was not passed. Section 0 is a real chapter, so the "absent" case
	// is a negative sentinel rather than the zero value.
	section int
}

// parseLaunchArgs reads -book, -shelf, and -section from the arguments the app
// was launched with.
//
// Its own FlagSet rather than the package-level one: wails parses os.Args itself
// in dev mode, and this has to be callable from a test. Errors are discarded the
// same way wails discards its own ("Parse args but ignore errors in case -appargs
// was used"): an argument this app does not define is not its to reject, and
// either way there is nothing here to act on.
func parseLaunchArgs(args []string) launchArgs {
	flags := flag.NewFlagSet("plainshelf-reader", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	book := flags.String("book", "", "path to the .bookpkg folder to open")
	shelf := flags.String("shelf", "", "real shelf id the book belongs to; empty uses the synthetic reader shelf")
	section := flags.Int("section", noLaunchSection, "reader section index to open at; negative opens at the restored progress")

	_ = flags.Parse(args)
	return launchArgs{bookPath: *book, shelfID: *shelf, section: *section}
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
