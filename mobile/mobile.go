// Package mobile runs the PlainShelf server inside the Android app, the way
// desktop/app.go does for Wails. It is built with gomobile bind and the UI
// reaches it through no listener: the native plugin passes each request to
// Handle. The one listener is the LAN share (StartLANShare), open only while
// the user runs it. Shelves are created, imported, exported and removed
// through the functions below, never through an HTTP route.
//
// Panic recovery is limited to individual entry points; not every exported
// function converts panics into errors.
package mobile

import (
	"bufio"
	"crypto/rand"
	"encoding/json/v2"
	"errors"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime/debug"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/voilelab/plainshelf/internal/fsutil"
	"github.com/voilelab/plainshelf/internal/jsonopt"
	"github.com/voilelab/plainshelf/internal/logutil"
	"github.com/voilelab/plainshelf/internal/shelfzip"
	"github.com/voilelab/plainshelf/internal/util"
	"github.com/voilelab/plainshelf/server"
	"github.com/voilelab/plainshelf/shelf"
)

// ErrNotStarted is returned before Start or after Stop.
var ErrNotStarted = errors.New("plainshelf: not started")

const (
	configFile = "shelves.json"
	shelvesDir = "shelves"
)

// Response is one HTTP answer; its body is in the file Handle was given.
// Header is a JSON object of single values.
type Response struct {
	Status int
	Header string
}

type shelfEntry struct {
	ID   string `json:"id"`
	Name string `json:"name"`

	LastExport string `json:"last_export,omitempty"`
}

type config struct {
	Shelves []shelfEntry `json:"shelves"`
}

type instance struct {
	mu      sync.RWMutex
	dataDir string
	conf    config
	app     *server.App
	handler http.Handler

	virtual map[string]virtualShelf

	share *lanShare

	stopped bool
}

var (
	stateMu sync.Mutex
	current *instance
)

// Start opens every local shelf under dataDir, the app's Directory.Data.
func Start(dataDir string) (err error) {
	defer recoverInto(&err)
	stateMu.Lock()
	defer stateMu.Unlock()
	if current != nil {
		return nil
	}
	rt, err := start(dataDir)
	if err != nil {
		return err
	}
	current = rt
	return nil
}

func start(dataDir string) (*instance, error) {
	// Android's default /data/local/tmp is not writable by an ordinary app.
	// Configure Go's temporary files before opening shelves or starting workers.
	tmpDir, err := filepath.Abs(filepath.Join(dataDir, "tmp"))
	if err != nil {
		return nil, util.Errorf("%w", err)
	}
	if err := os.MkdirAll(tmpDir, 0o700); err != nil {
		return nil, util.Errorf("%w", err)
	}
	if err := os.Setenv("TMPDIR", tmpDir); err != nil {
		return nil, util.Errorf("%w", err)
	}
	if err := os.MkdirAll(filepath.Join(dataDir, shelvesDir), 0o700); err != nil {
		return nil, util.Errorf("%w", err)
	}
	conf, err := loadConfig(dataDir)
	if err != nil {
		return nil, err
	}
	shelves := make([]*shelf.ShelfConfWithID, 0, len(conf.Shelves))
	for _, e := range conf.Shelves {
		c := shelfConf(dataDir, e)
		shelves = append(shelves, &c)
	}
	app, err := server.NewApp(&server.AppConf{
		Logger: logutil.LogConf{
			Level:  "info",
			Format: "json",
			LogFile: logutil.LogFileConf{
				Type:   logutil.LogFileTypeNameRotate,
				Dir:    filepath.Join(dataDir, "logs"),
				Prefix: "app",
			},
		},
		Shelves:    shelves,
		StorePath:  filepath.Join(dataDir, "store"),
		CoverToJPG: true,

		Security:    &server.SecurityConf{Mode: server.SecurityModeNone},
		Credentials: credentialResolver{},
	})
	if err != nil {
		return nil, util.Errorf("%w", err)
	}
	if err := app.Start(); err != nil {
		_ = app.Close()
		return nil, util.Errorf("%w", err)
	}
	return &instance{dataDir: dataDir, conf: *conf, app: app, handler: app.Handler(), virtual: map[string]virtualShelf{}}, nil
}

