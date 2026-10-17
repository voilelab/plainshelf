package server

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"syscall"

	"github.com/voilelab/plainshelf/internal/fsutil"
	"github.com/voilelab/plainshelf/internal/logutil"
	"github.com/voilelab/plainshelf/internal/util"
	"github.com/voilelab/plainshelf/shelf"
	"golang.org/x/net/webdav"
)

// DAVExportConf turns on the read-only WebDAV export of every local shelf, on
// a listener of its own. Absent, nothing is exported and no port is opened.
type DAVExportConf struct {
	Addr         string `yaml:"addr"`
	ReadTimeout  string `yaml:"read_timeout"`
	WriteTimeout string `yaml:"write_timeout"`

	// TokenFile holds the bearer token every request must carry. It is
	// generated when missing and replaced only by RotateDAVExportToken.
	TokenFile string `yaml:"token_file"`

	// AllowedOrigins are the browser origins sent CORS headers; empty sends
	// none. No wildcard: any page holding the token could then read the shelf.
	AllowedOrigins []string `yaml:"allowed_origins"`
}

func (conf *DAVExportConf) validate(apiAddr string) error {
	switch {
	case conf.Addr == "":
		return util.Errorf("dav_export.addr is required")
	case conf.Addr == apiAddr:
		return util.Errorf("dav_export.addr must differ from server_conf.addr")
	case conf.TokenFile == "":
		return util.Errorf("dav_export.token_file is required")
	}
	_, err := davOriginSet(conf.AllowedOrigins)
	return err
}

// davOriginSet normalizes allowed_origins; nil when there are none.
func davOriginSet(origins []string) (map[string]struct{}, error) {
	if len(origins) == 0 {
		return nil, nil
	}
	set := make(map[string]struct{}, len(origins))
	for _, origin := range origins {
		normalized, err := davNormalizeOrigin(origin)
		if err != nil {
			return nil, util.Errorf("invalid dav_export.allowed_origins entry %q: %w", origin, err)
		}
		set[normalized] = struct{}{}
	}
	return set, nil
}

// davNormalizeOrigin is normalizeOrigin without a default port, which a
// browser leaves out of the Origin it sends.
func davNormalizeOrigin(origin string) (string, error) {
	normalized, err := normalizeOrigin(origin)
	if err != nil {
		return "", err
	}
	switch {
	case strings.HasPrefix(normalized, "http://"):
		normalized = strings.TrimSuffix(normalized, ":80")
	case strings.HasPrefix(normalized, "https://"):
		normalized = strings.TrimSuffix(normalized, ":443")
	}
	return normalized, nil
}

const davExportPrefix = "/dav/"

// davExportAllow is every method the export answers; the rest get 405.
const davExportAllow = "OPTIONS, GET, HEAD, PROPFIND"

// davCORSAllowHeaders is what internal/davfs sends besides simple headers.
const davCORSAllowHeaders = "Authorization, Content-Type, Depth, Range"

const davTokenBytes = 32

// davPropfindMaxBody bounds a PROPFIND request body; a real one is a few
// hundred bytes.
const davPropfindMaxBody = 64 << 10

// EnsureDAVExportToken generates the token file when it does not exist yet.
func EnsureDAVExportToken(tokenFile string) error {
	token, err := readDAVExportToken(tokenFile)
	if err == nil {
		if token == "" {
			return util.Errorf("dav_export.token_file %q is empty", tokenFile)
		}
		return nil
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return util.Errorf("%w", err)
	}
	_, err = RotateDAVExportToken(tokenFile)
	return err
}

// RotateDAVExportToken replaces the token with a new one and returns it. A
// running export reads the file on every request, so the old token stops
// working at once.
func RotateDAVExportToken(tokenFile string) (string, error) {
	buf := make([]byte, davTokenBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", util.Errorf("%w", err)
	}
	token := base64.RawURLEncoding.EncodeToString(buf)

	// 0600, and a reader never sees a half-written token.
	if err := fsutil.WriteTextFileAtomic(tokenFile, filepath.Base(tokenFile)+".*.tmp", token+"\n"); err != nil {
		return "", util.Errorf("%w", err)
	}
	return token, nil
}

