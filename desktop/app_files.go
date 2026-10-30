package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/voilelab/plainshelf/internal/util"
	"github.com/voilelab/plainshelf/shelf"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type DesktopImportBookResult struct {
	Path  string `json:"path"`
	ID    string `json:"id,omitempty"`
	Error string `json:"error,omitempty"`
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
		return nil
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

func normalizeFolderParts(folderParts []string) shelf.FolderPath {
	normalizedParts := make(shelf.FolderPath, 0, len(folderParts))
	for _, part := range folderParts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		normalizedParts = append(normalizedParts, trimmed)
	}
	return normalizedParts
}

func (a *DesktopApp) ImportBookFromLocalPath(shelfID string, localPath string, folderParts []string) (DesktopImportBookResult, error) {
	if a.app == nil {
		return DesktopImportBookResult{}, util.NewError("desktop backend app instance is nil")
	}

	result := DesktopImportBookResult{Path: localPath}
	book, err := a.app.ImportFromLocalPath(shelfID, localPath, normalizeFolderParts(folderParts))
	if err != nil {
		result.Error = err.Error()
		return result, nil
	}
	result.ID = book.ID()
	return result, nil
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

func (a *DesktopApp) shelfLibRoot(shelfID string) (string, error) {
	shelfID = strings.TrimSpace(shelfID)
	if shelfID == "" {
		return "", util.Errorf("shelf ID cannot be empty")
	}

	conf, err := loadDesktopShelves(a.shelvesConfigPath)
	if err != nil {
		return "", util.Errorf("loading shelf config: %w", err)
	}

	entry := conf.find(shelfID)
	switch {
	case entry == nil:
		return "", util.Errorf("shelf with ID %q not found", shelfID)
	case entry.Source != nil:
		return "", util.Errorf("shelf %q is read from %s and has no folder on this computer", shelfID, entry.Source.URL)
	}
	return entry.LibRoot, nil
}

func (a *DesktopApp) OpenShelfInFinder(shelfID string) error {
	libRoot, err := a.shelfLibRoot(shelfID)
	if err != nil {
		return util.Errorf("%w", err)
	}

	normalizedRoot, err := normalizeDesktopShelfDirectory(libRoot)
	if err != nil {
		return util.Errorf("%w", err)
	}

	info, err := os.Stat(normalizedRoot)
	if err != nil {
		return util.Errorf("shelf directory unavailable: %w", err)
	}
	if !info.IsDir() {
		return util.Errorf("shelf path is not a directory")
	}

	if err := openFinder(normalizedRoot); err != nil {
		return util.Errorf("%w", err)
	}

	return nil
}

func resolveDesktopFolderPath(libRoot string, folderParts []string) (string, error) {
	normalizedRoot, err := normalizeDesktopShelfDirectory(libRoot)
	if err != nil {
		return "", util.Errorf("%w", err)
	}

	booksRoot := filepath.Clean(filepath.Join(normalizedRoot, "books"))
	targetPathParts := append([]string{booksRoot}, folderParts...)
	targetDir := filepath.Clean(filepath.Join(targetPathParts...))

	relPath, err := filepath.Rel(booksRoot, targetDir)
	if err != nil {
		return "", util.Errorf("resolving folder directory: %w", err)
	}
	if !filepath.IsLocal(relPath) {
		return "", util.Errorf("invalid folder path")
	}

	return targetDir, nil
}

func (a *DesktopApp) OpenFolderDirectory(shelfID string, folderParts []string) error {
	libRoot, err := a.shelfLibRoot(shelfID)
	if err != nil {
		return util.Errorf("%w", err)
	}

	targetDir, err := resolveDesktopFolderPath(libRoot, normalizeFolderParts(folderParts))
	if err != nil {
		return util.Errorf("%w", err)
	}

	info, err := os.Stat(targetDir)
	if err != nil {
		return util.Errorf("folder directory unavailable: %w", err)
	}
	if !info.IsDir() {
		return util.Errorf("folder path is not a directory")
	}

	if err := openFinder(targetDir); err != nil {
		return util.Errorf("%w", err)
	}

	return nil
}

func (a *DesktopApp) resolveBookPackagePath(shelfID, bookID string) (string, error) {
	if a.app == nil {
		return "", util.NewError("desktop backend app instance is nil")
	}

	libRoot, err := a.shelfLibRoot(shelfID)
	if err != nil {
		return "", util.Errorf("%w", err)
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
	if !filepath.IsLocal(relPath) {
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

const readerUnsupportedPlatformCode = "reader_unsupported_platform"

func (a *DesktopApp) OpenReader(shelfID, bookID string, section int) error {
	targetDir, err := a.resolveBookPackagePath(shelfID, bookID)
	if err != nil {
		return err
	}

	if runtime.GOOS != "darwin" {
		return util.NewError(readerUnsupportedPlatformCode + ": opening the standalone reader is only supported on macOS")
	}

	name, args := readerLaunchCommand(targetDir, shelfID, section)
	if err := exec.Command(name, args...).Run(); err != nil {
		return util.Errorf("launching PlainShelfReader: %w", err)
	}
	return nil
}

// macOS LaunchServices open(1) needs -n to create a new reader process;
// otherwise it reactivates the existing app without applying new startup args.
func readerLaunchCommand(bookPath, shelfID string, section int) (string, []string) {
	app := strings.TrimSpace(os.Getenv("PLAINSHELF_READER_APP"))
	if app == "" {
		app = "PlainShelfReader"
	}
	readerArgs := []string{"-book", bookPath}
	if shelfID = strings.TrimSpace(shelfID); shelfID != "" {
		readerArgs = append(readerArgs, "-shelf", shelfID)
	}
	if section >= 0 {
		readerArgs = append(readerArgs, "-section", strconv.Itoa(section))
	}
	return "open", append([]string{"-n", "-a", app, "--args"}, readerArgs...)
}
