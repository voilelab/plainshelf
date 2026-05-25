package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"github.com/voilelab/plainshelf/internal/logutil"
	"github.com/voilelab/plainshelf/internal/util"
	"github.com/voilelab/plainshelf/server"
	"github.com/voilelab/plainshelf/shelf"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	defaultZoomFactor = 1.0
	zoomStep          = 0.1
	minZoomFactor     = 0.5
	maxZoomFactor     = 4.0
)

type DesktopApp struct {
	app        *server.App
	ctx        context.Context
	zoomMu     sync.Mutex
	zoomFactor float64
}

func NewDesktopApp() *DesktopApp {
	return &DesktopApp{zoomFactor: defaultZoomFactor}
}

func (a *DesktopApp) Startup(ctx context.Context) {
	a.ctx = ctx
	err := a.startServer()
	if err != nil {
		panic(err)
	}
}

func (a *DesktopApp) Shutdown() {
	if a.app != nil {
		err := a.app.Close()
		if err != nil {
			log.Println("Failed to close app:", err)
		}
	}
}

func (a *DesktopApp) GetAPIHandler() http.Handler {
	return a.app.Handler()
}

func (a *DesktopApp) PreviousPage() {
	a.navigateHistory(-1)
}

func (a *DesktopApp) NextPage() {
	a.navigateHistory(1)
}

func (a *DesktopApp) ZoomIn() {
	a.zoomMu.Lock()
	current := a.zoomFactor
	a.zoomMu.Unlock()
	a.setZoom(current + zoomStep)
}

func (a *DesktopApp) ZoomOut() {
	a.zoomMu.Lock()
	current := a.zoomFactor
	a.zoomMu.Unlock()
	a.setZoom(current - zoomStep)
}

func (a *DesktopApp) ResetZoom() {
	a.setZoom(defaultZoomFactor)
}

func (a *DesktopApp) GetZoomFactor() float64 {
	a.zoomMu.Lock()
	defer a.zoomMu.Unlock()
	return a.zoomFactor
}

func (a *DesktopApp) setZoom(factor float64) {
	if factor < minZoomFactor {
		factor = minZoomFactor
	}
	if factor > maxZoomFactor {
		factor = maxZoomFactor
	}
	a.zoomMu.Lock()
	a.zoomFactor = factor
	ctx := a.ctx
	a.zoomMu.Unlock()
	if ctx == nil {
		return
	}
	wailsruntime.WindowExecJS(ctx, zoomScript(factor))
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

func (a *DesktopApp) startServer() error {
	// Store desktop app data under the current user's config directory.
	dataRoot, err := os.UserConfigDir()
	if err != nil {
		return util.Errorf("%w", err)
	}
	dataRoot = filepath.Join(dataRoot, "PlainShelf")
	if err := os.MkdirAll(dataRoot, 0o755); err != nil {
		return util.Errorf("%w", err)
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
		Shelf: &shelf.ShelfConf{
			Logger: logutil.LogConf{
				Level:  "info",
				Format: "json",
				LogFile: logutil.LogFileConf{
					Type:   logutil.LogFileTypeNameRotate,
					Dir:    filepath.Join(dataRoot, "logs"),
					Prefix: "shelf",
				},
			},
			LibRoot: filepath.Join(dataRoot, "shelf"),
		},
		StorePath:        filepath.Join(dataRoot, "store"),
		CoverToJPG:       true,
		ReadHistoryLimit: 100,
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

	a.app = app
	return nil
}
