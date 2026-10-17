// Package davfs reads a shelf over WebDAV, as an fsutil.ReadFS.
//
// It speaks the read subset only — PROPFIND at Depth 0 and 1, and GET — so the
// same client opens another PlainShelf's export and any general WebDAV server.
// The credential travels in the Authorization header and never in the URL, and
// nothing here can send a write.
package davfs

import (
	"bytes"
	"context"
	"encoding/xml"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"path"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/voilelab/plainshelf/internal/fsutil"
	"github.com/voilelab/plainshelf/internal/util"
)

// Metadata fails fast, a transfer may be slow.
const (
	requestTimeout  = 30 * time.Second
	downloadTimeout = 5 * time.Minute
	maxAttempts     = 3
	retryBaseDelay  = 500 * time.Millisecond

	// maxPropfindBytes bounds a PROPFIND reply; a Depth 1 listing of a large
	// folder is the biggest one.
	maxPropfindBytes = 64 << 20
)

// Secret returns the credential to send. It is asked once at first use and
// again after a 401, so a rotated token is picked up without reopening.
type Secret func(ctx context.Context) (string, error)

// Options configures an FS.
type Options struct {
	// Username, when set, sends the secret as HTTP Basic; empty sends it as a
	// bearer token.
	Username string

	// Secret is nil for a server that needs no credential.
	Secret Secret

	// HTTPClient is a default client when nil; the time budgets are applied
	// per request either way.
	HTTPClient *http.Client
}

// FS is a WebDAV collection as an fsutil.ReadFS. Every call is a request:
// caching listings is the shelf's job, which reads a remote shelf from its
// exported book cache rather than walking it.
type FS struct {
	base     *url.URL
	username string
	secret   Secret
	http     *http.Client

	// A variable so tests run without real backoff.
	retryDelay time.Duration

	mu         sync.Mutex
	credential string
	resolved   bool
}

var _ fsutil.ReadFS = (*FS)(nil)

// ParseURL checks that raw can name a shelf directory and returns it
// normalized: http or https, lower-case host, a path ending in "/", and no
// credentials, query or fragment — a URL is logged and copied, a secret is not.
func ParseURL(raw string) (*url.URL, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return nil, util.Errorf("invalid WebDAV URL: %w", err)
	}
	u.Scheme = strings.ToLower(u.Scheme)
	switch {
	case u.Scheme != "http" && u.Scheme != "https":
		return nil, util.Errorf("WebDAV URL %q must be http or https", raw)
	case u.Host == "":
		return nil, util.Errorf("WebDAV URL %q has no host", raw)
	case u.User != nil:
		return nil, util.NewError("WebDAV URL must not carry credentials; use credential instead")
	case u.RawQuery != "" || u.ForceQuery || u.Fragment != "":
		return nil, util.Errorf("WebDAV URL %q must not have a query or fragment", raw)
	}
	u.Host = strings.ToLower(u.Host)
	if !strings.HasSuffix(u.Path, "/") {
		u.Path += "/"
		if u.RawPath != "" {
			u.RawPath += "/"
		}
	}
	return u, nil
}

// New returns an FS reading the collection at rawURL. Nothing is requested
// until first use.
func New(rawURL string, opts Options) (*FS, error) {
	base, err := ParseURL(rawURL)
	if err != nil {
		return nil, err
	}
	httpClient := opts.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{}
	}
	return &FS{
		base:       base,
		username:   opts.Username,
		secret:     opts.Secret,
		http:       httpClient,
		retryDelay: retryBaseDelay,
	}, nil
}

// target is the URL of name; dir adds the trailing slash a collection is
// addressed by, which saves a redirect on servers that insist on it.
func (f *FS) target(name string, dir bool) *url.URL {
	escaped := f.base.EscapedPath()
	if name != "." {
		segments := strings.Split(name, "/")
		for i, s := range segments {
			segments[i] = url.PathEscape(s)
		}
		escaped += strings.Join(segments, "/")
		if dir {
			escaped += "/"
		}
	}
	u := *f.base
	u.RawPath = escaped
	u.Path, _ = url.PathUnescape(escaped) // escaped above, cannot fail
	return &u
}

