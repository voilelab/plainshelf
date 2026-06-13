package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/voilelab/plainshelf/internal/logutil"
	"github.com/voilelab/plainshelf/internal/util"
	"github.com/voilelab/plainshelf/server"
	"github.com/voilelab/plainshelf/shelf"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type DesktopApp struct {
	app               *server.App
	ctx               context.Context
	shelvesConfigPath string
}

type DesktopImportBookResult struct {
	Path  string `json:"path"`
	ID    string `json:"id,omitempty"`
	Error string `json:"error,omitempty"`
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

func (a *DesktopApp) ImportBooksFromLocalPaths(shelfID string, localPaths []string, layerParts []string) ([]DesktopImportBookResult, error) {
	if a.app == nil {
		return nil, util.NewError("desktop backend app instance is nil")
	}

	normalizedPaths := normalizeSelectedLocalPaths(localPaths)
	if len(normalizedPaths) == 0 {
		return []DesktopImportBookResult{}, nil
	}

	normalizedLayerParts := normalizeLayerParts(layerParts)
	results := make([]DesktopImportBookResult, 0, len(normalizedPaths))
	for _, localPath := range normalizedPaths {
		book, err := a.app.ImportFromLocalPath(shelfID, localPath, normalizedLayerParts)
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

func (a *DesktopApp) OpenShelfDirectory() (string, error) {
	if a.ctx == nil {
		return "", nil
	}
	dir, err := wailsruntime.OpenDirectoryDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title: "Select shelf directory",
	})
	if err != nil {
		return "", util.Errorf("%w", err)
	}
	return dir, nil
}

func (a *DesktopApp) AddShelf(name, libRoot string) error {
	if a.app == nil {
		return util.NewError("desktop backend app instance is nil")
	}

	name = strings.TrimSpace(name)
	if name == "" {
		return util.Errorf("shelf name cannot be empty")
	}

	normalizedLibRoot, err := normalizeDesktopShelfDirectory(libRoot)
	if err != nil {
		return util.Errorf("%w", err)
	}

	conf, err := loadDesktopShelves(a.shelvesConfigPath)
	if err != nil {
		return util.Errorf("loading shelf config: %w", err)
	}

	existingIDs := map[string]bool{}
	for _, entry := range conf.Shelves {
		existingIDs[entry.ID] = true
	}

	// Find a unique ID not in the config and not registered at runtime.
	base := slugifyShelfID(name)
	var id string
	for i := 0; ; i++ {
		if i == 0 {
			id = base
		} else {
			id = fmt.Sprintf("%s-%d", base, i+1)
		}
		if !existingIDs[id] {
			break
		}
	}

	conf.Shelves = append(conf.Shelves, desktopShelfEntry{
		ID:      id,
		Name:    name,
		LibRoot: normalizedLibRoot,
	})

	if err := saveDesktopShelves(a.shelvesConfigPath, conf); err != nil {
		return util.Errorf("saving shelf config: %w", err)
	}

	return nil
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

	shelvesConfigPath := filepath.Join(dataRoot, "shelves.json")
	storedConf, err := loadDesktopShelves(shelvesConfigPath)
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
		Shelves:          shelves,
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

	a.shelvesConfigPath = shelvesConfigPath
	a.app = app
	return nil
}
