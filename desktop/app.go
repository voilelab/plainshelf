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
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type DesktopApp struct {
	app *server.App
	ctx context.Context
}

type DesktopSelectedBookFile struct {
	Path    string `json:"path"`
	Name    string `json:"name"`
	Content []byte `json:"content"`
}

func NewDesktopApp() *DesktopApp {
	return &DesktopApp{}
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

func (a *DesktopApp) OpenBookFiles() ([]DesktopSelectedBookFile, error) {
	if a.ctx == nil {
		return []DesktopSelectedBookFile{}, nil
	}

	paths, err := wailsruntime.OpenMultipleFilesDialog(a.ctx, bookOpenDialogOptions())
	if err != nil {
		return nil, util.Errorf("%w", err)
	}
	if len(paths) == 0 {
		return []DesktopSelectedBookFile{}, nil
	}

	files, err := loadDesktopSelectedBookFiles(paths)
	if err != nil {
		return nil, err
	}

	return files, nil
}

func bookOpenDialogOptions() wailsruntime.OpenDialogOptions {
	return wailsruntime.OpenDialogOptions{
		Title: "Select books to import",
		Filters: []wailsruntime.FileFilter{
			{
				DisplayName: "Text Files (*.txt)",
				Pattern:     "*.txt",
			},
		},
	}
}

func loadDesktopSelectedBookFiles(paths []string) ([]DesktopSelectedBookFile, error) {
	files := make([]DesktopSelectedBookFile, 0, len(paths))

	for _, currentPath := range paths {
		if currentPath == "" {
			continue
		}

		content, err := os.ReadFile(currentPath)
		if err != nil {
			return nil, util.Errorf("%w", err)
		}

		files = append(files, DesktopSelectedBookFile{
			Path:    currentPath,
			Name:    filepath.Base(currentPath),
			Content: content,
		})
	}

	return files, nil
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
