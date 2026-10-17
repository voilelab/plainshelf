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
	if err := Start(dataDir); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { _ = Stop() })
	waitReady(t)
}

// waitReady waits out the initial scan, which answers reads with 503.
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

// result is a Response with its body read back from the out file.
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

// Shelves are managed through this package only; the HTTP API has no route
// that creates or deletes one.
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
	startT(t, t.TempDir())
	id := createShelf(t, "Phone")
	createBook(t, id, "Round Trip")

	zipPath := filepath.Join(t.TempDir(), "shelf.zip")
	if err := ExportShelf(id, zipPath); err != nil {
		t.Fatalf("ExportShelf: %v", err)
	}
	if got := listShelves(t); got[0].LastExport == "" {
		t.Fatal("last export time not recorded")
	}

	// Unzipped, it is a lib_root the desktop app or server opens as is.
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
	if err != nil || len(books) != 1 {
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
	if body := bookTitles(t, newID); !strings.Contains(body, "Round Trip") {
		t.Fatalf("imported shelf: %s", body)
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

// unzip extracts every entry as a file manager would.
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

// An instance fetched just before Stop refuses to serve once it gets the lock.
func TestStaleInstanceAfterStop(t *testing.T) {
	startT(t, t.TempDir())
	rt, err := running()
	if err != nil {
		t.Fatal(err)
	}
	if err := Stop(); err != nil {
		t.Fatal(err)
	}
	if err := rt.rlock(); !errors.Is(err, ErrNotStarted) {
		t.Fatalf("rlock after Stop: %v", err)
	}
	if err := rt.lock(); !errors.Is(err, ErrNotStarted) {
		t.Fatalf("lock after Stop: %v", err)
	}
}

// heldTask runs until release is closed.
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

// Export and removal wait for task chains accepted earlier, which keep writing
// after their request returned.
func TestExportAndRemoveWaitForTasks(t *testing.T) {
	startT(t, t.TempDir())
	id := createShelf(t, "Busy")
	rt, err := running()
	if err != nil {
		t.Fatal(err)
	}
	old := quiesceTimeout
	quiesceTimeout = 50 * time.Millisecond
	t.Cleanup(func() { quiesceTimeout = old })

	task := &heldTask{release: make(chan struct{})}
	release := sync.OnceFunc(func() { close(task.release) })
	// Before Stop's cleanup, which would wait on the task forever.
	t.Cleanup(release)
	if _, err := rt.app.TaskChains().Submit(&taskutil.TaskChain{Tasks: []taskutil.Task{task}}); err != nil {
		t.Fatal(err)
	}
	zipPath := filepath.Join(t.TempDir(), "shelf.zip")
	if err := ExportShelf(id, zipPath); !errors.Is(err, ErrBusy) {
		t.Fatalf("ExportShelf during a task: %v", err)
	}
	if err := RemoveShelf(id); !errors.Is(err, ErrBusy) {
		t.Fatalf("RemoveShelf during a task: %v", err)
	}
	if got := listShelves(t); len(got) != 1 {
		t.Fatalf("shelf removed while busy: %+v", got)
	}

	quiesceTimeout = 5 * time.Second
	release()
	if err := ExportShelf(id, zipPath); err != nil {
		t.Fatalf("ExportShelf after the task: %v", err)
	}
	if err := RemoveShelf(id); err != nil {
		t.Fatalf("RemoveShelf after the task: %v", err)
	}
}

// A large body goes through Handle as files on both sides and comes back
// byte for byte.
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
