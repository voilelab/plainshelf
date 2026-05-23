package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/voilelab/plainshelf/internal/logutil"
	"github.com/voilelab/plainshelf/internal/util"
	"github.com/voilelab/plainshelf/server"
	"github.com/voilelab/plainshelf/shelf"
)

type DesktopApp struct {
	app *server.App
}

func NewDesktopApp() *DesktopApp {
	return &DesktopApp{}
}

func (a *DesktopApp) Startup(_ context.Context) {
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
