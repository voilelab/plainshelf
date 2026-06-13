package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/voilelab/plainshelf/internal/logutil"
	"github.com/voilelab/plainshelf/internal/util"
	"github.com/voilelab/plainshelf/server"
	"github.com/voilelab/plainshelf/shelf"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type DesktopApp struct {
	app *server.App
	ctx context.Context

	dataRoot          string
	shelvesConfigPath string
	desktopShelves    []*shelf.ShelfConfWithID
	serverMu          sync.Mutex
	shelfMu           sync.Mutex
}

type DesktopImportBookResult struct {
	Path  string `json:"path"`
	ID    string `json:"id,omitempty"`
	Error string `json:"error,omitempty"`
}

type DesktopShelfInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func NewDesktopApp() *DesktopApp {
	return &DesktopApp{}
}

func (a *DesktopApp) Startup(ctx context.Context) {
	a.ctx = ctx
	if err := a.ensureServer(); err != nil {
		panic(err)
	}
}

func (a *DesktopApp) Shutdown() {
	a.serverMu.Lock()
	defer a.serverMu.Unlock()

	if a.app != nil {
		err := a.app.Close()
		if err != nil {
			log.Println("Failed to close app:", err)
		}
		a.app = nil
	}
}

func (a *DesktopApp) GetAPIHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := a.ensureServer(); err != nil {
			log.Println("Failed to start app for API request:", err)
			http.Error(w, "desktop app is not ready", http.StatusServiceUnavailable)
			return
		}

		app := a.currentServerApp()
		if app == nil {
			http.Error(w, "desktop app is not ready", http.StatusServiceUnavailable)
			return
		}

		app.Handler().ServeHTTP(w, r)
	})
}

func (a *DesktopApp) ensureServer() error {
	a.serverMu.Lock()
	defer a.serverMu.Unlock()

	if a.app != nil {
		return nil
	}
	return a.startServer()
}

func (a *DesktopApp) currentServerApp() *server.App {
	a.serverMu.Lock()
	defer a.serverMu.Unlock()
	return a.app
}

func (a *DesktopApp) PreviousPage() {
	a.navigateHistory(-1)
}

func (a *DesktopApp) NextPage() {
	a.navigateHistory(1)
}

func (a *DesktopApp) OpenBookFiles() ([]string, error) {
	if a.ctx == nil {
		return []string{}, nil
	}

	paths, err := wailsruntime.OpenMultipleFilesDialog(a.ctx, bookOpenDialogOptions())
	if err != nil {
		return nil, util.Errorf("%w", err)
	}
	return normalizeSelectedLocalPaths(paths), nil
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

func normalizeSelectedLocalPaths(paths []string) []string {
	localPaths := make([]string, 0, len(paths))
	for _, currentPath := range paths {
		trimmedPath := strings.TrimSpace(currentPath)
		if trimmedPath == "" {
			continue
		}
		localPaths = append(localPaths, trimmedPath)
	}
	return localPaths
}

func normalizeLayerParts(layerParts []string) shelf.Layers {
	normalizedParts := make(shelf.Layers, 0, len(layerParts))
	for _, part := range layerParts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		normalizedParts = append(normalizedParts, trimmed)
	}
	return normalizedParts
}

func (a *DesktopApp) OpenShelfDirectory() (string, error) {
	if a.ctx == nil {
		return "", nil
	}

	path, err := wailsruntime.OpenDirectoryDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title: "Select shelf library folder",
	})
	if err != nil {
		return "", util.Errorf("%w", err)
	}
	return strings.TrimSpace(path), nil
}

func (a *DesktopApp) AddShelf(name string, libRoot string) (*DesktopShelfInfo, error) {
	if err := a.ensureServer(); err != nil {
		return nil, util.Errorf("%w", err)
	}

	a.shelfMu.Lock()
	defer a.shelfMu.Unlock()

	conf, err := a.newDesktopShelfConf(name, libRoot)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	app := a.currentServerApp()
	if app == nil {
		return nil, util.NewError("desktop backend app instance is nil")
	}

	if err := app.RegisterShelf(*conf); err != nil {
		return nil, util.Errorf("%w", err)
	}

	nextShelves := append(cloneShelfConfs(a.desktopShelves), cloneShelfConf(conf))
	if err := saveDesktopShelfConfig(a.shelvesConfigPath, nextShelves); err != nil {
		if rollbackErr := app.UnregisterShelf(conf.ID); rollbackErr != nil {
			return nil, util.Errorf("failed to save shelf config: %w; rollback failed: %v", err, rollbackErr)
		}
		return nil, util.Errorf("%w", err)
	}

	a.desktopShelves = nextShelves
	return &DesktopShelfInfo{ID: conf.ID, Name: conf.Name}, nil
}

func (a *DesktopApp) ImportBooksFromLocalPaths(shelfID string, localPaths []string, layerParts []string) ([]DesktopImportBookResult, error) {
	if err := a.ensureServer(); err != nil {
		return nil, util.Errorf("%w", err)
	}
	app := a.currentServerApp()
	if app == nil {
		return nil, util.NewError("desktop backend app instance is nil")
	}

	normalizedPaths := normalizeSelectedLocalPaths(localPaths)
	if len(normalizedPaths) == 0 {
		return []DesktopImportBookResult{}, nil
	}

	normalizedLayerParts := normalizeLayerParts(layerParts)
	results := make([]DesktopImportBookResult, 0, len(normalizedPaths))
	for _, localPath := range normalizedPaths {
		book, err := app.ImportFromLocalPath(shelfID, localPath, normalizedLayerParts)
		result := DesktopImportBookResult{Path: localPath}
		if err != nil {
			result.Error = err.Error()
		} else {
			result.ID = book.ID()
		}
		results = append(results, result)
	}

	return results, nil
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

	shelvesConfigPath := filepath.Join(dataRoot, desktopShelfConfigFilename)
	desktopShelves, err := loadOrCreateDesktopShelfConfig(shelvesConfigPath, defaultDesktopShelfConf(dataRoot))
	if err != nil {
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
		Shelves:          desktopShelves,
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
	a.dataRoot = dataRoot
	a.shelvesConfigPath = shelvesConfigPath
	a.desktopShelves = cloneShelfConfs(desktopShelves)
	return nil
}
