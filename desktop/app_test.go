package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/voilelab/plainshelf/server"
	"github.com/voilelab/plainshelf/shelf"
)

func TestBookOpenDialogOptions(t *testing.T) {
	options := bookOpenDialogOptions()
	if len(options.Filters) != 1 {
		t.Fatalf("expected exactly one file filter, got %d", len(options.Filters))
	}

	filter := options.Filters[0]
	if filter.Pattern != "*.txt;*.md;*.epub" {
		t.Fatalf("expected txt+md+epub filter pattern, got %q", filter.Pattern)
	}
}

func TestNormalizeSelectedLocalPaths(t *testing.T) {
	paths := normalizeSelectedLocalPaths([]string{"", "  ", " /tmp/book-1.txt ", "/tmp/book-2.txt"})
	if len(paths) != 2 {
		t.Fatalf("expected two valid paths, got %d", len(paths))
	}
	if paths[0] != "/tmp/book-1.txt" {
		t.Fatalf("unexpected first path: %q", paths[0])
	}
	if paths[1] != "/tmp/book-2.txt" {
		t.Fatalf("unexpected second path: %q", paths[1])
	}
}

func TestNormalizeFolderParts(t *testing.T) {
	parts := normalizeFolderParts([]string{"", "  ", " fiction ", " sci-fi "})
	if len(parts) != 2 {
		t.Fatalf("expected two valid folder parts, got %d", len(parts))
	}
	if parts[0] != "fiction" {
		t.Fatalf("unexpected first part: %q", parts[0])
	}
	if parts[1] != "sci-fi" {
		t.Fatalf("unexpected second part: %q", parts[1])
	}
}

// startedDesktopAppWithShelf builds a started DesktopApp backed by a real server
// app with one ready shelf, for the tests that exercise ImportBookFromLocalPath
// end to end.
func startedDesktopAppWithShelf(t *testing.T, shelfID string) *DesktopApp {
	t.Helper()

	serverApp, err := server.NewApp(&server.AppConf{
		Shelves: []*shelf.ShelfConfWithID{
			{
				ID: shelfID,
				ShelfConf: shelf.ShelfConf{
					LibRoot: t.TempDir(),
				},
			},
		},
		StorePath: t.TempDir(),
		Security:  &server.SecurityConf{Mode: server.SecurityModeNone},
	})
	if err != nil {
		t.Fatalf("NewApp: %v", err)
	}
	t.Cleanup(func() {
		if err := serverApp.Close(); err != nil {
			t.Fatalf("Close app: %v", err)
		}
	})

	if err := serverApp.Start(); err != nil {
		t.Fatalf("Start app: %v", err)
	}
	for _, shelfData := range serverApp.ShelfManager().GetAllShelves() {
		if err := shelfData.WaitReady(t.Context()); err != nil {
			t.Fatalf("WaitReady for shelf %s: %v", shelfData.ID, err)
		}
	}

	return &DesktopApp{app: serverApp}
}

func TestImportBookFromLocalPathReturnsBookIDOnSuccess(t *testing.T) {
	const shelfID = "default_shelf"
	desktopApp := startedDesktopAppWithShelf(t, shelfID)

	srcPath := filepath.Join(t.TempDir(), "example-book.txt")
	if err := os.WriteFile(srcPath, []byte("Chapter one\n\nHello world.\n"), 0o600); err != nil {
		t.Fatalf("write source book: %v", err)
	}

	result, err := desktopApp.ImportBookFromLocalPath(shelfID, srcPath, nil)
	if err != nil {
		t.Fatalf("ImportBookFromLocalPath returned error: %v", err)
	}
	if result.Path != srcPath {
		t.Fatalf("result path = %q, want %q", result.Path, srcPath)
	}
	if result.ID == "" {
		t.Fatalf("expected a book ID on success, got empty (error=%q)", result.Error)
	}
	if result.Error != "" {
		t.Fatalf("expected no error on success, got %q", result.Error)
	}
}

