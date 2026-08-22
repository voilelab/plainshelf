package main

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/voilelab/plainshelf/internal/logutil"
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

// Reading history, progress, and stats are per-device state: they never reach the
// server, and on the desktop they must not depend on WebView storage either
// (clearing site data or a WebView profile change would take them with it).
// Each is stored as a JSON file next to shelves.json instead.
//
// The document formats belong to the frontend (frontend/src/storage/*), which
// owns the single implementation of merging and trimming; this side only checks
// that what it is handed is JSON of a sane size, and persists it.
const maxDeviceDocumentBytes = 1 << 20

// readDeviceDocument returns the stored document, or an empty string when this
// device has not stored one yet.
func readDeviceDocument(path string) (string, error) {
	if path == "" {
		return "", util.NewError("desktop storage is not ready")
	}

	bs, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", util.Errorf("%w", err)
	}
	return string(bs), nil
}

// writeDeviceDocument replaces the stored document. The write is atomic (temp
// file plus rename) so an interrupted write cannot leave a half-written
// document behind.
func writeDeviceDocument(path, label, doc string) error {
	if path == "" {
		return util.NewError("desktop storage is not ready")
	}
	if len(doc) > maxDeviceDocumentBytes {
		return util.Errorf("%s document is too large: %d bytes", label, len(doc))
	}
	if !json.Valid([]byte(doc)) {
		return util.Errorf("%s document is not valid JSON", label)
	}

	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".device_document-*.json")
	if err != nil {
		return util.Errorf("%w", err)
	}
	tmpPath := tmp.Name()

	if _, err := tmp.WriteString(doc); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return util.Errorf("%w", err)
	}
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return util.Errorf("%w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return util.Errorf("%w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return util.Errorf("%w", err)
	}

	return nil
}

// ReadReadHistory returns the stored reading-history document, or an empty
// string when this device has not stored one yet.
func (a *DesktopApp) ReadReadHistory() (string, error) {
	return readDeviceDocument(a.readHistoryPath)
}

// WriteReadHistory replaces the stored reading-history document.
func (a *DesktopApp) WriteReadHistory(doc string) error {
	return writeDeviceDocument(a.readHistoryPath, "read history", doc)
}

// ReadReadingProgress returns the stored reading-progress document, or an empty
// string when this device has not stored one yet.
//
// The standalone reader writes progress into the same file under its own
// synthetic shelf id ("book"). Before handing the document to the desktop
// frontend, any such reader progress is projected onto the real shelves it
// belongs to (by stable book id, taking the max) so the library reflects it.
// This is the projection trigger: it runs whenever the desktop reads progress —
// which is what a returning-from-the-reader user causes — and is cheap when
// there is no reader progress to fold (the common case): no lock, no write.
func (a *DesktopApp) ReadReadingProgress() (string, error) {
	if a.readingProgressSync == nil || a.readerNamespaceIsRealShelf() {
		return readDeviceDocument(a.readingProgressPath)
	}
	return projectStoredReaderProgress(a.readingProgressSync, a.resolveBookShelf)
}

// projectStoredReaderProgress folds any standalone-reader progress in store onto
// the real shelves resolve reports, and returns the document text to hand the
// frontend. It is cheap when there is no reader progress to fold — the common
// case — taking no lock and writing nothing; only genuinely new reader progress
// triggers a locked read-modify-write.
func projectStoredReaderProgress(store *readingprogress.Store, resolve readingprogress.ResolveShelf) (string, error) {
	doc, raw, err := store.Read()
	if err != nil {
		return "", err
	}
	if len(doc.Shelves[readingprogress.ReaderShelfID]) == 0 {
		return raw, nil
	}

	projected := readingprogress.Project(doc, readingprogress.ReaderShelfID, resolve)
	projectedText, err := readingprogress.Serialize(projected)
	if err != nil {
		return raw, nil //nolint:nilerr // convenience state: fall back to the stored view
	}
	currentText, err := readingprogress.Serialize(doc)
	if err != nil || projectedText == currentText {
		return raw, nil //nolint:nilerr // nothing new to fold in
	}

	// There is new reader progress to persist. Fold it in under the store's lock
	// (re-reading fresh, so a concurrent reader write is not lost). Best-effort:
	// still return the projected view if the write fails.
	final, err := store.Mutate(func(disk readingprogress.Document) readingprogress.Document {
		return readingprogress.Project(disk, readingprogress.ReaderShelfID, resolve)
	})
	if err != nil {
		log.Println("failed to persist projected reading progress:", err)
		return projectedText, nil
	}
	finalText, err := readingprogress.Serialize(final)
	if err != nil {
		return projectedText, nil //nolint:nilerr // convenience state: return the projected view
	}
	return finalText, nil
}