// Stop closes the server; Start may be called again afterwards.
func Stop() (err error) {
	defer recoverInto(&err)
	stateMu.Lock()
	defer stateMu.Unlock()
	if current == nil {
		return nil
	}
	rt := current
	current = nil
	rt.mu.Lock()
	defer rt.mu.Unlock()
	rt.stopped = true
	rt.closeLANShare()
	return rt.app.Close()
}

func running() (*instance, error) {
	stateMu.Lock()
	defer stateMu.Unlock()
	if current == nil {
		return nil, ErrNotStarted
	}
	return current, nil
}

func (rt *instance) rlockRunning() error {
	rt.mu.RLock()
	if rt.stopped {
		rt.mu.RUnlock()
		return ErrNotStarted
	}
	return nil
}

func (rt *instance) lockRunning() error {
	rt.mu.Lock()
	if rt.stopped {
		rt.mu.Unlock()
		return ErrNotStarted
	}
	return nil
}

var taskWaitTimeout = 30 * time.Second

// ErrBusy is returned when background tasks do not settle in time.
var ErrBusy = errors.New("plainshelf: background tasks are still running")

func (rt *instance) waitForTasks() error {
	deadline := time.Now().Add(taskWaitTimeout)
	for rt.app.TaskChains().Busy() {
		if time.Now().After(deadline) {
			return ErrBusy
		}
		time.Sleep(20 * time.Millisecond)
	}
	return nil
}

// Handle serves one request. headerJSON is a JSON object of header values and
// may be empty. The request body is read from bodyPath, none when empty, and
// the response body is written to outPath: bodies cross JNI as paths, never as
// byte arrays, so a large book is not held in memory on both sides.
func Handle(method, url, headerJSON, bodyPath, outPath string) (res *Response, err error) {
	defer recoverInto(&err)
	rt, err := running()
	if err != nil {
		return nil, err
	}
	var hdrs map[string]string
	if headerJSON != "" {
		if err := json.Unmarshal([]byte(headerJSON), &hdrs); err != nil {
			return nil, util.Errorf("header: %w", err)
		}
	}
	var body io.Reader = http.NoBody
	if bodyPath != "" {
		f, err := os.Open(bodyPath)
		if err != nil {
			return nil, util.Errorf("%w", err)
		}
		defer f.Close()
		body = f
	}
	out, err := os.Create(outPath)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}
	defer out.Close()

	req := httptest.NewRequest(method, url, body)
	for k, v := range hdrs {
		req.Header.Set(k, v)
	}
	w := &fileResponseWriter{header: http.Header{}, out: bufio.NewWriter(out)}

	if err := rt.rlockRunning(); err != nil {
		return nil, err
	}
	defer rt.mu.RUnlock()
	rt.handler.ServeHTTP(w, req)

	if w.err == nil {
		w.err = w.out.Flush()
	}
	if w.err != nil {
		return nil, util.Errorf("%w", w.err)
	}
	if err := out.Close(); err != nil {
		return nil, util.Errorf("%w", err)
	}
	if w.status == 0 {
		w.status = http.StatusOK
	}
	flat := make(map[string]string, len(w.header))
	for k := range w.header {
		flat[k] = w.header.Get(k)
	}
	hdrOut, err := json.Marshal(flat, jsonopt.API())
	if err != nil {
		return nil, util.Errorf("%w", err)
	}
	return &Response{Status: w.status, Header: string(hdrOut)}, nil
}

type fileResponseWriter struct {
	header http.Header
	status int
	out    *bufio.Writer
	err    error
}

func (w *fileResponseWriter) Header() http.Header { return w.header }

func (w *fileResponseWriter) WriteHeader(status int) {
	if w.status == 0 {
		w.status = status
	}
}

func (w *fileResponseWriter) Write(p []byte) (int, error) {
	w.WriteHeader(http.StatusOK)
	if w.err != nil {
		return 0, w.err
	}
	n, err := w.out.Write(p)
	w.err = err
	return n, err
}

// ListShelves returns the local shelves as a JSON array of
// {id, name, last_export}.
func ListShelves() (out string, err error) {
	defer recoverInto(&err)
	rt, err := running()
	if err != nil {
		return "", err
	}
	if err := rt.rlockRunning(); err != nil {
		return "", err
	}
	defer rt.mu.RUnlock()
	bs, err := json.Marshal(rt.conf.Shelves, jsonopt.API())
	if err != nil {
		return "", util.Errorf("%w", err)
	}
	return string(bs), nil
}