func TestImportBookFromLocalPathReportsFailureInResult(t *testing.T) {
	const shelfID = "default_shelf"
	desktopApp := startedDesktopAppWithShelf(t, shelfID)

	// A missing source path must fail through the result's Error field, not a Go
	// error, so the frontend's per-file loop keeps stepping through the batch.
	missingPath := filepath.Join(t.TempDir(), "does-not-exist.txt")
	result, err := desktopApp.ImportBookFromLocalPath(shelfID, missingPath, nil)
	if err != nil {
		t.Fatalf("ImportBookFromLocalPath returned a Go error for a bad path: %v", err)
	}
	if result.Path != missingPath {
		t.Fatalf("result path = %q, want %q", result.Path, missingPath)
	}
	if result.ID != "" {
		t.Fatalf("expected no book ID on failure, got %q", result.ID)
	}
	if result.Error == "" {
		t.Fatal("expected an error message on failure, got empty")
	}
}

func TestSaveAndLoadDesktopShelves(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "shelves.json")

	// Loading a missing config returns empty config without error.
	conf, err := loadDesktopShelves(configPath)
	if err != nil {
		t.Fatalf("loadDesktopShelves on missing file: %v", err)
	}
	if len(conf.Shelves) != 0 {
		t.Fatalf("expected empty shelves, got %d", len(conf.Shelves))
	}

	conf.Shelves = []desktopShelfEntry{
		{ID: "my-books", Name: "My Books", LibRoot: "/home/user/books", ScanInterval: "10m"},
		{ID: "work", Name: "Work", LibRoot: "/home/user/work"},
	}
	if err := saveDesktopShelves(configPath, conf); err != nil {
		t.Fatalf("saveDesktopShelves: %v", err)
	}

	loaded, err := loadDesktopShelves(configPath)
	if err != nil {
		t.Fatalf("loadDesktopShelves after save: %v", err)
	}
	if len(loaded.Shelves) != 2 {
		t.Fatalf("expected 2 shelves, got %d", len(loaded.Shelves))
	}
	if loaded.Shelves[0].ID != "my-books" || loaded.Shelves[0].Name != "My Books" || loaded.Shelves[0].LibRoot != "/home/user/books" || loaded.Shelves[0].ScanInterval != "10m" {
		t.Fatalf("unexpected first shelf: %+v", loaded.Shelves[0])
	}
	if loaded.Shelves[1].ID != "work" {
		t.Fatalf("unexpected second shelf ID: %q", loaded.Shelves[1].ID)
	}

	// Verify file permissions are restricted.
	info, err := os.Stat(configPath)
	if err != nil {
		t.Fatalf("stat config file: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("config file permissions = %o, want 0600", perm)
	}
}

func TestDesktopShelfEntryIncludesScanInterval(t *testing.T) {
	entry := desktopShelfEntry{
		ID:           "network-books",
		Name:         "Network Books",
		LibRoot:      "/mnt/books",
		ScanInterval: "10m",
	}

	conf := toShelfConfWithID(entry)
	if conf.ScanInterval != "10m" {
		t.Fatalf("scan interval = %q, want %q", conf.ScanInterval, "10m")
	}
}

func TestDesktopShelfEntryIncludesReadOnly(t *testing.T) {
	entry := desktopShelfEntry{
		ID:       "archive",
		Name:     "Archive",
		LibRoot:  "/mnt/archive",
		ReadOnly: true,
	}

	conf := toShelfConfWithID(entry)
	if !conf.ReadOnly {
		t.Fatal("read_only was not carried into the shelf configuration")
	}
}