// WriteReadingProgress replaces the desktop's reading-progress entries. It only
// mutates the real-shelf namespaces the desktop owns: the standalone reader's
// "book" entries in the same file are re-read under the store's lock and
// preserved, so a desktop write never clobbers reader progress it has not yet
// projected.
func (a *DesktopApp) WriteReadingProgress(doc string) error {
	if a.readingProgressSync == nil || a.readerNamespaceIsRealShelf() {
		return writeDeviceDocument(a.readingProgressPath, "reading progress", doc)
	}

	incoming, err := readingprogress.ParseStrict(doc)
	if err != nil {
		return err
	}
	_, err = a.readingProgressSync.Mutate(func(disk readingprogress.Document) readingprogress.Document {
		return readingprogress.MergeDesktopWrite(disk, incoming, readingprogress.ReaderShelfID)
	})
	return err
}

// resolveBookShelf reports which real shelf holds the given stable book id.
//
// Book ids are only unique within a shelf, so a copied or legacy package can
// carry the same id in more than one shelf. It therefore scans every shelf and
// resolves the book only when exactly one holds it — an ambiguous id is left
// unresolved rather than projected onto whichever shelf happened to come first
// in the map-ordered scan. A shelf that is still initializing (or otherwise
// errors) is treated as "not here": the book stays under the reader's namespace
// and a later read re-attempts the projection once the shelf is ready.
func (a *DesktopApp) resolveBookShelf(bookID string) (string, bool) {
	if a.app == nil {
		return "", false
	}
	manager := a.app.ShelfManager()
	if manager == nil {
		return "", false
	}
	match := ""
	for _, shelfData := range manager.GetAllShelves() {
		if _, err := shelfData.GetBook(bookID); err == nil {
			if match != "" {
				return "", false // ambiguous: more than one shelf holds this id
			}
			match = shelfData.ID
		}
	}
	if match == "" {
		return "", false
	}
	return match, true
}

// readerNamespaceIsRealShelf reports whether a real shelf uses the same id the
// standalone reader stores progress under. In that degenerate configuration the
// "book" namespace belongs to a real shelf, so reader projection is disabled and
// the desktop treats the document as entirely its own — the real shelf must not
// be blocked from saving its own progress. New shelves cannot take this id (see
// generateDesktopShelfID); this guards a config that predates that rule.
func (a *DesktopApp) readerNamespaceIsRealShelf() bool {
	if a.app == nil {
		return false
	}
	manager := a.app.ShelfManager()
	if manager == nil {
		return false
	}
	for _, shelfData := range manager.GetAllShelves() {
		if shelfData.ID == readingprogress.ReaderShelfID {
			return true
		}
	}
	return false
}

// ReadReadingStats returns the stored reading-stats document, or an empty
// string when this device has not stored one yet.
func (a *DesktopApp) ReadReadingStats() (string, error) {
	return readDeviceDocument(a.readingStatsPath)
}