// CreateShelf makes an empty shelf and returns its ID.
func CreateShelf(name string) (id string, err error) {
	defer recoverInto(&err)
	rt, err := running()
	if err != nil {
		return "", err
	}
	if err := rt.lockRunning(); err != nil {
		return "", err
	}
	defer rt.mu.Unlock()
	return rt.addShelf(name, nil)
}

// ImportShelf makes a new shelf from a shelf zip, such as one ExportShelf or
// the wasm demo wrote, and returns its ID. A zip that is not a shelf is
// rejected and no shelf is created.
func ImportShelf(name, zipPath string) (id string, err error) {
	defer recoverInto(&err)
	rt, err := running()
	if err != nil {
		return "", err
	}
	if err := rt.lockRunning(); err != nil {
		return "", err
	}
	defer rt.mu.Unlock()
	if err := rt.waitForTasks(); err != nil {
		return "", err
	}
	return rt.addShelf(name, func(libRoot string) error {
		f, err := os.Open(zipPath)
		if err != nil {
			return util.Errorf("%w", err)
		}
		defer f.Close()
		info, err := f.Stat()
		if err != nil {
			return util.Errorf("%w", err)
		}
		if info.Size() > MaxImportBytes() {
			return ErrTooLarge
		}
		return shelfzip.Import(f, info.Size(), libRoot)
	})
}

// ErrTooLarge rejects a zip over MaxImportBytes.
var ErrTooLarge = shelfzip.ErrTooLarge

// MaxImportBytes is the largest zip ImportShelf takes; the plugin stops
// staging a picked file past it.
func MaxImportBytes() int64 { return shelfzip.MaxImportBytes() }

func (rt *instance) addShelf(name string, populate func(libRoot string) error) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", util.Errorf("shelf name cannot be empty")
	}
	entry := shelfEntry{ID: newShelfID(), Name: name}
	conf := shelfConf(rt.dataDir, entry)
	if err := os.MkdirAll(conf.LibRoot, 0o700); err != nil {
		return "", util.Errorf("%w", err)
	}
	ok := false
	defer func() {
		if !ok {
			_ = os.RemoveAll(conf.LibRoot)
		}
	}()
	if populate != nil {
		if err := populate(conf.LibRoot); err != nil {
			return "", err
		}
	}
	next := rt.conf
	next.Shelves = append(slices.Clone(rt.conf.Shelves), entry)
	if err := saveConfig(rt.dataDir, &next); err != nil {
		return "", err
	}
	if err := rt.app.AddShelf(conf); err != nil {
		_ = saveConfig(rt.dataDir, &rt.conf)
		return "", util.Errorf("%w", err)
	}
	rt.conf = next
	ok = true
	return entry.ID, nil
}

// RenameShelf changes a shelf's display name; its ID and books stay.
func RenameShelf(id, name string) (err error) {
	defer recoverInto(&err)
	rt, err := running()
	if err != nil {
		return err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return util.Errorf("shelf name cannot be empty")
	}
	if err := rt.lockRunning(); err != nil {
		return err
	}
	defer rt.mu.Unlock()
	if err := rt.waitForTasks(); err != nil {
		return err
	}
	i, err := rt.find(id)
	if err != nil {
		return err
	}
	next := rt.conf
	next.Shelves = slices.Clone(rt.conf.Shelves)
	next.Shelves[i].Name = name
	if err := saveConfig(rt.dataDir, &next); err != nil {
		return err
	}
	if err := rt.app.UpdateShelf(shelfConf(rt.dataDir, next.Shelves[i])); err != nil {
		_ = saveConfig(rt.dataDir, &rt.conf)
		return util.Errorf("%w", err)
	}
	rt.conf = next
	return nil
}

