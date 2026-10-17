// Package pcloudtest is a fake pCloud API serving a local directory, for tests
// that read a shelf through pcloud.FS.
package pcloudtest

import (
	"archive/zip"
	"encoding/json/v2"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/voilelab/plainshelf/internal/jsonopt"
)

// Token is the only access token the server accepts.
const Token = "pcloudtest-token"

// Server serves dir as the folder at Mount in an otherwise empty account.
// Folder and file IDs are assigned by path on first sight and stay stable.
type Server struct {
	*httptest.Server

	dir   string
	mount string

	mu       sync.Mutex
	ids      map[string]int64
	paths    map[int64]string
	next     int64
	requests []string
	failures []int

	refuseRecursive bool
	offline         bool
	tokenLeaked     bool
}

// New starts a server for dir, mounted at mount (for example "/Books/Shelf"),
// which must not be the account root.
func New(t testing.TB, dir, mount string) *Server {
	t.Helper()
	s := &Server{
		dir:   dir,
		mount: "/" + strings.Trim(mount, "/"),
		ids:   map[string]int64{"/": 0},
		paths: map[int64]string{0: "/"},
		next:  1000,
	}
	if s.mount == "/" {
		t.Fatal("pcloudtest: mount must name a folder")
	}
	s.Server = httptest.NewTLSServer(http.HandlerFunc(s.serve))
	t.Cleanup(s.Close)
	return s
}

// Host is the API host to hand pcloud.NewClient along with Client().
func (s *Server) Host() string { return APIHost }

// APIHost is a real region host: pcloud.NewClient refuses any other.
const APIHost = "api.pcloud.com"

// downloadHost is what getfilelink and getziplink name, as pCloud's own
// content hosts differ from the API host.
const downloadHost = "c1.pcloud.com"

// Client sends every request, whatever its host, to this server.
func (s *Server) Client() *http.Client {
	inner := s.Server.Client()
	return &http.Client{Transport: redirect{to: strings.TrimPrefix(s.URL, "https://"), next: inner.Transport}}
}

type redirect struct {
	to   string
	next http.RoundTripper
}

func (r redirect) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	req.URL.Host = r.to
	return r.next.RoundTrip(req)
}

// RefuseRecursive makes a recursive listfolder answer result 1101.
func (s *Server) RefuseRecursive() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.refuseRecursive = true
}

// SetOffline closes every connection without answering while set.
func (s *Server) SetOffline(offline bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.offline = offline
}

// TokenLeaked reports whether the token reached a download or a query string.
func (s *Server) TokenLeaked() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.tokenLeaked
}

// FailNext answers the next calls with these HTTP statuses, one per call.
func (s *Server) FailNext(statuses ...int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.failures = append(s.failures, statuses...)
}

// Take returns the requests served so far, as "method" or "download", and
// starts a new count.
func (s *Server) Take() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := s.requests
	s.requests = nil
	return out
}

