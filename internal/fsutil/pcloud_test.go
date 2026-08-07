package fsutil

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

func TestPCloudFSReadAndMetadata(t *testing.T) {
	api := newFakePCloudAPI(t)
	scopeID := api.addFolder(0, "scope")
	dirID := api.addFolder(scopeID, "adir")
	api.addFile(dirID, "inside.txt", []byte("nested"))
	api.addFile(scopeID, "z.txt", []byte("last"))
	noteID := api.addFile(scopeID, "note.txt", []byte("hello pCloud"))
	api.entries[noteID].modified = 1_700_000_000
	fsys := api.filesystem(t, scopeID, "token")

	rootInfo, err := fsys.Stat(".")
	if err != nil {
		t.Fatalf("Stat root: %v", err)
	}
	if !rootInfo.IsDir() || rootInfo.Name() != "." {
		t.Fatalf("unexpected root info: name=%q dir=%v", rootInfo.Name(), rootInfo.IsDir())
	}

	entries, err := fsys.ReadDir(".")
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	gotNames := make([]string, len(entries))
	for i := range entries {
		gotNames[i] = entries[i].Name()
	}
	if got, want := strings.Join(gotNames, ","), "adir,note.txt,z.txt"; got != want {
		t.Fatalf("ReadDir names = %q, want %q", got, want)
	}

	info, err := fsys.Stat("note.txt")
	if err != nil {
		t.Fatalf("Stat file: %v", err)
	}
	if info.IsDir() || info.Size() != 12 || !info.ModTime().Equal(time.Unix(1_700_000_000, 0)) {
		t.Fatalf("unexpected file info: mode=%v size=%d mod=%v", info.Mode(), info.Size(), info.ModTime())
	}

	data, err := fs.ReadFile(fsys, "note.txt")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(data) != "hello pCloud" {
		t.Fatalf("ReadFile = %q", data)
	}

	dir, err := fsys.Open("adir")
	if err != nil {
		t.Fatalf("Open directory: %v", err)
	}
	_, readErr := dir.Read(make([]byte, 1))
	if !errors.Is(readErr, syscall.EISDIR) {
		t.Fatalf("reading directory error = %v, want EISDIR", readErr)
	}
	if err := dir.Close(); err != nil {
		t.Fatalf("Close directory: %v", err)
	}

	var walked []string
	err = fs.WalkDir(fsys, ".", func(name string, _ fs.DirEntry, err error) error {
		if err == nil {
			walked = append(walked, name)
		}
		return err
	})
	if err != nil {
		t.Fatalf("WalkDir: %v", err)
	}
	wantWalk := []string{".", "adir", "adir/inside.txt", "note.txt", "z.txt"}
	if strings.Join(walked, ",") != strings.Join(wantWalk, ",") {
		t.Fatalf("WalkDir = %v, want %v", walked, wantWalk)
	}

	if _, err := fsys.Stat("../escape"); !errors.Is(err, fs.ErrInvalid) {
		t.Fatalf("invalid path error = %v, want fs.ErrInvalid", err)
	}
	if _, err := fsys.ReadDir("note.txt"); !errors.Is(err, syscall.ENOTDIR) {
		t.Fatalf("ReadDir file error = %v, want ENOTDIR", err)
	}
}

func TestPCloudFSWriteOperationsAreUnsupported(t *testing.T) {
	api := newFakePCloudAPI(t)
	scopeID := api.addFolder(0, "scope")
	// A deliberately invalid token proves these methods return locally without
	// consulting the pCloud API first.
	fsys := api.filesystem(t, scopeID, "wrong-token")

	operations := []struct {
		name string
		call func() error
	}{
		{"OpenWriter", func() error { _, err := fsys.OpenWriter("new.txt"); return err }},
		{"WriteFile", func() error { return fsys.WriteFile("new.txt", []byte("data")) }},
		{"Mkdir", func() error { return fsys.Mkdir("new-dir") }},
		{"MkdirAll", func() error { return fsys.MkdirAll("new-dir/child") }},
		{"Rename", func() error { return fsys.Rename("old.txt", "new.txt") }},
		{"Remove", func() error { return fsys.Remove("old.txt") }},
		{"RemoveAll", func() error { return fsys.RemoveAll("old-dir") }},
	}

	for _, operation := range operations {
		t.Run(operation.name, func(t *testing.T) {
			err := operation.call()
			if !errors.Is(err, errors.ErrUnsupported) {
				t.Fatalf("error = %v, want errors.ErrUnsupported", err)
			}
			var pathErr *fs.PathError
			if !errors.As(err, &pathErr) {
				t.Fatalf("error = %T, want *fs.PathError", err)
			}
		})
	}
}

func TestPCloudFSErrorMappingAndConfig(t *testing.T) {
	if _, err := NewPCloudFSWithConfig(PCloudConfig{APIURL: "://bad"}); err == nil {
		t.Fatal("invalid API URL unexpectedly succeeded")
	}
	if _, err := NewPCloudFSWithConfig(PCloudConfig{RootFolderID: -1}); err == nil {
		t.Fatal("negative root folder ID unexpectedly succeeded")
	}

	api := newFakePCloudAPI(t)
	fsys := api.filesystem(t, 0, "wrong-token")
	_, err := fsys.Stat(".")
	if !errors.Is(err, fs.ErrPermission) {
		t.Fatalf("authentication error = %v, want fs.ErrPermission", err)
	}
	var apiErr *PCloudError
	if !errors.As(err, &apiErr) || apiErr.Result != 2000 || apiErr.Message != "Log in failed." {
		t.Fatalf("authentication error does not preserve pCloud response: %#v", apiErr)
	}
}

