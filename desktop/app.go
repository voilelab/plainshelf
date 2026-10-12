package main

import (
	"context"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/voilelab/plainshelf/internal/logutil"
	"github.com/voilelab/plainshelf/internal/readingclose"
	"github.com/voilelab/plainshelf/internal/readingprogress"
	"github.com/voilelab/plainshelf/internal/util"
	"github.com/voilelab/plainshelf/internal/version"
	"github.com/voilelab/plainshelf/server"
	"github.com/voilelab/plainshelf/shelf"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type DesktopApp struct {
	app                 *server.App
	apiHandler          http.Handler
	ctx                 context.Context
	shelvesConfigPath   string
	readHistoryPath     string
	readingProgressPath string
	readingStatsPath    string
	readingProgressSync *readingprogress.Store
	startupErr          error
	progressStager      *readingclose.Stager
}

var openFinder = util.OpenFinder

func NewDesktopApp() *DesktopApp {
	return &DesktopApp{}
}

func (a *DesktopApp) Startup(ctx context.Context) {
	a.ctx = ctx
	err := a.startServer()
	if err != nil {
		// Don't call runtime methods (e.g. MessageDialog) here: from
		// OnStartup the window is still initializing and they are not
		// guaranteed to work — on Windows MessageDialog panics. Record the
		// failure and report it from DomReady instead. See wailsapp/wails#1660.
		log.Println("Failed to start PlainShelf backend:", err)
		a.startupErr = err
		return
	}
	a.apiHandler = a.app.Handler()
}

// DomReady runs once the window and DOM are ready, which is the safe point to
// use runtime methods. If the backend failed to start, surface the cause and
// quit gracefully instead of leaving the user staring at a dead UI.
func (a *DesktopApp) DomReady(ctx context.Context) {
	if a.startupErr == nil {
		return
	}

	_, dialogErr := wailsruntime.MessageDialog(ctx, wailsruntime.MessageDialogOptions{
		Type:    wailsruntime.ErrorDialog,
		Title:   "PlainShelf failed to start",
		Message: "PlainShelf could not start its backend and will now close.\n\n" + a.startupErr.Error(),
	})
	if dialogErr != nil {
		log.Println("Failed to show startup error dialog:", dialogErr)
	}

	wailsruntime.Quit(ctx)
}

func (a *DesktopApp) Shutdown() {
	if a.app != nil {
		err := a.app.Close()
		if err != nil {
			log.Println("Failed to close app:", err)
		}
	}
}

// beforeClose is the OnBeforeClose hook. It writes the reading position the
// frontend last staged to disk before allowing the window to close; see
// readingclose.Stager. It is unexported so Wails does not bind it as a frontend
// method.
func (a *DesktopApp) beforeClose(context.Context) (prevent bool) {
	a.progressStager.PersistOnClose()
	return false
}

func (a *DesktopApp) GetAPIHandler() http.Handler {
	if a.apiHandler == nil {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "server starting", http.StatusServiceUnavailable)
		})
	}
	return a.apiHandler
}

func (a *DesktopApp) OpenExternalURL(rawURL string) error {
	rawURL = strings.TrimSpace(rawURL)
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed == nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return util.Errorf("invalid external URL: %q", rawURL)
	}

	wailsruntime.BrowserOpenURL(a.ctx, parsed.String())
	return nil
}

func (a *DesktopApp) PreviousPage() {
	a.navigateHistory(-1)
}

func (a *DesktopApp) NextPage() {
	a.navigateHistory(1)
}

func (a *DesktopApp) ZoomIn() {
	a.runZoomScript("in")
}

func (a *DesktopApp) ZoomOut() {
	a.runZoomScript("out")
}

func (a *DesktopApp) ResetZoom() {
	a.runZoomScript("reset")
}

func (a *DesktopApp) navigateHistory(step int) {
	if a.ctx == nil {
		return
	}

	script := historyNavigationScript(step)
	if script == "" {
		return
	}

	wailsruntime.WindowExecJS(a.ctx, script)
}

func (a *DesktopApp) runZoomScript(action string) {
	if a.ctx == nil {
		return
	}

	script := zoomScript(action)
	if script == "" {
		return
	}

	wailsruntime.WindowExecJS(a.ctx, script)
}

func (a *DesktopApp) startServer() error {
	log.Println("PlainShelf version:", version.Version)

	// Shared with the standalone reader, which writes progress here.
	dataRoot, err := readingprogress.SharedDataDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dataRoot, 0o755); err != nil {
		return util.Errorf("%w", err)
	}

	shelvesConfigPath := filepath.Join(dataRoot, "shelves.json")
	storedConf, err := loadOrMigrateDesktopShelves(shelvesConfigPath, dataRoot)
	if err != nil {
		return util.Errorf("loading shelf config: %w", err)
	}

	shelves := []*shelf.ShelfConfWithID{}
	for _, entry := range storedConf.Shelves {
		conf := toShelfConfWithID(entry)
		shelves = append(shelves, &conf)
	}

	appConf := &server.AppConf{
		Logger: logutil.LogConf{
			Level:  "info",
			Format: "json",
			LogFile: logutil.LogFileConf{
				Type:   logutil.LogFileTypeNameRotate,
				Dir:    filepath.Join(dataRoot, "logs"),
				Prefix: "app",
			},
		},
		Shelves:    shelves,
		StorePath:  filepath.Join(dataRoot, "store"),
		CoverToJPG: true,
		Security: &server.SecurityConf{
			Mode: server.SecurityModeNone,
		},
	}

	app, err := server.NewApp(appConf)
	if err != nil {
		return util.Errorf("%w", err)
	}

	err = app.Start()
	if err != nil {
		return util.Errorf("%w", err)
	}

	a.shelvesConfigPath = shelvesConfigPath
	a.readHistoryPath = filepath.Join(dataRoot, "read_history.json")
	a.readingProgressPath = filepath.Join(dataRoot, "reading_progress.json")
	a.readingStatsPath = filepath.Join(dataRoot, "reading_stats.json")
	a.readingProgressSync = readingprogress.NewStore(a.readingProgressPath)
	a.progressStager = readingclose.NewStager(a.readingProgressSync, readingclose.DefaultTimeout)
	a.app = app
	return nil
}