func (s *Server) serve(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	if s.offline {
		s.mu.Unlock()
		if hj, ok := w.(http.Hijacker); ok {
			if conn, _, err := hj.Hijack(); err == nil {
				conn.Close()
				return
			}
		}
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	method := strings.TrimPrefix(r.URL.Path, "/")
	if strings.HasPrefix(method, "dl/") || strings.HasPrefix(method, "zip/") {
		method = "download"
	}
	s.requests = append(s.requests, method)
	if strings.Contains(r.URL.RawQuery, Token) || method == "download" && r.Header.Get("Authorization") != "" {
		s.tokenLeaked = true
	}
	if len(s.failures) > 0 {
		status := s.failures[0]
		s.failures = s.failures[1:]
		s.mu.Unlock()
		w.WriteHeader(status)
		return
	}
	refuse := s.refuseRecursive
	s.mu.Unlock()

	if method == "download" {
		s.serveDownload(w, r)
		return
	}
	if r.Header.Get("Authorization") != "Bearer "+Token {
		reply(w, map[string]any{"result": 2094, "error": "Invalid 'access_token' provided."})
		return
	}

	q := r.URL.Query()
	switch method {
	case "listfolder":
		recursive := q.Get("recursive") == "1"
		if recursive && refuse {
			reply(w, map[string]any{"result": 1101, "error": "Recursive listing is not allowed."})
			return
		}
		p, ok := s.pathOf(q.Get("folderid"))
		if !ok {
			reply(w, map[string]any{"result": 2005, "error": "Directory does not exist."})
			return
		}
		meta, ok := s.item(p, true, recursive)
		if !ok {
			reply(w, map[string]any{"result": 2005, "error": "Directory does not exist."})
			return
		}
		reply(w, map[string]any{"result": 0, "metadata": meta})
	case "getfilelink":
		p, ok := s.pathOf(q.Get("fileid"))
		if info, err := os.Stat(s.local(p)); !ok || err != nil || info.IsDir() {
			reply(w, map[string]any{"result": 2009, "error": "File not found."})
			return
		}
		reply(w, map[string]any{"result": 0, "hosts": []string{downloadHost}, "path": "/dl/" + q.Get("fileid") + "/" + path.Base(p)})
	case "getziplink":
		reply(w, map[string]any{"result": 0, "hosts": []string{downloadHost}, "path": "/zip/" + q.Get("fileids")})
	default:
		reply(w, map[string]any{"result": 2000, "error": "Unknown method."})
	}
}

func (s *Server) serveDownload(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/"), "/")
	if len(parts) < 2 {
		http.NotFound(w, r)
		return
	}
	if parts[0] == "dl" {
		p, ok := s.pathOf(parts[1])
		if !ok {
			http.NotFound(w, r)
			return
		}
		http.ServeFile(w, r, s.local(p))
		return
	}
	zw := zip.NewWriter(w)
	for id := range strings.SplitSeq(parts[1], ",") {
		p, ok := s.pathOf(id)
		if !ok {
			continue
		}
		data, err := os.ReadFile(s.local(p))
		if err != nil {
			continue
		}
		out, err := zw.Create(path.Base(p))
		if err == nil {
			_, _ = out.Write(data)
		}
	}
	_ = zw.Close()
}

func reply(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.MarshalWrite(w, v, jsonopt.API())
}

func (s *Server) pathOf(id string) (string, bool) {
	n, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return "", false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.paths[n]
	return p, ok
}

func (s *Server) idOf(p string) int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	if id, ok := s.ids[p]; ok {
		return id
	}
	s.next++
	s.ids[p], s.paths[s.next] = s.next, p
	return s.next
}

// local maps an account path at or under the mount to dir; "" otherwise.
func (s *Server) local(p string) string {
	if p == s.mount {
		return s.dir
	}
	rel, ok := strings.CutPrefix(p, s.mount+"/")
	if !ok {
		return ""
	}
	return filepath.Join(s.dir, filepath.FromSlash(rel))
}

// item describes the account path p; withContents lists one level, and
// recursive every level.
func (s *Server) item(p string, withContents, recursive bool) (map[string]any, bool) {
	name := path.Base(p)
	if p == "/" {
		name = "/"
	}

	// An ancestor of the mount holds only the next folder towards it.
	if p == "/" || strings.HasPrefix(s.mount, p+"/") {
		meta := map[string]any{"name": name, "isfolder": true, "folderid": s.idOf(p)}
		if withContents {
			next, _, _ := strings.Cut(strings.TrimPrefix(s.mount, strings.TrimSuffix(p, "/")+"/"), "/")
			child, ok := s.item(path.Join(p, next), recursive, recursive)
			if !ok {
				return nil, false
			}
			meta["contents"] = []any{child}
		}
		return meta, true
	}

	local := s.local(p)
	if local == "" {
		return nil, false
	}
	info, err := os.Stat(local)
	if err != nil {
		return nil, false
	}
	meta := map[string]any{
		"name":     name,
		"isfolder": info.IsDir(),
		"modified": info.ModTime().UTC().Format(time.RFC1123Z),
	}
	if !info.IsDir() {
		meta["fileid"] = s.idOf(p)
		meta["size"] = info.Size()
		return meta, true
	}
	meta["folderid"] = s.idOf(p)
	if !withContents {
		return meta, true
	}
	entries, err := os.ReadDir(local)
	if err != nil {
		return nil, false
	}
	contents := make([]any, 0, len(entries))
	for _, entry := range entries {
		if entry.Type()&fs.ModeSymlink != 0 {
			continue
		}
		child, ok := s.item(path.Join(p, entry.Name()), recursive, recursive)
		if ok {
			contents = append(contents, child)
		}
	}
	meta["contents"] = contents
	return meta, true
}
