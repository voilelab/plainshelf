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

// ImportBookFromLocalPath imports one book the desktop client picked from disk
// and reports the outcome as a single result. The frontend calls it once per
// selected file so it can drive the same N/M progress and file-boundary abort
// the web upload path uses; a per-file import failure is reported through the
// result's Error field, not a Go error, so one bad file does not abort the batch
// the frontend is stepping through.
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

// shelfLibRoot returns the configured lib_root of the shelf with the given id,
// or an error when the id is empty or unknown.
func (a *DesktopApp) shelfLibRoot(shelfID string) (string, error) {
	shelfID = strings.TrimSpace(shelfID)
	if shelfID == "" {
		return "", util.Errorf("shelf ID cannot be empty")
	}

	conf, err := loadDesktopShelves(a.shelvesConfigPath)
	if err != nil {
		return "", util.Errorf("loading shelf config: %w", err)
	}

	for _, entry := range conf.Shelves {
		if entry.ID == shelfID {
			return entry.LibRoot, nil
		}
	}
	return "", util.Errorf("shelf with ID %q not found", shelfID)
}

// OpenShelfInFinder reveals a shelf's lib_root in the host file explorer. It is
// the shelf-level counterpart to OpenFolderDirectory/OpenBookDirectory, so a
// first-time user can find where their books actually live on disk without
// guessing at the config directory. OpenShelfDirectory (no arguments) is the
// unrelated directory *picker* used when adding a shelf.
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

	// normalizeFolderParts trims user-provided segments and drops empty entries;
	// resolveDesktopFolderPath then enforces that the final path stays under
	// <shelf>/books.
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

// resolveBookPackagePath returns the absolute path of a book's .bookpkg
// directory, verified to exist and stay within its shelf. It is shared by
// "reveal in Finder" and "open in the standalone reader".
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

// readerUnsupportedPlatformCode is a stable token embedded in the OpenReader
// error returned on non-macOS platforms, where no standalone reader binary
// exists at all. The frontend matches this token to tell that case apart from a
// genuine macOS launch failure (reader not installed, or the launch itself
// failed) and word its in-app fallback notice accordingly. It survives the
// util.NewError function-name prefix, so a substring match stays reliable. Keep
// in sync with READER_UNSUPPORTED_PLATFORM_CODE in frontend/src/api/desktop.ts.
const readerUnsupportedPlatformCode = "reader_unsupported_platform"

// OpenReader launches the standalone PlainShelfReader in its own window, showing
// the given book. It is the desktop's reading path: the frontend routes the read
// action here on desktop instead of opening the in-app reader.
//
// The reader is a separate macOS app (installed via the Homebrew cask, bundle id
// com.voilelab.plainshelf-reader); it reads the .bookpkg directory directly and
// persists progress into the shared reading_progress.json this app also uses, so
// the position shows up in this library. macOS-only for now — there is no reader
// binary on other platforms.
//
// section is the reader section index to open at, or negative to open at the
// book's restored progress. The frontend passes the chapter it was launched from
// (a chapter "read" action) so the standalone reader lands on that chapter.
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

// readerLaunchCommand builds the macOS command that opens the standalone reader
// at bookPath. It launches the app by name so LaunchServices finds the installed
// PlainShelfReader.app and gives it its own window; PLAINSHELF_READER_APP
// overrides the app (a path or name) for development against a local build.
//
// -shelf passes the book's real shelf id so the reader reports it as the active
// shelf and reads/writes progress under shelves.<shelfID>.<bookID> — the same key
// the desktop library uses — instead of the reader's synthetic shelf. It is
// omitted when shelfID is empty so a launch with no real shelf still falls back
// to the synthetic one.
//
// -section passes the reader section index to open at, so a chapter "read" action
// lands the standalone reader on that chapter rather than the restored progress.
// It is omitted when section is negative (no chapter requested), which opens the
// book at its restored progress like a default read.
//
// -n forces a new instance: the reader takes its book only from its startup
// arguments and shows a single book, so without it a second launch while a
// reader is already open would merely reactivate the first window (still showing
// the first book) and report success, so the frontend would not fall back.
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