// The shelf setting that opens a shelf without writing to it has to be
// reachable from the desktop UI, and reversible from it: shelves.json lives in
// the desktop data directory, outside every shelf, so a shelf's own read_only
// never governs whether its settings can be edited. See DesktopApp.ModifyShelf.
func TestModifyShelfTogglesReadOnlyBothWays(t *testing.T) {
	const shelfID = "archive"
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "shelves.json")
	libRoot := filepath.Join(tempDir, "archive-shelf")
	if err := os.MkdirAll(libRoot, 0o755); err != nil {
		t.Fatalf("MkdirAll(libRoot): %v", err)
	}

	serverApp, err := server.NewApp(&server.AppConf{
		StorePath: filepath.Join(tempDir, "store"),
		Security:  &server.SecurityConf{Mode: server.SecurityModeNone},
	})
	if err != nil {
		t.Fatalf("NewApp: %v", err)
	}
	t.Cleanup(func() { serverApp.Close() })

	desktopApp := &DesktopApp{app: serverApp, shelvesConfigPath: configPath}
	if err := desktopApp.AddShelf("Archive", libRoot, "", false); err != nil {
		t.Fatalf("AddShelf: %v", err)
	}

	if err := desktopApp.ModifyShelf(shelfID, "Archive", "", true); err != nil {
		t.Fatalf("ModifyShelf to read-only: %v", err)
	}
	assertShelfReadOnly(t, desktopApp, shelfID, true)

	// The modify form reads its initial state from here, so a shelf that could
	// be turned read-only but never showed the toggle as on would be a one-way
	// door in the UI even though the backend can still be told otherwise.
	details, err := desktopApp.GetShelfDetails(shelfID)
	if err != nil {
		t.Fatalf("GetShelfDetails: %v", err)
	}
	if !details.ReadOnly {
		t.Fatal("GetShelfDetails reported read_only = false for a read-only shelf")
	}

	if err := desktopApp.ModifyShelf(shelfID, "Archive", "", false); err != nil {
		t.Fatalf("ModifyShelf back to writable: %v", err)
	}
	assertShelfReadOnly(t, desktopApp, shelfID, false)

	details, err = desktopApp.GetShelfDetails(shelfID)
	if err != nil {
		t.Fatalf("GetShelfDetails after turning read_only off: %v", err)
	}
	if details.ReadOnly {
		t.Fatal("GetShelfDetails still reported read_only = true after it was turned off")
	}
}

// A read-only shelf is taken exactly as it is found: opening one must leave no
// app/ directory, no lock file and no exported book cache behind.
func TestAddShelfReadOnlyWritesNothingToTheShelf(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "shelves.json")
	libRoot := filepath.Join(tempDir, "archive-shelf")
	if err := os.MkdirAll(libRoot, 0o755); err != nil {
		t.Fatalf("MkdirAll(libRoot): %v", err)
	}

	serverApp, err := server.NewApp(&server.AppConf{
		StorePath: filepath.Join(tempDir, "store"),
		Security:  &server.SecurityConf{Mode: server.SecurityModeNone},
	})
	if err != nil {
		t.Fatalf("NewApp: %v", err)
	}
	t.Cleanup(func() { serverApp.Close() })

	desktopApp := &DesktopApp{app: serverApp, shelvesConfigPath: configPath}
	if err := desktopApp.AddShelf("Archive", libRoot, "", true); err != nil {
		t.Fatalf("AddShelf read-only: %v", err)
	}
	assertShelfReadOnly(t, desktopApp, "archive", true)

	entries, err := os.ReadDir(libRoot)
	if err != nil {
		t.Fatalf("ReadDir(libRoot): %v", err)
	}
	if len(entries) != 0 {
		names := make([]string, 0, len(entries))
		for _, entry := range entries {
			names = append(names, entry.Name())
		}
		t.Fatalf("a read-only shelf wrote into lib_root: %v", names)
	}

	conf, err := loadDesktopShelves(configPath)
	if err != nil {
		t.Fatalf("loadDesktopShelves: %v", err)
	}
	if len(conf.Shelves) != 1 || !conf.Shelves[0].ReadOnly {
		t.Fatalf("read_only was not persisted: %+v", conf.Shelves)
	}
}

func assertShelfReadOnly(t *testing.T, desktopApp *DesktopApp, shelfID string, want bool) {
	t.Helper()

	shelfData, ok := desktopApp.app.ShelfManager().GetShelf(shelfID)
	if !ok {
		t.Fatalf("GetShelf(%q) did not find the shelf", shelfID)
	}
	if err := shelfData.WaitReady(t.Context()); err != nil {
		t.Fatalf("WaitReady(%q): %v", shelfID, err)
	}
	if got := shelfData.ReadOnly(); got != want {
		t.Fatalf("shelf %q ReadOnly() = %v, want %v", shelfID, got, want)
	}
}

