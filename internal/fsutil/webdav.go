package fsutil

import (
	"bytes"
	"io"
	"io/fs"
	"net"
	"net/url"
	"path"
	"strconv"
	"strings"

	"github.com/studio-b12/gowebdav"
	"github.com/voilelab/plainshelf/internal/util"
)

var _ FS = (*WebDAVFS)(nil)

var _ fs.File = (*webDAVFile)(nil)
var _ fs.DirEntry = (*webDAVDirEntry)(nil)

type WebDAVFS struct {
	client  *gowebdav.Client
	baseDir string
}

type WebDAVConf struct {
	Host     string
	Port     int
	User     string
	Password string
	BaseDir  string
}

func NewWebDAVFS(conf *WebDAVConf) (*WebDAVFS, error) {
	if conf == nil {
		return nil, util.Errorf("nil webdav config")
	}

	endpoint, err := buildWebDAVEndpoint(conf.Host, conf.Port)
	if err != nil {
		return nil, util.Errorf("build endpoint: %w", err)
	}

	client := gowebdav.NewClient(endpoint, conf.User, conf.Password)
	if err := client.Connect(); err != nil {
		return nil, util.Errorf("connect webdav: %w", err)
	}

	return &WebDAVFS{
		client:  client,
		baseDir: cleanWebDAVBaseDir(conf.BaseDir),
	}, nil
}

func (w *WebDAVFS) resolve(name string) string {
	cleanName := path.Clean("/" + strings.TrimPrefix(name, "/"))
	if cleanName == "/" {
		return w.baseDir
	}

	if w.baseDir == "/" {
		return cleanName
	}

	return path.Join(w.baseDir, strings.TrimPrefix(cleanName, "/"))
}

func (w *WebDAVFS) Open(name string) (fs.File, error) {
	fullPath := w.resolve(name)
	rc, err := w.client.ReadStream(fullPath)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	return &webDAVFile{
		client:   w.client,
		fullPath: fullPath,
		rc:       rc,
	}, nil
}

func (w *WebDAVFS) ReadDir(name string) ([]fs.DirEntry, error) {
	infos, err := w.client.ReadDir(w.resolve(name))
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	entries := make([]fs.DirEntry, len(infos))
	for i := range infos {
		entries[i] = &webDAVDirEntry{info: infos[i]}
	}

	return entries, nil
}

func (w *WebDAVFS) Stat(name string) (fs.FileInfo, error) {
	info, err := w.client.Stat(w.resolve(name))
	if err != nil {
		return nil, util.Errorf("%w", err)
	}
	return info, nil
}

func (w *WebDAVFS) OpenWriter(name string) (io.WriteCloser, error) {
	return &webDAVWriteCloser{
		fullPath: w.resolve(name),
		client:   w.client,
	}, nil
}

func (w *WebDAVFS) Mkdir(name string) error {
	if err := w.client.Mkdir(w.resolve(name), 0755); err != nil {
		return util.Errorf("%w", err)
	}
	return nil
}

func (w *WebDAVFS) MkdirAll(pth string) error {
	if err := w.client.MkdirAll(w.resolve(pth), 0755); err != nil {
		return util.Errorf("%w", err)
	}
	return nil
}

func (w *WebDAVFS) Rename(oldPath, newPath string) error {
	if err := w.client.Rename(w.resolve(oldPath), w.resolve(newPath), true); err != nil {
		return util.Errorf("%w", err)
	}
	return nil
}

func (w *WebDAVFS) Remove(name string) error {
	if err := w.client.Remove(w.resolve(name)); err != nil {
		return util.Errorf("%w", err)
	}
	return nil
}

func (w *WebDAVFS) RemoveAll(name string) error {
	if err := w.client.RemoveAll(w.resolve(name)); err != nil {
		return util.Errorf("%w", err)
	}
	return nil
}

type webDAVFile struct {
	client   *gowebdav.Client
	fullPath string
	rc       io.ReadCloser
}

func (f *webDAVFile) Stat() (fs.FileInfo, error) {
	info, err := f.client.Stat(f.fullPath)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}
	return info, nil
}

func (f *webDAVFile) Read(p []byte) (int, error) {
	n, err := f.rc.Read(p)
	if err != nil {
		if err == io.EOF {
			return n, io.EOF
		}
		return n, util.Errorf("%w", err)
	}
	return n, nil
}

func (f *webDAVFile) Close() error {
	if err := f.rc.Close(); err != nil {
		return util.Errorf("%w", err)
	}
	return nil
}

type webDAVDirEntry struct {
	info fs.FileInfo
}

func (d *webDAVDirEntry) Name() string {
	return d.info.Name()
}

func (d *webDAVDirEntry) IsDir() bool {
	return d.info.IsDir()
}

func (d *webDAVDirEntry) Type() fs.FileMode {
	return d.info.Mode().Type()
}

func (d *webDAVDirEntry) Info() (fs.FileInfo, error) {
	return d.info, nil
}

type webDAVWriteCloser struct {
	fullPath string
	client   *gowebdav.Client
	buf      bytes.Buffer
	closed   bool
}

func (w *webDAVWriteCloser) Write(p []byte) (int, error) {
	if w.closed {
		return 0, util.Errorf("write on closed writer")
	}
	return w.buf.Write(p)
}

func (w *webDAVWriteCloser) Close() error {
	if w.closed {
		return nil
	}
	w.closed = true

	if err := w.client.Write(w.fullPath, w.buf.Bytes(), 0644); err != nil {
		return util.Errorf("%w", err)
	}

	return nil
}

func cleanWebDAVBaseDir(baseDir string) string {
	if strings.TrimSpace(baseDir) == "" {
		return "/"
	}

	clean := path.Clean("/" + strings.TrimPrefix(baseDir, "/"))
	if clean == "." {
		return "/"
	}
	return clean
}

func buildWebDAVEndpoint(host string, port int) (string, error) {
	host = strings.TrimSpace(host)
	if host == "" {
		return "", util.Errorf("host is required")
	}

	raw := host
	if !strings.Contains(raw, "://") {
		raw = "http://" + raw
	}

	u, err := url.Parse(raw)
	if err != nil {
		return "", util.Errorf("%w", err)
	}

	if u.Host == "" {
		return "", util.Errorf("invalid host: %q", host)
	}

	if port > 0 && u.Port() == "" {
		u.Host = net.JoinHostPort(u.Hostname(), strconv.Itoa(port))
	}

	u.Path = ""
	u.RawPath = ""
	u.RawQuery = ""
	u.Fragment = ""

	return strings.TrimRight(u.String(), "/"), nil
}
