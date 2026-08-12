package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/voilelab/plainshelf/server"
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

func TestNormalizeLayerParts(t *testing.T) {
	parts := normalizeLayerParts([]string{"", "  ", " fiction ", " sci-fi "})
	if len(parts) != 2 {
		t.Fatalf("expected two valid layer parts, got %d", len(parts))
	}
	if parts[0] != "fiction" {
		t.Fatalf("unexpected first part: %q", parts[0])
	}
	if parts[1] != "sci-fi" {
		t.Fatalf("unexpected second part: %q", parts[1])
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
	if err := desktopApp.AddShelf("Broken Shelf", badShelfPath, "10m"); err == nil {
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

func TestResolveDesktopLayerDirectory(t *testing.T) {
	libRoot := filepath.Join(t.TempDir(), "shelf")

	path, err := resolveDesktopLayerDirectory(libRoot, []string{"fiction", "sci-fi"})
	if err != nil {
		t.Fatalf("resolveDesktopLayerDirectory returned error: %v", err)
	}
	wantPath := filepath.Join(libRoot, "books", "fiction", "sci-fi")
	if path != wantPath {
		t.Fatalf("resolved path = %q, want %q", path, wantPath)
	}

	rootPath, err := resolveDesktopLayerDirectory(libRoot, nil)
	if err != nil {
		t.Fatalf("resolve root layer directory returned error: %v", err)
	}
	wantRootPath := filepath.Join(libRoot, "books")
	if rootPath != wantRootPath {
		t.Fatalf("resolved root path = %q, want %q", rootPath, wantRootPath)
	}
}

func TestResolveDesktopLayerDirectoryRejectsTraversal(t *testing.T) {
	libRoot := filepath.Join(t.TempDir(), "shelf")

	if _, err := resolveDesktopLayerDirectory(libRoot, []string{"..", "outside"}); err == nil {
		t.Fatal("expected traversal layer path to fail, got nil")
	}
}

func TestOpenLayerDirectoryOpensFinderForLayerPath(t *testing.T) {
	tempDir := t.TempDir()
	libRoot := filepath.Join(tempDir, "library")
	layerDir := filepath.Join(libRoot, "books", "fiction", "sci-fi")
	if err := os.MkdirAll(layerDir, 0o755); err != nil {
		t.Fatalf("create layer dir: %v", err)
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

	if err := app.OpenLayerDirectory(" shelf-1 ", []string{" fiction ", "sci-fi "}); err != nil {
		t.Fatalf("OpenLayerDirectory returned error: %v", err)
	}
	if openedPath != layerDir {
		t.Fatalf("openFinder path = %q, want %q", openedPath, layerDir)
	}
}

// The read history and the reading stats are two device documents over one
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
		readHistoryPath:  filepath.Join(dir, "read_history.json"),
		readingStatsPath: filepath.Join(dir, "reading_stats.json"),
	}
	history := `{"version":1,"limit":100,"shelves":{"main":["book-1"]}}`
	stats := `{"version":1,"shelves":{"main":{"2026-08-02":45}}}`

	if err := app.WriteReadHistory(history); err != nil {
		t.Fatalf("WriteReadHistory: %v", err)
	}
	if err := app.WriteReadingStats(stats); err != nil {
		t.Fatalf("WriteReadingStats: %v", err)
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
	if gotStats != stats {
		t.Fatalf("ReadReadingStats = %q, want %q", gotStats, stats)
	}
}