func TestNormalizeDesktopShelfDirectory(t *testing.T) {
	cases := []struct {
		input   string
		want    string
		wantErr bool
	}{
		{input: "  /home/user/books  ", want: "/home/user/books"},
		{input: "/abs/path", want: "/abs/path"},
		{input: "", wantErr: true},
		{input: "  ", wantErr: true},
		{input: "relative/path", wantErr: true},
	}
	for _, tc := range cases {
		got, err := normalizeDesktopShelfDirectory(tc.input)
		if tc.wantErr {
			if err == nil {
				t.Errorf("normalizeDesktopShelfDirectory(%q): want error, got nil", tc.input)
			}
			continue
		}
		if err != nil {
			t.Errorf("normalizeDesktopShelfDirectory(%q): unexpected error: %v", tc.input, err)
			continue
		}
		if got != tc.want {
			t.Errorf("normalizeDesktopShelfDirectory(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestGenerateDesktopShelfID(t *testing.T) {
	cases := []struct {
		name        string
		existingIDs map[string]bool
		want        string
	}{
		{name: "My Books", existingIDs: map[string]bool{}, want: "my-books"},
		{name: "  Hello World  ", existingIDs: map[string]bool{}, want: "hello-world"},
		{name: "My Books", existingIDs: map[string]bool{"my-books": true}, want: "my-books-2"},
		{name: "My Books", existingIDs: map[string]bool{"my-books": true, "my-books-2": true}, want: "my-books-3"},
		{name: "!!!###", existingIDs: map[string]bool{}, want: "shelf"},
		{name: "shelf", existingIDs: map[string]bool{"shelf": true}, want: "shelf-2"},
	}
	for _, tc := range cases {
		got := generateDesktopShelfID(tc.name, tc.existingIDs)
		if got != tc.want {
			t.Errorf("generateDesktopShelfID(%q, %v) = %q, want %q", tc.name, tc.existingIDs, got, tc.want)
		}
	}
}

func TestAddShelfDoesNotPersistWhenRegistrationFails(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "shelves.json")
	badShelfPath := filepath.Join(tempDir, "not-a-directory")
	if err := os.WriteFile(badShelfPath, []byte("not a shelf directory"), 0o600); err != nil {
		t.Fatalf("write bad shelf path: %v", err)
	}

	serverApp, err := server.NewApp(&server.AppConf{
		StorePath: filepath.Join(tempDir, "store"),
		Security:  &server.SecurityConf{Mode: server.SecurityModeNone},
	})
	if err != nil {
		t.Fatalf("NewApp: %v", err)
	}
	defer serverApp.Close()

	desktopApp := &DesktopApp{app: serverApp, shelvesConfigPath: configPath}
	if err := desktopApp.AddShelf("Broken Shelf", badShelfPath, "10m", false); err == nil {
		t.Fatal("AddShelf with regular file path: want error, got nil")
	}

	conf, err := loadDesktopShelves(configPath)
	if err != nil {
		t.Fatalf("loadDesktopShelves: %v", err)
	}
	if len(conf.Shelves) != 0 {
		t.Fatalf("expected failed shelf registration not to be persisted, got %+v", conf.Shelves)
	}
}

func TestLoadOrMigrateDesktopShelvesSeedsLegacyDefaultShelf(t *testing.T) {
	dataRoot := t.TempDir()
	configPath := filepath.Join(dataRoot, "shelves.json")

	conf, err := loadOrMigrateDesktopShelves(configPath, dataRoot)
	if err != nil {
		t.Fatalf("loadOrMigrateDesktopShelves: %v", err)
	}
	if len(conf.Shelves) != 1 {
		t.Fatalf("expected one migrated legacy shelf, got %d", len(conf.Shelves))
	}

	entry := conf.Shelves[0]
	if entry.ID != desktopLegacyDefaultShelfID {
		t.Fatalf("legacy shelf ID = %q, want %q", entry.ID, desktopLegacyDefaultShelfID)
	}
	if entry.Name != desktopLegacyDefaultShelfName {
		t.Fatalf("legacy shelf name = %q, want %q", entry.Name, desktopLegacyDefaultShelfName)
	}
	wantLibRoot := filepath.Join(dataRoot, desktopLegacyShelfDirName)
	if entry.LibRoot != wantLibRoot {
		t.Fatalf("legacy shelf lib root = %q, want %q", entry.LibRoot, wantLibRoot)
	}

	loaded, err := loadDesktopShelves(configPath)
	if err != nil {
		t.Fatalf("load migrated shelves config: %v", err)
	}
	if len(loaded.Shelves) != 1 || loaded.Shelves[0] != entry {
		t.Fatalf("persisted migrated config = %+v, want %+v", loaded.Shelves, conf.Shelves)
	}
}

func TestResolveDesktopFolderPath(t *testing.T) {
	libRoot := filepath.Join(t.TempDir(), "shelf")

	path, err := resolveDesktopFolderPath(libRoot, []string{"fiction", "sci-fi"})
	if err != nil {
		t.Fatalf("resolveDesktopFolderPath returned error: %v", err)
	}
	wantPath := filepath.Join(libRoot, "books", "fiction", "sci-fi")
	if path != wantPath {
		t.Fatalf("resolved path = %q, want %q", path, wantPath)
	}

	rootPath, err := resolveDesktopFolderPath(libRoot, nil)
	if err != nil {
		t.Fatalf("resolve root folder directory returned error: %v", err)
	}
	wantRootPath := filepath.Join(libRoot, "books")
	if rootPath != wantRootPath {
		t.Fatalf("resolved root path = %q, want %q", rootPath, wantRootPath)
	}
}

func TestResolveDesktopFolderPathRejectsTraversal(t *testing.T) {
	libRoot := filepath.Join(t.TempDir(), "shelf")

	if _, err := resolveDesktopFolderPath(libRoot, []string{"..", "outside"}); err == nil {
		t.Fatal("expected traversal folder path to fail, got nil")
	}
}

func TestOpenFolderDirectoryOpensFinderForFolderPath(t *testing.T) {
	tempDir := t.TempDir()
	libRoot := filepath.Join(tempDir, "library")
	folderDir := filepath.Join(libRoot, "books", "fiction", "sci-fi")
	if err := os.MkdirAll(folderDir, 0o755); err != nil {
		t.Fatalf("create folder dir: %v", err)
	}

	configPath := filepath.Join(tempDir, "shelves.json")
	conf := &desktopShelvesConfig{
		Shelves: []desktopShelfEntry{
			{ID: "shelf-1", Name: "Shelf", LibRoot: libRoot},
		},
	}
	if err := saveDesktopShelves(configPath, conf); err != nil {
		t.Fatalf("saveDesktopShelves: %v", err)
	}

	app := &DesktopApp{shelvesConfigPath: configPath}
	var openedPath string
	originalOpenFinder := openFinder
	openFinder = func(path string) error {
		openedPath = path
		return nil
	}
	t.Cleanup(func() {
		openFinder = originalOpenFinder
	})

	if err := app.OpenFolderDirectory(" shelf-1 ", []string{" fiction ", "sci-fi "}); err != nil {
		t.Fatalf("OpenFolderDirectory returned error: %v", err)
	}
	if openedPath != folderDir {
		t.Fatalf("openFinder path = %q, want %q", openedPath, folderDir)
	}
}

func TestOpenShelfInFinderOpensLibRoot(t *testing.T) {
	tempDir := t.TempDir()
	libRoot := filepath.Join(tempDir, "library")
	if err := os.MkdirAll(libRoot, 0o755); err != nil {
		t.Fatalf("create lib root: %v", err)
	}

	configPath := filepath.Join(tempDir, "shelves.json")
	conf := &desktopShelvesConfig{
		Shelves: []desktopShelfEntry{{ID: "shelf-1", Name: "Shelf", LibRoot: libRoot}},
	}
	if err := saveDesktopShelves(configPath, conf); err != nil {
		t.Fatalf("saveDesktopShelves: %v", err)
	}

	app := &DesktopApp{shelvesConfigPath: configPath}
	var openedPath string
	originalOpenFinder := openFinder
	openFinder = func(path string) error {
		openedPath = path
		return nil
	}
	t.Cleanup(func() {
		openFinder = originalOpenFinder
	})

	if err := app.OpenShelfInFinder(" shelf-1 "); err != nil {
		t.Fatalf("OpenShelfInFinder returned error: %v", err)
	}
	if openedPath != libRoot {
		t.Fatalf("openFinder path = %q, want %q", openedPath, libRoot)
	}
}

func TestOpenShelfInFinderRejectsUnknownShelf(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "shelves.json")
	if err := saveDesktopShelves(configPath, &desktopShelvesConfig{}); err != nil {
		t.Fatalf("saveDesktopShelves: %v", err)
	}

	app := &DesktopApp{shelvesConfigPath: configPath}
	if err := app.OpenShelfInFinder("missing"); err == nil {
		t.Fatal("OpenShelfInFinder for unknown shelf: want error, got nil")
	}
}

func TestPreviewShelfID(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "shelves.json")
	conf := &desktopShelvesConfig{
		Shelves: []desktopShelfEntry{{ID: "shelf", Name: "小說", LibRoot: filepath.Join(t.TempDir(), "novels")}},
	}
	if err := saveDesktopShelves(configPath, conf); err != nil {
		t.Fatalf("saveDesktopShelves: %v", err)
	}

	app := &DesktopApp{shelvesConfigPath: configPath}
	cases := []struct {
		name string
		want string
	}{
		{name: "", want: ""},
		{name: "   ", want: ""},
		{name: "My Books", want: "my-books"},
		// A purely non-ASCII name slugifies to nothing and falls back to
		// "shelf"; the seeded config already holds "shelf", so the next free id
		// is "shelf-2" — exactly what the user would silently receive.
		{name: "漫畫", want: "shelf-2"},
	}
	for _, tc := range cases {
		got, err := app.PreviewShelfID(tc.name)
		if err != nil {
			t.Fatalf("PreviewShelfID(%q) returned error: %v", tc.name, err)
		}
		if got != tc.want {
			t.Errorf("PreviewShelfID(%q) = %q, want %q", tc.name, got, tc.want)
		}
	}
}