type fakePCloudEntry struct {
	id       int64
	parent   int64
	name     string
	folder   bool
	data     []byte
	modified int64
}

type fakePCloudAPI struct {
	t       *testing.T
	server  *httptest.Server
	mu      sync.Mutex
	nextID  int64
	entries map[int64]*fakePCloudEntry
}

func newFakePCloudAPI(t *testing.T) *fakePCloudAPI {
	t.Helper()
	api := &fakePCloudAPI{
		t:      t,
		nextID: 1,
		entries: map[int64]*fakePCloudEntry{
			0: {id: 0, parent: -1, name: "/", folder: true, modified: 1_600_000_000},
		},
	}
	api.server = httptest.NewServer(http.HandlerFunc(api.serveHTTP))
	t.Cleanup(api.server.Close)
	return api
}

func (a *fakePCloudAPI) filesystem(t *testing.T, rootID int64, token string) *PCloudFS {
	t.Helper()
	fsys, err := NewPCloudFSWithConfig(PCloudConfig{
		AccessToken:  token,
		APIURL:       a.server.URL,
		RootFolderID: rootID,
		HTTPClient:   a.server.Client(),
	})
	if err != nil {
		t.Fatalf("NewPCloudFSWithConfig: %v", err)
	}
	return fsys
}

func (a *fakePCloudAPI) addFolder(parent int64, name string) int64 {
	return a.addEntry(parent, name, true, nil)
}

func (a *fakePCloudAPI) addFile(parent int64, name string, data []byte) int64 {
	return a.addEntry(parent, name, false, data)
}

func (a *fakePCloudAPI) addEntry(parent int64, name string, folder bool, data []byte) int64 {
	id := a.nextID
	a.nextID++
	a.entries[id] = &fakePCloudEntry{
		id: id, parent: parent, name: name, folder: folder,
		data: append([]byte(nil), data...), modified: 1_600_000_000 + id,
	}
	return id
}

func (a *fakePCloudAPI) serveHTTP(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/download/") {
		a.serveDownload(w, r)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if r.URL.RawQuery != "" {
		a.t.Errorf("API credentials/parameters leaked into query: %q", r.URL.RawQuery)
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if !a.authenticate(w, r.Form.Get("access_token")) {
		return
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	switch r.URL.Path {
	case "/listfolder":
		a.serveListFolder(w, r.Form)
	case "/getfilelink":
		a.serveGetFileLink(w, r.Form)
	default:
		http.NotFound(w, r)
	}
}

func (a *fakePCloudAPI) authenticate(w http.ResponseWriter, token string) bool {
	if token == "token" {
		return true
	}
	a.writeError(w, 2000, "Log in failed.")
	return false
}

func (a *fakePCloudAPI) serveListFolder(w http.ResponseWriter, form url.Values) {
	id := formInt64(form, "folderid")
	entry, ok := a.entries[id]
	if !ok || !entry.folder {
		a.writeError(w, 2005, "Directory does not exist.")
		return
	}
	contents := make([]any, 0)
	children := make([]*fakePCloudEntry, 0)
	for _, child := range a.entries {
		if child.parent == id {
			children = append(children, child)
		}
	}
	sort.Slice(children, func(i, j int) bool { return children[i].name > children[j].name })
	for _, child := range children {
		contents = append(contents, a.metadata(child, nil))
	}
	a.writeJSON(w, map[string]any{
		"result":   0,
		"metadata": a.metadata(entry, contents),
	})
}

func (a *fakePCloudAPI) serveGetFileLink(w http.ResponseWriter, form url.Values) {
	id := formInt64(form, "fileid")
	entry, ok := a.entries[id]
	if !ok || entry.folder {
		a.writeError(w, 2009, "File not found.")
		return
	}
	host := strings.TrimPrefix(a.server.URL, "http://")
	a.writeJSON(w, map[string]any{
		"result": 0,
		"hosts":  []string{host},
		"path":   fmt.Sprintf("/download/%d?signature=test", id),
	})
}

func (a *fakePCloudAPI) serveDownload(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(strings.TrimPrefix(r.URL.Path, "/download/"), 10, 64)
	a.mu.Lock()
	entry, ok := a.entries[id]
	var data []byte
	if ok {
		data = append([]byte(nil), entry.data...)
	}
	a.mu.Unlock()
	if !ok || entry.folder || r.URL.Query().Get("signature") != "test" {
		http.NotFound(w, r)
		return
	}
	_, _ = w.Write(data)
}

func (a *fakePCloudAPI) metadata(entry *fakePCloudEntry, contents []any) map[string]any {
	meta := map[string]any{
		"name":           entry.name,
		"isfolder":       entry.folder,
		"parentfolderid": entry.parent,
		"modified":       entry.modified,
		"created":        entry.modified,
	}
	if entry.folder {
		meta["folderid"] = entry.id
		if contents != nil {
			meta["contents"] = contents
		}
	} else {
		meta["fileid"] = entry.id
		meta["size"] = len(entry.data)
	}
	return meta
}

func (a *fakePCloudAPI) writeError(w http.ResponseWriter, result int, message string) {
	a.writeJSON(w, map[string]any{"result": result, "error": message})
}

func (a *fakePCloudAPI) writeJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(value); err != nil {
		a.t.Errorf("encode fake pCloud response: %v", err)
	}
}

func formInt64(form url.Values, name string) int64 {
	value, _ := strconv.ParseInt(form.Get(name), 10, 64)
	return value
}