// WriteReadingStats replaces the stored reading-stats document.
func (a *DesktopApp) WriteReadingStats(doc string) error {
	return writeDeviceDocument(a.readingStatsPath, "reading stats", doc)
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
				DisplayName: "Books (*.txt, *.md, *.epub)",
				Pattern:     "*.txt;*.md;*.epub",
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

// resolveBookPackagePath returns the absolute path of a book's .bookpkg
// directory, verified to exist and stay within its shelf. It is shared by
// "reveal in Finder" and "open in the standalone reader".
func (a *DesktopApp) resolveBookPackagePath(shelfID, bookID string) (string, error) {
	if a.app == nil {
		return "", util.NewError("desktop backend app instance is nil")
	}

	shelfID = strings.TrimSpace(shelfID)
	if shelfID == "" {
		return "", util.Errorf("shelf ID cannot be empty")
	}

	conf, err := loadDesktopShelves(a.shelvesConfigPath)
	if err != nil {
		return "", util.Errorf("loading shelf config: %w", err)
	}

	var libRoot string
	for _, entry := range conf.Shelves {
		if entry.ID == shelfID {
			libRoot = entry.LibRoot
			break
		}
	}
	if libRoot == "" {
		return "", util.Errorf("shelf with ID %q not found", shelfID)
	}

	relativeBookPath, err := a.app.GetBookFolderPath(shelfID, bookID)
	if err != nil {
		return "", util.Errorf("resolving book directory: %w", err)
	}

	normalizedRoot, err := normalizeDesktopShelfDirectory(libRoot)
	if err != nil {
		return "", util.Errorf("%w", err)
	}
	targetDir := filepath.Clean(filepath.Join(normalizedRoot, filepath.FromSlash(relativeBookPath)))

	relPath, err := filepath.Rel(normalizedRoot, targetDir)
	if err != nil {
		return "", util.Errorf("resolving book directory: %w", err)
	}
	if relPath == ".." || strings.HasPrefix(relPath, ".."+string(os.PathSeparator)) {
		return "", util.Errorf("invalid book path")
	}

	info, err := os.Stat(targetDir)
	if err != nil {
		return "", util.Errorf("book directory unavailable: %w", err)
	}
	if !info.IsDir() {
		return "", util.Errorf("book path is not a directory")
	}

	return targetDir, nil
}

func (a *DesktopApp) OpenBookDirectory(shelfID, bookID string) error {
	targetDir, err := a.resolveBookPackagePath(shelfID, bookID)
	if err != nil {
		return err
	}

	if err := openFinder(targetDir); err != nil {
		return util.Errorf("%w", err)
	}

	return nil
}

// OpenReader launches the standalone PlainShelfReader in its own window, showing
// the given book. It is the desktop's reading path: the frontend routes the read
// action here on desktop instead of opening the in-app reader.
//
// The reader is a separate macOS app (installed via the Homebrew cask, bundle id
// com.voilelab.plainshelf-reader); it reads the .bookpkg directory directly and
// persists progress into the shared reading_progress.json this app also uses, so
// the position shows up in this library. macOS-only for now — there is no reader
// binary on other platforms.
func (a *DesktopApp) OpenReader(shelfID, bookID string) error {
	targetDir, err := a.resolveBookPackagePath(shelfID, bookID)
	if err != nil {
		return err
	}

	if runtime.GOOS != "darwin" {
		return util.NewError("opening the standalone reader is only supported on macOS")
	}

	name, args := readerLaunchCommand(targetDir)
	if err := exec.Command(name, args...).Run(); err != nil {
		return util.Errorf("launching PlainShelfReader: %w", err)
	}
	return nil
}

// readerLaunchCommand builds the macOS command that opens the standalone reader
// at bookPath. It launches the app by name so LaunchServices finds the installed
// PlainShelfReader.app and gives it its own window; PLAINSHELF_READER_APP
// overrides the app (a path or name) for development against a local build.
func readerLaunchCommand(bookPath string) (string, []string) {
	app := strings.TrimSpace(os.Getenv("PLAINSHELF_READER_APP"))
	if app == "" {
		app = "PlainShelfReader"
	}
	return "open", []string{"-a", app, "--args", "-book", bookPath}
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
	a.app = app
	return nil
}
