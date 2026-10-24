package mobile

import (
	"archive/zip"
	"context"
	"encoding/json/v2"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/voilelab/plainshelf/internal/shelfzip"
	"github.com/voilelab/plainshelf/internal/taskutil"
	"github.com/voilelab/plainshelf/shelf"
)

func startT(t *testing.T, dataDir string) {
	t.Helper()

	t.Setenv("TMPDIR", os.Getenv("TMPDIR"))
	if err := Start(dataDir); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { _ = Stop() })
	waitReady(t)
}

func waitReady(t *testing.T) {
	t.Helper()
	rt, err := running()
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range rt.app.ShelfManager().GetAllShelves() {
		if err := s.WaitReady(t.Context()); err != nil {
			t.Fatalf("WaitReady %s: %v", s.ID, err)
		}
	}
}

type result struct {
	Status int
	Header map[string]string
	Body   []byte
}

func handle(t *testing.T, method, url, body string) *result {
	t.Helper()
	dir := t.TempDir()
	bodyPath := ""
	if body != "" {
		bodyPath = filepath.Join(dir, "body")
		if err := os.WriteFile(bodyPath, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return handleFile(t, method, url, `{"Content-Type":"application/json"}`, bodyPath, filepath.Join(dir, "out"))
}

func handleFile(t *testing.T, method, url, headerJSON, bodyPath, outPath string) *result {
	t.Helper()
	res, err := Handle(method, url, headerJSON, bodyPath, outPath)
	if err != nil {
		t.Fatalf("Handle %s %s: %v", method, url, err)
	}
	var hdr map[string]string
	if err := json.Unmarshal([]byte(res.Header), &hdr); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	return &result{Status: res.Status, Header: hdr, Body: data}
}

func createShelf(t *testing.T, name string) string {
	t.Helper()
	id, err := CreateShelf(name)
	if err != nil {
		t.Fatalf("CreateShelf: %v", err)
	}
	waitReady(t)
	return id
}

func createBook(t *testing.T, shelfID, title string) {
	t.Helper()
	res := handle(t, http.MethodPost, "/api/shelves/"+shelfID+"/books", `{"title":"`+title+`"}`)
	if res.Status/100 != 2 {
		t.Fatalf("create book: %d %s", res.Status, res.Body)
	}
}

func bookTitles(t *testing.T, shelfID string) string {
	t.Helper()
	res := handle(t, http.MethodGet, "/api/shelves/"+shelfID+"/books", "")
	if res.Status != http.StatusOK {
		t.Fatalf("list books: %d %s", res.Status, res.Body)
	}
	return string(res.Body)
}

func listShelves(t *testing.T) []shelfEntry {
	t.Helper()
	out, err := ListShelves()
	if err != nil {
		t.Fatalf("ListShelves: %v", err)
	}
	var entries []shelfEntry
	if err := json.Unmarshal([]byte(out), &entries); err != nil {
		t.Fatal(err)
	}
	return entries
}

func TestNotStarted(t *testing.T) {
	if _, err := Handle(http.MethodGet, "/health", "", "", filepath.Join(t.TempDir(), "out")); !errors.Is(err, ErrNotStarted) {
		t.Fatalf("Handle before Start: %v", err)
	}
	if _, err := CreateShelf("x"); !errors.Is(err, ErrNotStarted) {
		t.Fatalf("CreateShelf before Start: %v", err)
	}
}

func TestShelfAndBooksSurviveRestart(t *testing.T) {
	dataDir := t.TempDir()
	startT(t, dataDir)
	id := createShelf(t, "Phone")
	createBook(t, id, "Kept Title")

	if err := Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	startT(t, dataDir)

	if got := listShelves(t); len(got) != 1 || got[0].ID != id || got[0].Name != "Phone" {
		t.Fatalf("shelves after restart: %+v", got)
	}
	if body := bookTitles(t, id); !strings.Contains(body, "Kept Title") {
		t.Fatalf("book missing after restart: %s", body)
	}
	if _, err := os.Stat(filepath.Join(dataDir, shelvesDir, id, "books")); err != nil {
		t.Fatalf("shelf is not under the data directory: %v", err)
	}
}

func TestHandlerHasNoShelfManagementRoutes(t *testing.T) {
	startT(t, t.TempDir())
	id := createShelf(t, "Phone")

	for _, tc := range []struct{ method, url string }{
		{http.MethodPost, "/api/shelves"},
		{http.MethodPut, "/api/shelves/" + id},
		{http.MethodPatch, "/api/shelves/" + id},
		{http.MethodDelete, "/api/shelves/" + id},
	} {
		res := handle(t, tc.method, tc.url, `{"name":"x"}`)
		if res.Status != http.StatusNotFound && res.Status != http.StatusMethodNotAllowed {
			t.Errorf("%s %s answered %d", tc.method, tc.url, res.Status)
		}
	}
	if got := listShelves(t); len(got) != 1 {
		t.Fatalf("shelves changed: %+v", got)
	}
}

func TestRenameKeepsID(t *testing.T) {
	startT(t, t.TempDir())
	id := createShelf(t, "Old")
	createBook(t, id, "Stays")
	if err := RenameShelf(id, "New"); err != nil {
		t.Fatalf("RenameShelf: %v", err)
	}
	waitReady(t)
	if got := listShelves(t); got[0].ID != id || got[0].Name != "New" {
		t.Fatalf("after rename: %+v", got)
	}
	if body := bookTitles(t, id); !strings.Contains(body, "Stays") {
		t.Fatalf("book missing after rename: %s", body)
	}
}

func TestRemoveDeletesTheShelf(t *testing.T) {
	dataDir := t.TempDir()
	startT(t, dataDir)
	id := createShelf(t, "Gone")
	if err := RemoveShelf(id); err != nil {
		t.Fatalf("RemoveShelf: %v", err)
	}
	if got := listShelves(t); len(got) != 0 {
		t.Fatalf("shelves after remove: %+v", got)
	}
	if _, err := os.Stat(libRootOf(dataDir, id)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("shelf directory left behind: %v", err)
	}
	if res := handle(t, http.MethodGet, "/api/shelves/"+id+"/books", ""); res.Status != http.StatusNotFound {
		t.Fatalf("removed shelf still served: %d", res.Status)
	}
}

func TestExportOpensAsNativeShelfAndImportsBack(t *testing.T) {
	dataDir := t.TempDir()
	startT(t, dataDir)
	id := createShelf(t, "Phone")
	createBook(t, id, "Round Trip")
	if res := handle(t, http.MethodPost, "/api/shelves/"+id+"/books", `{"title":"Kept Apart","folder":["Adult"]}`); res.Status/100 != 2 {
		t.Fatalf("create book: %d %s", res.Status, res.Body)
	}

	shelfJSON := `{"schema_version":1,"content":{"nsfw_folders":[{"path":"Adult"}]}}`
	if err := os.WriteFile(filepath.Join(libRootOf(dataDir, id), "shelf.json"), []byte(shelfJSON), 0o644); err != nil {
		t.Fatal(err)
	}

	zipPath := filepath.Join(t.TempDir(), "shelf.zip")
	if err := ExportShelf(id, zipPath); err != nil {
		t.Fatalf("ExportShelf: %v", err)
	}

	if got := listShelves(t); got[0].LastExport != "" {
		t.Fatalf("last export recorded before MarkExported: %q", got[0].LastExport)
	}
	if err := MarkExported(id); err != nil {
		t.Fatalf("MarkExported: %v", err)
	}
	if got := listShelves(t); got[0].LastExport == "" {
		t.Fatal("last export time not recorded")
	}
	if err := MarkExported("local-missing"); err == nil {
		t.Fatal("MarkExported accepted an unknown shelf")
	}

	libRoot := t.TempDir()
	unzip(t, zipPath, libRoot)
	s, err := shelf.NewShelf(&shelf.ShelfConf{LibRoot: libRoot, LockMode: "none"})
	if err != nil {
		t.Fatalf("NewShelf: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	if err := s.WaitReady(t.Context()); err != nil {
		t.Fatal(err)
	}
	books, err := s.ListBooks()
	if err != nil || len(books) != 2 {
		t.Fatalf("native shelf books: %v %v", books, err)
	}

	newID, err := ImportShelf("Restored", zipPath)
	if err != nil {
		t.Fatalf("ImportShelf: %v", err)
	}
	waitReady(t)
	if newID == id {
		t.Fatal("import reused the exported shelf's ID")
	}
	if body := bookTitles(t, newID); !strings.Contains(body, "Round Trip") || strings.Contains(body, "Kept Apart") {
		t.Fatalf("imported shelf with adult content hidden: %s", body)
	}

	if res := handle(t, http.MethodPost, "/api/setting/show_nsfw", "true"); res.Status != http.StatusNoContent {
		t.Fatalf("show_nsfw: %d %s", res.Status, res.Body)
	}
	if body := bookTitles(t, newID); !strings.Contains(body, "Kept Apart") {
		t.Fatalf("imported shelf with adult content shown: %s", body)
	}
}

func TestImportRejectsNonShelfZip(t *testing.T) {
	dataDir := t.TempDir()
	startT(t, dataDir)
	id := createShelf(t, "Existing")
	createBook(t, id, "Untouched")

	zipPath := filepath.Join(t.TempDir(), "photos.zip")
	f, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	w, _ := zw.Create("photos/cat.jpg")
	_, _ = w.Write([]byte("meow"))
	_ = zw.Close()
	_ = f.Close()

	if _, err := ImportShelf("Photos", zipPath); !errors.Is(err, shelfzip.ErrNotShelf) {
		t.Fatalf("ImportShelf: %v", err)
	}
	if got := listShelves(t); len(got) != 1 || got[0].ID != id {
		t.Fatalf("shelves after rejected import: %+v", got)
	}
	entries, err := os.ReadDir(filepath.Join(dataDir, shelvesDir))
	if err != nil || len(entries) != 1 {
		t.Fatalf("shelf directories after rejected import: %v %v", entries, err)
	}
	if body := bookTitles(t, id); !strings.Contains(body, "Untouched") {
		t.Fatalf("existing shelf changed: %s", body)
	}
}

func TestImportRejectsOversizedZip(t *testing.T) {
	dataDir := t.TempDir()
	startT(t, dataDir)
	zipPath := filepath.Join(t.TempDir(), "huge.zip")
	f, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}

	if err := f.Truncate(MaxImportBytes() + 1); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()

	if _, err := ImportShelf("Huge", zipPath); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("ImportShelf: %v", err)
	}
	if entries, err := os.ReadDir(filepath.Join(dataDir, shelvesDir)); err != nil || len(entries) != 0 {
		t.Fatalf("shelf directories after rejected import: %v %v", entries, err)
	}
}

func unzip(t *testing.T, zipPath, dst string) {
	t.Helper()
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	for _, f := range zr.File {
		p := filepath.Join(dst, filepath.FromSlash(f.Name))
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(p, 0o755); err != nil {
				t.Fatal(err)
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		bs, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, bs, 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestStaleInstanceAfterStop(t *testing.T) {
	startT(t, t.TempDir())
	rt, err := running()
	if err != nil {
		t.Fatal(err)
	}
	if err := Stop(); err != nil {
		t.Fatal(err)
	}
	if err := rt.rlockRunning(); !errors.Is(err, ErrNotStarted) {
		t.Fatalf("rlock after Stop: %v", err)
	}
	if err := rt.lockRunning(); !errors.Is(err, ErrNotStarted) {
		t.Fatalf("lock after Stop: %v", err)
	}
}

type heldTask struct {
	release chan struct{}
	done    atomic.Bool
}

func (h *heldTask) Run(context.Context) error { <-h.release; h.done.Store(true); return nil }
func (h *heldTask) Name() string              { return "held" }
func (h *heldTask) Title() string             { return "held" }
func (h *heldTask) Description() string       { return "held" }
func (h *heldTask) Percentage() float64       { return 0 }
func (h *heldTask) Status() taskutil.Status {
	if h.done.Load() {
		return taskutil.StatusCompleted
	}
	return taskutil.StatusRunning
}

func TestExportImportAndRemoveWaitForTasks(t *testing.T) {
	dataDir := t.TempDir()
	startT(t, dataDir)
	id := createShelf(t, "Busy")
	createBook(t, id, "Backed Up")
	rt, err := running()
	if err != nil {
		t.Fatal(err)
	}
	old := taskWaitTimeout
	taskWaitTimeout = 50 * time.Millisecond
	t.Cleanup(func() { taskWaitTimeout = old })

	task := &heldTask{release: make(chan struct{})}
	release := sync.OnceFunc(func() { close(task.release) })

	t.Cleanup(release)
	if _, err := rt.app.TaskChains().Submit(&taskutil.TaskChain{Tasks: []taskutil.Task{task}}); err != nil {
		t.Fatal(err)
	}
	zipPath := filepath.Join(t.TempDir(), "shelf.zip")
	if err := ExportShelf(id, zipPath); !errors.Is(err, ErrBusy) {
		t.Fatalf("ExportShelf during a task: %v", err)
	}
	if _, err := os.Stat(zipPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("busy export left a zip: %v", err)
	}
	if _, err := ImportShelf("Restored", zipPath); !errors.Is(err, ErrBusy) {
		t.Fatalf("ImportShelf during a task: %v", err)
	}
	if err := RemoveShelf(id); !errors.Is(err, ErrBusy) {
		t.Fatalf("RemoveShelf during a task: %v", err)
	}
	if got := listShelves(t); len(got) != 1 {
		t.Fatalf("shelves changed while busy: %+v", got)
	}
	if entries, err := os.ReadDir(filepath.Join(dataDir, shelvesDir)); err != nil || len(entries) != 1 {
		t.Fatalf("shelf directories while busy: %v %v", entries, err)
	}

	taskWaitTimeout = 5 * time.Second
	release()
	if err := ExportShelf(id, zipPath); err != nil {
		t.Fatalf("ExportShelf after the task: %v", err)
	}
	if _, err := ImportShelf("Restored", zipPath); err != nil {
		t.Fatalf("ImportShelf after the task: %v", err)
	}
	if err := RemoveShelf(id); err != nil {
		t.Fatalf("RemoveShelf after the task: %v", err)
	}
}

func TestHandleStreamsBodiesThroughFiles(t *testing.T) {
	startT(t, t.TempDir())
	shelfID := createShelf(t, "Phone")
	res := handle(t, http.MethodPost, "/api/shelves/"+shelfID+"/books", `{"title":"Big"}`)
	var book struct {
		Meta struct {
			ID            string `json:"id"`
			CurrentSource string `json:"current_source"`
		} `json:"meta"`
	}
	if err := json.Unmarshal(res.Body, &book); err != nil || book.Meta.CurrentSource == "" {
		t.Fatalf("create book: %d %s %v", res.Status, res.Body, err)
	}
	contentURL := "/api/shelves/" + shelfID + "/books/" + book.Meta.ID + "/sources/" + book.Meta.CurrentSource + "/content"

	dir := t.TempDir()
	want := []byte(strings.Repeat("一行文字 line of text\n", 8<<20/32))
	bodyPath := filepath.Join(dir, "body")
	if err := os.WriteFile(bodyPath, want, 0o600); err != nil {
		t.Fatal(err)
	}
	res = handleFile(t, http.MethodPatch, contentURL, `{"Content-Type":"text/plain; charset=utf-8"}`, bodyPath, filepath.Join(dir, "patch-out"))
	if res.Status != http.StatusNoContent {
		t.Fatalf("PATCH content: %d %s", res.Status, res.Body)
	}
	res = handleFile(t, http.MethodGet, contentURL, "", "", filepath.Join(dir, "get-out"))
	if res.Status != http.StatusOK || string(res.Body) != string(want) {
		t.Fatalf("GET content: %d, %d bytes, want %d", res.Status, len(res.Body), len(want))
	}
	if res.Header["Content-Type"] == "" {
		t.Fatalf("GET content: no Content-Type in %v", res.Header)
	}
}