// davTokenFileExposed reports a token file readable by group or others. Windows
// has no such bits to check.
func davTokenFileExposed(tokenFile string) bool {
	if runtime.GOOS == "windows" {
		return false
	}
	info, err := os.Stat(tokenFile)
	return err == nil && info.Mode().Perm()&0o077 != 0
}

func readDAVExportToken(tokenFile string) (string, error) {
	bs, err := os.ReadFile(tokenFile)
	if err != nil {
		return "", util.Errorf("%w", err)
	}
	return strings.TrimSpace(string(bs)), nil
}

// DAVExportHandler serves /dav/{shelf_id}/ for every local shelf: a read-only
// WebDAV subset limited to the shelf's own files (shelf.IsShelfFile). It holds
// no route of the HTTP API.
func (app *App) DAVExportHandler(tokenFile string) http.Handler {
	return app.DAVExportShelvesHandler(tokenFile, nil)
}

// DAVExportShelvesHandler is DAVExportHandler limited to the shelves exported
// reports true for, asked on every request; nil exports every local shelf. A
// shelf it refuses is 404, the same as one that does not exist.
func (app *App) DAVExportShelvesHandler(tokenFile string, exported func(shelfID string) bool) http.Handler {
	return app.davExportHandler(tokenFile, exported, nil)
}

// davExportCORSHandler is DAVExportHandler answering CORS for the origins
// allowedOrigins names.
func (app *App) davExportCORSHandler(tokenFile string, allowedOrigins []string) (http.Handler, error) {
	origins, err := davOriginSet(allowedOrigins)
	if err != nil {
		return nil, err
	}
	return app.davExportHandler(tokenFile, nil, origins), nil
}

func (app *App) davExportHandler(tokenFile string, exported func(shelfID string) bool, origins map[string]struct{}) *davExport {
	return &davExport{
		logger:    &app.Logger,
		shelves:   app.shelfManager,
		tokenFile: tokenFile,
		exported:  exported,
		origins:   origins,
		locks:     webdav.NewMemLS(),
	}
}

type davExport struct {
	logger    *logutil.Logger
	shelves   *shelf.ShelfManager
	tokenFile string
	exported  func(shelfID string) bool
	origins   map[string]struct{}

	// tokenErrLogged keeps an unreadable token file from logging once per
	// request; cleared when the file reads again.
	tokenErrLogged atomic.Bool

	// Never used: LOCK is refused before the handler, which only insists on
	// having one.
	locks webdav.LockSystem
}

