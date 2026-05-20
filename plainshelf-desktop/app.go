package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/voilelab/plainshelf/internal/util"
	"github.com/voilelab/plainshelf/server"
)

type DesktopApp struct {
	ctx    context.Context
	cancel context.CancelFunc
	app    *server.App
}

func NewDesktopApp() *DesktopApp {
	return &DesktopApp{}
}

func (a *DesktopApp) Startup(ctx context.Context) {
	a.ctx, a.cancel = context.WithCancel(ctx)
	if err := a.startServer(); err != nil {
		panic(err)
	}
}

func (a *DesktopApp) Shutdown() {
	if a.cancel != nil {
		a.cancel()
	}
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
	// Currently we only support darwin
	dataRoot, err := os.UserConfigDir()
	if err != nil {
		return util.Errorf("%w", err)
	}
	dataRoot = filepath.Join(dataRoot, "PlainShelf")
	if err := os.MkdirAll(dataRoot, 0o755); err != nil {
		return util.Errorf("%w", err)
	}

	appConf := &server.AppConf{
		ShelfPath:        filepath.Join(dataRoot, "shelf"),
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