// Reading history, progress, and stats are device documents over one
// pair of helpers (readDeviceDocument/writeDeviceDocument), so they are held to
// one set of expectations instead of two copies of the same test.
type deviceDocument struct {
	name        string
	fileName    string
	newApp      func(path string) *DesktopApp
	read        func(*DesktopApp) (string, error)
	write       func(*DesktopApp, string) error
	stored      string
	replacement string
	valid       string
}

func deviceDocuments() []deviceDocument {
	return []deviceDocument{
		{
			name:        "read history",
			fileName:    "read_history.json",
			newApp:      func(path string) *DesktopApp { return &DesktopApp{readHistoryPath: path} },
			read:        (*DesktopApp).ReadReadHistory,
			write:       (*DesktopApp).WriteReadHistory,
			stored:      `{"version":1,"limit":100,"shelves":{"main":["book-1","book-2"]}}`,
			replacement: `{"version":1,"limit":100,"shelves":{}}`,
			valid:       `{"version":1,"limit":100,"shelves":{}}`,
		},
		{
			name:        "reading progress",
			fileName:    "reading_progress.json",
			newApp:      func(path string) *DesktopApp { return &DesktopApp{readingProgressPath: path} },
			read:        (*DesktopApp).ReadReadingProgress,
			write:       (*DesktopApp).WriteReadingProgress,
			stored:      `{"version":1,"shelves":{"main":{"book-1":42}}}`,
			replacement: `{"version":1,"shelves":{}}`,
			valid:       `{"version":1,"shelves":{}}`,
		},
		{
			name:        "reading stats",
			fileName:    "reading_stats.json",
			newApp:      func(path string) *DesktopApp { return &DesktopApp{readingStatsPath: path} },
			read:        (*DesktopApp).ReadReadingStats,
			write:       (*DesktopApp).WriteReadingStats,
			stored:      `{"version":1,"shelves":{"main":{"2026-08-02":45}}}`,
			replacement: `{"version":1,"shelves":{}}`,
			valid:       `{"version":1,"shelves":{}}`,
		},
	}
}

