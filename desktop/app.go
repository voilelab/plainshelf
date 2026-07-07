package main

import (
	"context"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/voilelab/plainshelf/internal/logutil"
	"github.com/voilelab/plainshelf/internal/util"
	"github.com/voilelab/plainshelf/internal/version"
	"github.com/voilelab/plainshelf/server"
	"github.com/voilelab/plainshelf/shelf"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type DesktopApp struct {
	app               *server.App
	apiHandler        http.Handler
	ctx               context.Context
	shelvesConfigPath string
	startupErr        error
}

type DesktopImportBookResult struct {
	Path  string `json:"path"`
	ID    string `json:"id,omitempty"`
	Error string `json:"error,omitempty"`
}

type DesktopShelfDetails struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Path         string `json:"path"`
	ScanInterval string `json:"scan_interval"`
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

func (a *DesktopApp) GetAPIHandler() http.Handler {
	if a.apiHandler == nil {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "server starting", http.StatusServiceUnavailable)
		})
	}
	return a.apiHandler
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

func (a *DesktopApp) SaveBookContent(shelfID, bookID, suggestedName string) error {
	if a.ctx == nil {
		return util.NewError("desktop context not ready")
	}

	savePath, err := wailsruntime.SaveFileDialog(a.ctx, wailsruntime.SaveDialogOptions{
		DefaultFilename: suggestedName,
		Title:           "Save book",
		Filters: []wailsruntime.FileFilter{
			{
				DisplayName: "Text Files (*.txt)",
				Pattern:     "*.txt",
			},
		},
	})
	if err != nil {
		return util.Errorf("%w", err)
	}
	if savePath == "" {
		return nil // user cancelled
	}

	apiPath, err := url.JoinPath("/api/shelves", shelfID, "books", bookID, "content")
	if err != nil {
		return util.Errorf("building content path: %w", err)
	}
	req := httptest.NewRequest(http.MethodGet, apiPath, nil).WithContext(a.ctx)

	rec := httptest.NewRecorder()
	a.GetAPIHandler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		return util.Errorf("fetching book content: HTTP %d", rec.Code)
	}

	if err := os.WriteFile(savePath, rec.Body.Bytes(), 0o600); err != nil {
		return util.Errorf("writing file: %w", err)
	}

	return nil
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

func (a *DesktopApp) OpenBookFolder(shelfID, bookID string) error {
	if a.app == nil {
		return util.NewError("desktop backend app instance is nil")
	}

	if err := a.app.OpenBookFolder(shelfID, bookID); err != nil {
		return util.Errorf("%w", err)
	}

	return nil
}

func resolveDesktopLayerDirectory(libRoot string, layerParts []string) (string, error) {
	normalizedRoot, err := normalizeDesktopShelfDirectory(libRoot)
	if err != nil {
		return "", util.Errorf("%w", err)
	}

	booksRoot := filepath.Clean(filepath.Join(normalizedRoot, "books"))
	targetPathParts := append([]string{booksRoot}, layerParts...)
	targetDir := filepath.Clean(filepath.Join(targetPathParts...))

	relPath, err := filepath.Rel(booksRoot, targetDir)
	if err != nil {
		return "", util.Errorf("resolving layer directory: %w", err)
	}
	if relPath == ".." || strings.HasPrefix(relPath, ".."+string(os.PathSeparator)) {
		return "", util.Errorf("invalid layer path")
	}

	return targetDir, nil
}

func (a *DesktopApp) OpenLayerDirectory(shelfID string, layerParts []string) error {
	shelfID = strings.TrimSpace(shelfID)
	if shelfID == "" {
		return util.Errorf("shelf ID cannot be empty")
	}

	conf, err := loadDesktopShelves(a.shelvesConfigPath)
	if err != nil {
		return util.Errorf("loading shelf config: %w", err)
	}

	var libRoot string
	for _, entry := range conf.Shelves {
		if entry.ID == shelfID {
			libRoot = entry.LibRoot
			break
		}
	}
	if libRoot == "" {
		return util.Errorf("shelf with ID %q not found", shelfID)
	}

	// normalizeLayerParts trims user-provided segments and drops empty entries;
	// resolveDesktopLayerDirectory then enforces that the final path stays under
	// <shelf>/books.
	targetDir, err := resolveDesktopLayerDirectory(libRoot, normalizeLayerParts(layerParts))
	if err != nil {
		return util.Errorf("%w", err)
	}

	info, err := os.Stat(targetDir)
	if err != nil {
		return util.Errorf("layer directory unavailable: %w", err)
	}
	if !info.IsDir() {
		return util.Errorf("layer path is not a directory")
	}

	if err := openFinder(targetDir); err != nil {
		return util.Errorf("%w", err)
	}

	return nil
}