func (d *davExport) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if d.applyCORS(w, r) {
		return
	}
	// Method first, so a write is 405 with or without a token.
	switch r.Method {
	case http.MethodOptions, http.MethodGet, http.MethodHead, "PROPFIND":
	default:
		w.Header().Set("Allow", davExportAllow)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	rest, ok := strings.CutPrefix(r.URL.Path, davExportPrefix)
	if !ok {
		http.NotFound(w, r)
		return
	}
	if !d.authorized(r) {
		w.Header().Set("WWW-Authenticate", `Bearer realm="PlainShelf"`)
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if !davPathSafe(rest) {
		http.Error(w, "bad path", http.StatusBadRequest)
		return
	}

	shelfID, _, _ := strings.Cut(rest, "/")
	if d.exported != nil && !d.exported(shelfID) {
		http.NotFound(w, r)
		return
	}
	data, ok := d.shelves.GetShelf(shelfID)
	if !ok || data.Shelf == nil {
		http.NotFound(w, r)
		return
	}
	files, ok := data.LocalFiles()
	if !ok {
		http.NotFound(w, r)
		return
	}

	switch r.Method {
	case http.MethodOptions:
		w.Header().Set("Allow", davExportAllow)
		w.Header().Set("DAV", "1")
		w.WriteHeader(http.StatusOK)
		return
	case "PROPFIND":
		// RFC 4918 9.1: a missing Depth means infinity, which is refused.
		if depth := r.Header.Get("Depth"); depth != "0" && depth != "1" {
			http.Error(w, "only Depth 0 and 1 are supported", http.StatusForbidden)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, davPropfindMaxBody)
	}

	h := &webdav.Handler{
		Prefix:     davExportPrefix + shelfID,
		FileSystem: davFS{root: files},
		LockSystem: d.locks,
	}
	h.ServeHTTP(w, r)
}

// applyCORS adds CORS headers for an allowed origin, and reports true when it
// answered a preflight. A preflight carries no token, so it is answered before
// any path is resolved: it reveals nothing about the shelves.
func (d *davExport) applyCORS(w http.ResponseWriter, r *http.Request) bool {
	if d.origins == nil {
		return false
	}
	h := w.Header()
	h.Add("Vary", "Origin")
	originHeader := r.Header.Get("Origin")
	origin, err := davNormalizeOrigin(originHeader)
	if err != nil {
		return false
	}
	if _, ok := d.origins[origin]; !ok {
		return false
	}
	h.Set("Access-Control-Allow-Origin", originHeader)
	if r.Method != http.MethodOptions || r.Header.Get("Access-Control-Request-Method") == "" {
		return false
	}
	h.Set("Access-Control-Allow-Methods", davExportAllow)
	h.Set("Access-Control-Allow-Headers", davCORSAllowHeaders)
	w.WriteHeader(http.StatusNoContent)
	return true
}

func (d *davExport) authorized(r *http.Request) bool {
	got := bearerToken(r.Header.Get("Authorization"))
	if got == "" {
		return false
	}
	want, err := readDAVExportToken(d.tokenFile)
	if err != nil || want == "" {
		// Fail closed: a missing token file rejects every request.
		if !d.tokenErrLogged.Swap(true) {
			d.logger.Error("dav export token unreadable", "token_file", d.tokenFile, "error", err)
		}
		return false
	}
	d.tokenErrLogged.Store(false)
	// Hashed first so the comparison does not leak the token's length.
	gotSum, wantSum := sha256.Sum256([]byte(got)), sha256.Sum256([]byte(want))
	return subtle.ConstantTimeCompare(gotSum[:], wantSum[:]) == 1
}

// davPathSafe rejects a ".." segment, a backslash or a NUL anywhere in the
// decoded path, before anything resolves it.
func davPathSafe(p string) bool {
	if strings.ContainsAny(p, "\\\x00") {
		return false
	}
	for seg := range strings.SplitSeq(p, "/") {
		if seg == ".." {
			return false
		}
	}
	return true
}

// davFS is a read-only webdav.FileSystem over one shelf directory. A path
// outside shelf.IsShelfFile does not exist, and neither does one through a
// symlink: os.Root follows a link that stays inside the shelf, which would
// reach app/ from under books/.
type davFS struct {
	root fsutil.ReadFS
}

var _ webdav.FileSystem = davFS{}

// rel maps a webdav name ("/books/x") to a shelf path ("books/x", "." for the
// root), reporting false when it is not exported.
func (davFS) rel(name string) (string, bool) {
	rel := strings.TrimPrefix(path.Clean("/"+name), "/")
	if rel == "" {
		rel = "."
	}
	return rel, shelf.IsShelfFile(rel)
}

// open opens an exported path through fsutil.NoFollowFS, so a symlink along
// it, one swapped in during the request included, does not exist.
func (d davFS) open(name string) (fs.File, string, error) {
	rel, ok := d.rel(name)
	if !ok {
		return nil, "", os.ErrNotExist
	}
	nf, ok := d.root.(fsutil.NoFollowFS)
	if !ok {
		return nil, "", os.ErrNotExist
	}
	f, err := nf.OpenNoFollow(rel)
	if err != nil {
		return nil, "", davErr(err)
	}
	return f, rel, nil
}

func (davFS) Mkdir(context.Context, string, os.FileMode) error { return os.ErrPermission }
func (davFS) RemoveAll(context.Context, string) error          { return os.ErrPermission }
func (davFS) Rename(context.Context, string, string) error     { return os.ErrPermission }

func (d davFS) OpenFile(_ context.Context, name string, flag int, _ os.FileMode) (webdav.File, error) {
	if flag&(os.O_WRONLY|os.O_RDWR|os.O_APPEND|os.O_CREATE|os.O_TRUNC) != 0 {
		return nil, os.ErrPermission
	}
	f, rel, err := d.open(name)
	if err != nil {
		return nil, err
	}
	seeker, ok := f.(io.Seeker)
	if !ok {
		_ = f.Close()
		return nil, util.Errorf("dav export: %q is not seekable", rel)
	}
	return &davFile{File: f, seeker: seeker, rel: rel}, nil
}

func (d davFS) Stat(_ context.Context, name string) (os.FileInfo, error) {
	f, _, err := d.open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, davErr(err)
	}
	return davFileInfo{info}, nil
}