func TestDeviceDocumentReturnsEmptyWhenUnwritten(t *testing.T) {
	for _, doc := range deviceDocuments() {
		t.Run(doc.name, func(t *testing.T) {
			app := doc.newApp(filepath.Join(t.TempDir(), doc.fileName))

			got, err := doc.read(app)
			if err != nil {
				t.Fatalf("read: %v", err)
			}
			if got != "" {
				t.Fatalf("read on a fresh profile = %q, want empty", got)
			}
		})
	}
}

func TestDeviceDocumentWriteAndReadRoundTrip(t *testing.T) {
	for _, doc := range deviceDocuments() {
		t.Run(doc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), doc.fileName)
			app := doc.newApp(path)

			if err := doc.write(app, doc.stored); err != nil {
				t.Fatalf("write: %v", err)
			}

			got, err := doc.read(app)
			if err != nil {
				t.Fatalf("read: %v", err)
			}
			if got != doc.stored {
				t.Fatalf("read = %q, want %q", got, doc.stored)
			}

			// A second write replaces the document rather than appending to it,
			// and leaves no temp file behind.
			if err := doc.write(app, doc.replacement); err != nil {
				t.Fatalf("write (replace): %v", err)
			}
			got, err = doc.read(app)
			if err != nil {
				t.Fatalf("read (replace): %v", err)
			}
			if got != doc.replacement {
				t.Fatalf("read after replace = %q, want %q", got, doc.replacement)
			}

			entries, err := os.ReadDir(filepath.Dir(path))
			if err != nil {
				t.Fatalf("ReadDir: %v", err)
			}
			if len(entries) != 1 || entries[0].Name() != filepath.Base(path) {
				t.Fatalf("unexpected files left in the data directory: %v", entries)
			}
		})
	}
}