func (f *FS) authorization(ctx context.Context, refresh bool) (string, error) {
	if f.secret == nil {
		return "", nil
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if !f.resolved || refresh {
		secret, err := f.secret(ctx)
		if err != nil {
			return "", util.Errorf("resolve WebDAV credential: %w", err)
		}
		f.credential, f.resolved = strings.TrimSpace(secret), true
	}
	if f.username != "" {
		req := http.Request{Header: http.Header{}}
		req.SetBasicAuth(f.username, f.credential)
		return req.Header.Get("Authorization"), nil
	}
	return "Bearer " + f.credential, nil
}

// do sends one request and returns a 2xx reply. Transient failures are retried,
// and a 401 once more after the credential is resolved again, if it changed.
// Any other reply is an *Error with its body closed.
func (f *FS) do(ctx context.Context, op, name, method string, target *url.URL, header http.Header, body []byte) (*http.Response, error) {
	refreshed := false
	for attempt := 1; ; attempt++ {
		auth, err := f.authorization(ctx, false)
		if err != nil {
			return nil, &Error{Op: op, Path: name, Status: http.StatusUnauthorized, Message: err.Error()}
		}
		res, err := f.send(ctx, method, target, header, body, auth)
		if err != nil {
			err = newTransportError(op, name, err)
		} else {
			if res.StatusCode/100 == 2 {
				return res, nil
			}
			_ = res.Body.Close()
			err = &Error{Op: op, Path: name, Status: res.StatusCode, Message: res.Status}

			if res.StatusCode == http.StatusUnauthorized && f.secret != nil && !refreshed {
				refreshed = true
				fresh, rerr := f.authorization(ctx, true)
				if rerr == nil && fresh != auth {
					attempt--
					continue
				}
			}
		}
		if attempt == maxAttempts || !isRetryable(err) {
			return nil, err
		}
		if serr := sleep(ctx, f.retryDelay<<(attempt-1)); serr != nil {
			return nil, newTransportError(op, name, serr)
		}
	}
}

func (f *FS) send(ctx context.Context, method string, target *url.URL, header http.Header, body []byte, auth string) (*http.Response, error) {
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, target.String(), reader)
	if err != nil {
		return nil, err
	}
	for key, values := range header {
		req.Header[key] = values
	}
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}
	return f.http.Do(req)
}

const propfindBody = `<?xml version="1.0" encoding="utf-8"?>` +
	`<D:propfind xmlns:D="DAV:"><D:prop><D:resourcetype/><D:getcontentlength/><D:getlastmodified/></D:prop></D:propfind>`

type multistatus struct {
	XMLName   xml.Name      `xml:"DAV: multistatus"`
	Responses []davResponse `xml:"DAV: response"`
}

type davResponse struct {
	Href      string        `xml:"DAV: href"`
	Propstats []davPropstat `xml:"DAV: propstat"`
}

type davPropstat struct {
	Status string `xml:"DAV: status"`
	Prop   struct {
		ResourceType struct {
			Collection *struct{} `xml:"DAV: collection"`
		} `xml:"DAV: resourcetype"`
		ContentLength string `xml:"DAV: getcontentlength"`
		LastModified  string `xml:"DAV: getlastmodified"`
	} `xml:"DAV: prop"`
}

// propfind lists name at depth "0" or "1" and returns name itself and, at
// depth 1, its children sorted by name.
func (f *FS) propfind(op, name, depth string) (fileInfo, []fs.DirEntry, error) {
	if !fs.ValidPath(name) {
		return fileInfo{}, nil, &fs.PathError{Op: op, Path: name, Err: fs.ErrInvalid}
	}
	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()

	target := f.target(name, depth == "1")
	header := http.Header{"Depth": {depth}, "Content-Type": {"application/xml; charset=utf-8"}}
	res, err := f.do(ctx, op, name, "PROPFIND", target, header, []byte(propfindBody))
	if err != nil {
		return fileInfo{}, nil, err
	}
	defer res.Body.Close()

	// A 200 here came from something that is not a WebDAV server — a captive
	// portal, a web page — and must not read as an empty folder.
	if res.StatusCode != http.StatusMultiStatus {
		return fileInfo{}, nil, &Error{Op: op, Path: name, Status: res.StatusCode, Message: "reply is not a WebDAV multistatus"}
	}
	var reply multistatus
	if err := xml.NewDecoder(io.LimitReader(res.Body, maxPropfindBytes)).Decode(&reply); err != nil {
		return fileInfo{}, nil, &Error{Op: op, Path: name, Message: "unreadable PROPFIND reply: " + err.Error(), cause: err}
	}

	self := strings.TrimSuffix(target.Path, "/")
	var found bool
	var info fileInfo
	var children []fs.DirEntry
	for _, r := range reply.Responses {
		href, err := url.Parse(r.Href)
		if err != nil {
			continue
		}
		hrefPath := strings.TrimSuffix(href.Path, "/")
		entry, ok := propInfo(path.Base(hrefPath), r.Propstats)
		if !ok {
			continue
		}
		switch {
		case hrefPath == self:
			entry.name = path.Base(name)
			info, found = entry, true
		case depth == "1" && path.Dir(hrefPath) == self:
			children = append(children, fs.FileInfoToDirEntry(entry))
		}
	}
	if !found {
		return fileInfo{}, nil, &Error{Op: op, Path: name, Status: res.StatusCode, Message: "PROPFIND reply does not describe the requested path"}
	}
	slices.SortFunc(children, func(a, b fs.DirEntry) int { return strings.Compare(a.Name(), b.Name()) })
	return info, children, nil
}