// davErr unwraps a not-exist error to os.ErrNotExist: webdav tests with
// os.IsNotExist, which does not see through util.Errorf, and answers anything
// else with 405. A path through a file (ENOTDIR) is not there either; keeping
// it 404 also leaves no status difference to probe hidden names with.
func davErr(err error) error {
	if errors.Is(err, fs.ErrNotExist) || errors.Is(err, syscall.ENOTDIR) {
		return os.ErrNotExist
	}
	return err
}

type davFile struct {
	fs.File
	seeker io.Seeker
	rel    string

	entries []fs.FileInfo
	pos     int
	listed  bool
}

func (f *davFile) Seek(offset int64, whence int) (int64, error) {
	return f.seeker.Seek(offset, whence)
}

func (f *davFile) Write([]byte) (int, error) { return 0, os.ErrPermission }

func (f *davFile) Stat() (fs.FileInfo, error) {
	info, err := f.File.Stat()
	if err != nil {
		return nil, err
	}
	return davFileInfo{info}, nil
}

func (f *davFile) Readdir(count int) ([]fs.FileInfo, error) {
	if !f.listed {
		// Read from the handle that was checked, not by path again.
		dir, ok := f.File.(fs.ReadDirFile)
		if !ok {
			return nil, os.ErrNotExist
		}
		entries, err := dir.ReadDir(-1)
		if err != nil {
			return nil, davErr(err)
		}
		for _, e := range entries {
			if e.Type()&fs.ModeSymlink != 0 || !shelf.IsShelfFile(path.Join(f.rel, e.Name())) {
				continue
			}
			info, err := e.Info()
			if err != nil {
				continue // removed since the listing
			}
			f.entries = append(f.entries, davFileInfo{info})
		}
		f.listed = true
	}
	if count <= 0 {
		rest := f.entries[f.pos:]
		f.pos = len(f.entries)
		return rest, nil
	}
	if f.pos >= len(f.entries) {
		return nil, io.EOF
	}
	end := min(f.pos+count, len(f.entries))
	out := f.entries[f.pos:end]
	f.pos = end
	return out, nil
}

// davFileInfo names the content type from the extension, so PROPFIND does not
// open and sniff every file it lists — a round trip each on a network mount.
type davFileInfo struct {
	fs.FileInfo
}

func (fi davFileInfo) ContentType(context.Context) (string, error) {
	if ct := mime.TypeByExtension(path.Ext(fi.Name())); ct != "" {
		return ct, nil
	}
	return "application/octet-stream", nil
}