func TestDeviceDocumentWriteRejectsInvalidDocuments(t *testing.T) {
	for _, doc := range deviceDocuments() {
		t.Run(doc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), doc.fileName)
			app := doc.newApp(path)
			if err := doc.write(app, doc.valid); err != nil {
				t.Fatalf("write: %v", err)
			}

			if err := doc.write(app, "not json"); err == nil {
				t.Fatal("write accepted a non-JSON document")
			}
			if err := doc.write(app, `{"pad":"`+strings.Repeat("x", maxDeviceDocumentBytes)+`"}`); err == nil {
				t.Fatal("write accepted an oversized document")
			}

			// A rejected write must not disturb the document already on disk.
			got, err := doc.read(app)
			if err != nil {
				t.Fatalf("read: %v", err)
			}
			if got != doc.valid {
				t.Fatalf("stored document after rejected writes = %q, want %q", got, doc.valid)
			}
		})
	}
}

func TestDeviceDocumentFailsWithoutStoragePath(t *testing.T) {
	for _, doc := range deviceDocuments() {
		t.Run(doc.name, func(t *testing.T) {
			app := &DesktopApp{}

			if _, err := doc.read(app); err == nil {
				t.Fatal("read succeeded before startup configured a path")
			}
			if err := doc.write(app, `{}`); err == nil {
				t.Fatal("write succeeded before startup configured a path")
			}
		})
	}
}

// The two documents are separate files: writing one must not disturb the other.
func TestDeviceDocumentsAreIndependent(t *testing.T) {
	dir := t.TempDir()
	app := &DesktopApp{
		readHistoryPath:     filepath.Join(dir, "read_history.json"),
		readingProgressPath: filepath.Join(dir, "reading_progress.json"),
		readingStatsPath:    filepath.Join(dir, "reading_stats.json"),
	}
	history := `{"version":1,"limit":100,"shelves":{"main":["book-1"]}}`
	progress := `{"version":1,"shelves":{"main":{"book-1":42}}}`
	stats := `{"version":1,"shelves":{"main":{"2026-08-02":45}}}`

	if err := app.WriteReadHistory(history); err != nil {
		t.Fatalf("WriteReadHistory: %v", err)
	}
	if err := app.WriteReadingStats(stats); err != nil {
		t.Fatalf("WriteReadingStats: %v", err)
	}
	if err := app.WriteReadingProgress(progress); err != nil {
		t.Fatalf("WriteReadingProgress: %v", err)
	}

	gotHistory, err := app.ReadReadHistory()
	if err != nil {
		t.Fatalf("ReadReadHistory: %v", err)
	}
	gotStats, err := app.ReadReadingStats()
	if err != nil {
		t.Fatalf("ReadReadingStats: %v", err)
	}
	if gotHistory != history {
		t.Fatalf("ReadReadHistory = %q, want %q", gotHistory, history)
	}
	gotProgress, err := app.ReadReadingProgress()
	if err != nil {
		t.Fatalf("ReadReadingProgress: %v", err)
	}
	if gotProgress != progress {
		t.Fatalf("ReadReadingProgress = %q, want %q", gotProgress, progress)
	}
	if gotStats != stats {
		t.Fatalf("ReadReadingStats = %q, want %q", gotStats, stats)
	}
}