func (a *DesktopApp) AddShelf(name, libRoot, scanInterval string) error {
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

	scanInterval = strings.TrimSpace(scanInterval)

	conf, err := loadDesktopShelves(a.shelvesConfigPath)
	if err != nil {
		return util.Errorf("loading shelf config: %w", err)
	}

	existingIDs := map[string]bool{}
	for _, entry := range conf.Shelves {
		existingIDs[entry.ID] = true
	}

	id := generateDesktopShelfID(name, existingIDs)

	entry := desktopShelfEntry{
		ID:           id,
		Name:         name,
		LibRoot:      normalizedLibRoot,
		ScanInterval: scanInterval,
	}

	err = a.app.AddShelf(toShelfConfWithID(entry))
	if err != nil {
		return util.Errorf("registering shelf: %w", err)
	}

	conf.Shelves = append(conf.Shelves, entry)
	if err := saveDesktopShelves(a.shelvesConfigPath, conf); err != nil {
		if removeErr := a.app.RemoveShelf(id); removeErr != nil {
			return util.Errorf("saving shelf config: %w; rolling back runtime shelf: %v", err, removeErr)
		}
		return util.Errorf("saving shelf config: %w", err)
	}

	return nil
}

func (a *DesktopApp) GetShelfDetails(shelfID string) (*DesktopShelfDetails, error) {
	if a.app == nil {
		return nil, util.NewError("desktop backend app instance is nil")
	}

	shelfID = strings.TrimSpace(shelfID)
	if shelfID == "" {
		return nil, util.Errorf("shelf ID cannot be empty")
	}

	conf, err := loadDesktopShelves(a.shelvesConfigPath)
	if err != nil {
		return nil, util.Errorf("loading shelf config: %w", err)
	}

	for _, entry := range conf.Shelves {
		if entry.ID == shelfID {
			return &DesktopShelfDetails{
				ID:           entry.ID,
				Name:         entry.Name,
				Path:         entry.LibRoot,
				ScanInterval: entry.ScanInterval,
			}, nil
		}
	}

	return nil, util.Errorf("shelf with ID %q not found", shelfID)
}

func (a *DesktopApp) ModifyShelf(shelfID, name, scanInterval string) error {
	if a.app == nil {
		return util.NewError("desktop backend app instance is nil")
	}

	shelfID = strings.TrimSpace(shelfID)
	if shelfID == "" {
		return util.Errorf("shelf ID cannot be empty")
	}

	name = strings.TrimSpace(name)
	if name == "" {
		return util.Errorf("shelf name cannot be empty")
	}

	scanInterval = strings.TrimSpace(scanInterval)

	conf, err := loadDesktopShelves(a.shelvesConfigPath)
	if err != nil {
		return util.Errorf("loading shelf config: %w", err)
	}

	var found *desktopShelfEntry
	for i := range conf.Shelves {
		if conf.Shelves[i].ID == shelfID {
			found = &conf.Shelves[i]
			break
		}
	}
	if found == nil {
		return util.Errorf("shelf with ID %q not found in config", shelfID)
	}

	oldName := found.Name
	oldScanInterval := found.ScanInterval

	if err := a.app.UpdateShelf(shelfID, name, scanInterval); err != nil {
		return util.Errorf("updating shelf: %w", err)
	}

	found.Name = name
	found.ScanInterval = scanInterval

	if err := saveDesktopShelves(a.shelvesConfigPath, conf); err != nil {
		if rollbackErr := a.app.UpdateShelf(shelfID, oldName, oldScanInterval); rollbackErr != nil {
			return util.Errorf("saving shelf config: %w; rolling back runtime shelf: %v", err, rollbackErr)
		}
		return util.Errorf("saving shelf config: %w", err)
	}

	return nil
}

func (a *DesktopApp) RemoveShelf(shelfID string) error {
	if a.app == nil {
		return util.NewError("desktop backend app instance is nil")
	}

	shelfID = strings.TrimSpace(shelfID)
	if shelfID == "" {
		return util.Errorf("shelf ID cannot be empty")
	}

	conf, err := loadDesktopShelves(a.shelvesConfigPath)
	if err != nil {
		return util.Errorf("loading shelf config: %w", err)
	}

	newShelves := make([]desktopShelfEntry, 0, len(conf.Shelves))
	found := false
	for _, entry := range conf.Shelves {
		if entry.ID == shelfID {
			found = true
			continue
		}
		newShelves = append(newShelves, entry)
	}

	if !found {
		return util.Errorf("shelf with ID %q not found in config", shelfID)
	}

	conf.Shelves = newShelves
	if err := saveDesktopShelves(a.shelvesConfigPath, conf); err != nil {
		return util.Errorf("saving shelf config: %w", err)
	}

	if err := a.app.RemoveShelf(shelfID); err != nil {
		return util.Errorf("removing shelf from runtime: %w", err)
	}

	return nil
}

func (a *DesktopApp) startServer() error {
	log.Println("PlainShelf version:", version.Version)

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
