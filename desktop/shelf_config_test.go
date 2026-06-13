package main

import (
	"errors"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/voilelab/plainshelf/server"
	"github.com/voilelab/plainshelf/shelf"
)

func TestLoadOrCreateDesktopShelfConfigCreatesDefault(t *testing.T) {
	dataRoot := t.TempDir()
	configPath := filepath.Join(dataRoot, desktopShelfConfigFilename)

	shelves, err := loadOrCreateDesktopShelfConfig(configPath, defaultDesktopShelfConf(dataRoot))
	if err != nil {
		t.Fatalf("loadOrCreateDesktopShelfConfig: %v", err)
	}
	if len(shelves) != 1 {
		t.Fatalf("len(shelves) = %d, want 1", len(shelves))
	}
	if shelves[0].ID != "default_shelf" || shelves[0].LibRoot != filepath.Join(dataRoot, "shelf") {
		t.Fatalf("default shelf = %#v", shelves[0])
	}
	if _, err := os.Stat(configPath); err != nil {
		t.Fatalf("expected persisted config: %v", err)
	}
}

func TestDesktopAppAddShelfPersistsAndRegistersRuntimeShelf(t *testing.T) {
	dataRoot := t.TempDir()
	defaultConf := defaultDesktopShelfConf(dataRoot)
	app, err := server.NewApp(&server.AppConf{
		Shelves:   []*shelf.ShelfConfWithID{defaultConf},
		StorePath: filepath.Join(dataRoot, "store"),
	})
	if err != nil {
		t.Fatalf("NewApp: %v", err)
	}
	defer app.Close()

	desktopApp := &DesktopApp{
		app:               app,
		dataRoot:          dataRoot,
		shelvesConfigPath: filepath.Join(dataRoot, desktopShelfConfigFilename),
		desktopShelves:    []*shelf.ShelfConfWithID{defaultConf},
	}

	libRoot := filepath.Join(dataRoot, "Books")
	if err := os.MkdirAll(libRoot, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	created, err := desktopApp.AddShelf("My Books", libRoot)
	if err != nil {
		t.Fatalf("AddShelf: %v", err)
	}
	if created.ID != "my-books" || created.Name != "My Books" {
		t.Fatalf("created shelf = %#v", created)
	}

	loaded, err := loadDesktopShelfConfig(desktopApp.shelvesConfigPath)
	if err != nil {
		t.Fatalf("loadDesktopShelfConfig: %v", err)
	}
	if len(loaded) != 2 || loaded[1].ID != "my-books" || loaded[1].LibRoot != libRoot {
		t.Fatalf("loaded shelves = %#v", loaded)
	}

	rec := httptest.NewRecorder()
	app.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/api/shelves", nil))
	if rec.Code != 200 {
		t.Fatalf("GET /api/shelves status = %d, body: %s", rec.Code, rec.Body.String())
	}
	if body := rec.Body.String(); !strings.Contains(body, "my-books") {
		t.Fatalf("GET /api/shelves body = %s, want new shelf", body)
	}
}

func TestDesktopAppAddShelfValidation(t *testing.T) {
	dataRoot := t.TempDir()
	libRoot := filepath.Join(dataRoot, "existing")
	if err := os.MkdirAll(libRoot, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	app := &DesktopApp{
		dataRoot: dataRoot,
		desktopShelves: []*shelf.ShelfConfWithID{
			{ID: "existing", Name: "Existing", ShelfConf: shelf.ShelfConf{LibRoot: libRoot}},
		},
	}

	if _, err := app.newDesktopShelfConf(" ", filepath.Join(dataRoot, "other")); err == nil {
		t.Fatal("expected empty name error")
	}
	if _, err := app.newDesktopShelfConf("Missing", filepath.Join(dataRoot, "missing")); err == nil {
		t.Fatal("expected missing directory error")
	}
	if _, err := app.newDesktopShelfConf("Duplicate", libRoot); err == nil {
		t.Fatal("expected duplicate directory error")
	}

	otherRoot := filepath.Join(dataRoot, "other")
	if err := os.MkdirAll(otherRoot, 0o755); err != nil {
		t.Fatalf("MkdirAll other: %v", err)
	}
	conf, err := app.newDesktopShelfConf("Existing", otherRoot)
	if err != nil {
		t.Fatalf("newDesktopShelfConf: %v", err)
	}
	if conf.ID != "existing-2" {
		t.Fatalf("conf.ID = %q, want existing-2", conf.ID)
	}
}

func TestGetAPIHandlerServesBeforeStartup(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	app := NewDesktopApp()
	t.Cleanup(app.Shutdown)

	rec := httptest.NewRecorder()
	app.GetAPIHandler().ServeHTTP(rec, httptest.NewRequest("GET", "/api/shelves", nil))
	if rec.Code != 200 {
		t.Fatalf("GET /api/shelves status = %d, body: %s", rec.Code, rec.Body.String())
	}
	if body := rec.Body.String(); !strings.Contains(body, "default_shelf") {
		t.Fatalf("GET /api/shelves body = %s, want default shelf", body)
	}
}

func TestRecoverDesktopStoreOpenErrorBacksUpEmptyValueLog(t *testing.T) {
	dataRoot := t.TempDir()
	storePath := filepath.Join(dataRoot, "store")
	if err := os.MkdirAll(storePath, 0o755); err != nil {
		t.Fatalf("MkdirAll store: %v", err)
	}
	markerPath := filepath.Join(storePath, "marker")
	if err := os.WriteFile(markerPath, []byte("store data"), 0o644); err != nil {
		t.Fatalf("WriteFile marker: %v", err)
	}
	vlogPath := filepath.Join(storePath, "000001.vlog")
	if err := os.WriteFile(vlogPath, nil, 0o644); err != nil {
		t.Fatalf("WriteFile vlog: %v", err)
	}

	recovered, err := recoverDesktopStoreOpenError(dataRoot, errors.New(`server.NewApp: store.New: During db.vlog.open err: Open existing file: "`+vlogPath+`" err: Create a new file`))
	if err != nil {
		t.Fatalf("recoverDesktopStoreOpenError: %v", err)
	}
	if !recovered {
		t.Fatal("expected recovery")
	}
	if _, err := os.Stat(storePath); err != nil {
		t.Fatalf("store path missing after recovery: %v", err)
	}
	if _, err := os.Stat(markerPath); err != nil {
		t.Fatalf("marker path missing after recovery: %v", err)
	}
	if _, err := os.Stat(vlogPath); !os.IsNotExist(err) {
		t.Fatalf("vlog path still exists or stat failed unexpectedly: %v", err)
	}

	matches, err := filepath.Glob(vlogPath + ".recovered-*")
	if err != nil {
		t.Fatalf("Glob: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("backup value log matches = %v, want one", matches)
	}
}

func TestRecoverDesktopStoreOpenErrorIgnoresUnrelatedErrors(t *testing.T) {
	recovered, err := recoverDesktopStoreOpenError(t.TempDir(), errors.New("some other startup error"))
	if err != nil {
		t.Fatalf("recoverDesktopStoreOpenError: %v", err)
	}
	if recovered {
		t.Fatal("unexpected recovery for unrelated error")
	}
}