// RemoveShelf closes a shelf and deletes its directory with every book in it.
func RemoveShelf(id string) (err error) {
	defer recoverInto(&err)
	rt, err := running()
	if err != nil {
		return err
	}
	if err := rt.lockRunning(); err != nil {
		return err
	}
	defer rt.mu.Unlock()
	if err := rt.waitForTasks(); err != nil {
		return err
	}
	i, err := rt.find(id)
	if err != nil {
		return err
	}
	next := rt.conf
	next.Shelves = slices.Delete(slices.Clone(rt.conf.Shelves), i, i+1)
	if err := saveConfig(rt.dataDir, &next); err != nil {
		return err
	}
	rt.conf = next
	rt.forgetShared(id)
	if err := rt.app.RemoveShelf(id); err != nil {
		return util.Errorf("%w", err)
	}
	if err := os.RemoveAll(libRootOf(rt.dataDir, id)); err != nil {
		return util.Errorf("%w", err)
	}
	return nil
}

// ExportShelf writes a shelf to zipPath in the layout a desktop or server
// lib_root has. The caller records it with MarkExported once the zip has
// reached its destination, which may be a copy away.
func ExportShelf(id, zipPath string) (err error) {
	defer recoverInto(&err)
	rt, err := running()
	if err != nil {
		return err
	}
	if err := rt.lockRunning(); err != nil {
		return err
	}
	defer rt.mu.Unlock()
	if err := rt.waitForTasks(); err != nil {
		return err
	}
	if _, err := rt.find(id); err != nil {
		return err
	}
	return writeZip(zipPath, libRootOf(rt.dataDir, id))
}

// MarkExported records now as the shelf's last export.
func MarkExported(id string) (err error) {
	defer recoverInto(&err)
	rt, err := running()
	if err != nil {
		return err
	}
	if err := rt.lockRunning(); err != nil {
		return err
	}
	defer rt.mu.Unlock()
	i, err := rt.find(id)
	if err != nil {
		return err
	}
	next := rt.conf
	next.Shelves = slices.Clone(rt.conf.Shelves)
	next.Shelves[i].LastExport = time.Now().UTC().Format(time.RFC3339)
	if err := saveConfig(rt.dataDir, &next); err != nil {
		return err
	}
	rt.conf = next
	return nil
}

func writeZip(zipPath, libRoot string) error {
	tmp, err := os.CreateTemp(filepath.Dir(zipPath), ".export-*.zip")
	if err != nil {
		return util.Errorf("%w", err)
	}
	defer os.Remove(tmp.Name())
	if err := shelfzip.Export(tmp, libRoot); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return util.Errorf("%w", err)
	}
	if err := os.Rename(tmp.Name(), zipPath); err != nil {
		return util.Errorf("%w", err)
	}
	return nil
}

func (rt *instance) find(id string) (int, error) {
	i := slices.IndexFunc(rt.conf.Shelves, func(e shelfEntry) bool { return e.ID == id })
	if i < 0 {
		return 0, util.Errorf("shelf %q not found", id)
	}
	return i, nil
}

func shelfConf(dataDir string, e shelfEntry) shelf.ShelfConfWithID {
	return shelf.ShelfConfWithID{
		ID:      e.ID,
		Name:    e.Name,
		LibRoot: libRootOf(dataDir, e.ID),

		LockMode: "none",
	}
}

func libRootOf(dataDir, id string) string {
	return filepath.Join(dataDir, shelvesDir, id)
}

func newShelfID() string {
	return "local-" + strings.ToLower(rand.Text()[:16])
}

func loadConfig(dataDir string) (*config, error) {
	bs, err := os.ReadFile(filepath.Join(dataDir, configFile))
	if errors.Is(err, fs.ErrNotExist) {
		return &config{}, nil
	}
	if err != nil {
		return nil, util.Errorf("%w", err)
	}
	var conf config
	if err := json.Unmarshal(bs, &conf, jsonopt.Disk()); err != nil {
		return nil, util.Errorf("%s: %w", configFile, err)
	}
	return &conf, nil
}

func saveConfig(dataDir string, conf *config) error {
	bs, err := json.Marshal(conf, jsonopt.Disk())
	if err != nil {
		return util.Errorf("%w", err)
	}
	if err := fsutil.WriteTextFileAtomic(filepath.Join(dataDir, configFile), "shelves-*.json", string(bs)); err != nil {
		return util.Errorf("%w", err)
	}
	return nil
}

func recoverInto(err *error) {
	if r := recover(); r != nil {
		*err = util.Errorf("plainshelf: panic: %v\n%s", r, debug.Stack())
	}
}