// propInfo reads the properties a 200 propstat carries; ok is false when none
// did, such as an entry the server reports but cannot describe.
func propInfo(name string, propstats []davPropstat) (fileInfo, bool) {
	info := fileInfo{name: name}
	ok := false
	for _, ps := range propstats {
		if fields := strings.Fields(ps.Status); len(fields) < 2 || fields[1] != "200" {
			continue
		}
		ok = true
		p := ps.Prop
		if p.ResourceType.Collection != nil {
			info.dir = true
		}
		if n, err := strconv.ParseInt(strings.TrimSpace(p.ContentLength), 10, 64); err == nil {
			info.size = n
		}
		// getlastmodified is an HTTP date, so whole seconds; the shelf already
		// tolerates that for the exported book cache it re-reads.
		if t, err := http.ParseTime(strings.TrimSpace(p.LastModified)); err == nil {
			info.modTime = t
		}
	}
	return info, ok
}

func (f *FS) Stat(name string) (fs.FileInfo, error) {
	info, _, err := f.propfind("stat", name, "0")
	if err != nil {
		return nil, err
	}
	return info, nil
}

func (f *FS) ReadDir(name string) ([]fs.DirEntry, error) {
	info, children, err := f.propfind("readdir", name, "1")
	if err != nil {
		return nil, err
	}
	if !info.dir {
		return nil, &fs.PathError{Op: "readdir", Path: name, Err: util.NewError("not a directory")}
	}
	if children == nil {
		children = []fs.DirEntry{}
	}
	return children, nil
}

// Open starts a GET of name. The file is seekable: a seek closes the transfer
// and the next read asks for the rest with a Range request.
func (f *FS) Open(name string) (fs.File, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrInvalid}
	}
	file := &file{fs: f, name: name, info: fileInfo{name: path.Base(name), size: -1}}
	if err := file.start(); err != nil {
		return nil, err
	}
	return file, nil
}

type fileInfo struct {
	name    string
	dir     bool
	size    int64
	modTime time.Time
}

func (i fileInfo) Name() string       { return i.name }
func (i fileInfo) Size() int64        { return i.size }
func (i fileInfo) ModTime() time.Time { return i.modTime }
func (i fileInfo) IsDir() bool        { return i.dir }
func (i fileInfo) Sys() any           { return nil }
func (i fileInfo) Mode() fs.FileMode {
	if i.dir {
		return fs.ModeDir | 0o555
	}
	return 0o444
}

type file struct {
	fs   *FS
	name string
	info fileInfo

	// body is nil after a seek, until the next read.
	body   io.ReadCloser
	cancel context.CancelFunc
	offset int64
}

// start requests the file from offset on. The download budget lasts until
// the transfer is closed.
func (f *file) start() error {
	ctx, cancel := context.WithTimeout(context.Background(), downloadTimeout)
	header := http.Header{}
	if f.offset > 0 {
		header.Set("Range", "bytes="+strconv.FormatInt(f.offset, 10)+"-")
	}
	res, err := f.fs.do(ctx, "open", f.name, http.MethodGet, f.fs.target(f.name, false), header, nil)
	if err != nil {
		cancel()
		return err
	}

	var skip int64
	switch {
	case res.StatusCode == http.StatusPartialContent:
	case f.offset > 0:
		// The server ignored Range and sent the whole file.
		skip = f.offset
	}
	if f.offset == 0 {
		f.info.size = res.ContentLength
		if t, err := http.ParseTime(res.Header.Get("Last-Modified")); err == nil {
			f.info.modTime = t
		}
	}
	if skip > 0 {
		if _, err := io.CopyN(io.Discard, res.Body, skip); err != nil {
			_ = res.Body.Close()
			cancel()
			return newTransportError("open", f.name, err)
		}
	}
	f.body, f.cancel = res.Body, cancel
	return nil
}

func (f *file) Stat() (fs.FileInfo, error) { return f.info, nil }

func (f *file) Read(p []byte) (int, error) {
	if f.body == nil {
		if f.info.size >= 0 && f.offset >= f.info.size {
			return 0, io.EOF
		}
		if err := f.start(); err != nil {
			return 0, err
		}
	}
	n, err := f.body.Read(p)
	f.offset += int64(n)
	if err != nil && err != io.EOF {
		err = newTransportError("read", f.name, err)
	}
	return n, err
}

func (f *file) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case io.SeekCurrent:
		offset += f.offset
	case io.SeekEnd:
		if f.info.size < 0 {
			return 0, &fs.PathError{Op: "seek", Path: f.name, Err: util.NewError("size unknown")}
		}
		offset += f.info.size
	}
	if offset < 0 {
		return 0, &fs.PathError{Op: "seek", Path: f.name, Err: fs.ErrInvalid}
	}
	if offset != f.offset {
		f.closeBody()
		f.offset = offset
	}
	return offset, nil
}

func (f *file) closeBody() {
	if f.body != nil {
		_ = f.body.Close()
		f.cancel()
		f.body, f.cancel = nil, nil
	}
}

func (f *file) Close() error {
	f.closeBody()
	return nil
}

func sleep(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
