package fsutil

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/voilelab/plainshelf/internal/util"
)

const maxPCloudAPIResponse = 8 << 20

type pCloudMetadata struct {
	Name           string           `json:"name"`
	Size           int64            `json:"size"`
	IsFolder       bool             `json:"isfolder"`
	IsDeleted      bool             `json:"isdeleted"`
	FolderID       int64            `json:"folderid"`
	FileID         int64            `json:"fileid"`
	ParentFolderID int64            `json:"parentfolderid"`
	Modified       pCloudTimestamp  `json:"modified"`
	Created        pCloudTimestamp  `json:"created"`
	Contents       []pCloudMetadata `json:"contents"`
}

type pCloudTimestamp struct {
	time.Time
}

func (t *pCloudTimestamp) UnmarshalJSON(data []byte) error {
	if string(data) == "null" || string(data) == `""` {
		return nil
	}
	var seconds int64
	if err := json.Unmarshal(data, &seconds); err == nil {
		t.Time = time.Unix(seconds, 0)
		return nil
	}
	var value string
	if err := json.Unmarshal(data, &value); err != nil {
		return util.Errorf("pcloud: invalid timestamp %s", string(data))
	}
	for _, layout := range []string{time.RFC1123Z, time.RFC1123} {
		parsed, err := time.Parse(layout, value)
		if err == nil {
			t.Time = parsed
			return nil
		}
	}
	return util.Errorf("pcloud: invalid timestamp %q", value)
}

type pCloudFileInfo struct {
	metadata pCloudMetadata
}

func (i pCloudFileInfo) Name() string       { return i.metadata.Name }
func (i pCloudFileInfo) Size() int64        { return i.metadata.Size }
func (i pCloudFileInfo) ModTime() time.Time { return i.metadata.Modified.Time }
func (i pCloudFileInfo) IsDir() bool        { return i.metadata.IsFolder }
func (i pCloudFileInfo) Sys() any           { return i.metadata }
func (i pCloudFileInfo) Mode() fs.FileMode {
	if i.metadata.IsFolder {
		return fs.ModeDir | 0o755
	}
	return 0o644
}

type pCloudDirEntry struct {
	metadata pCloudMetadata
}

func (e pCloudDirEntry) Name() string               { return e.metadata.Name }
func (e pCloudDirEntry) IsDir() bool                { return e.metadata.IsFolder }
func (e pCloudDirEntry) Type() fs.FileMode          { return pCloudFileInfo{metadata: e.metadata}.Mode().Type() }
func (e pCloudDirEntry) Info() (fs.FileInfo, error) { return pCloudFileInfo{metadata: e.metadata}, nil }

type pCloudFile struct {
	name   string
	info   pCloudFileInfo
	body   io.ReadCloser
	closed bool
}

func (f *pCloudFile) Stat() (fs.FileInfo, error) {
	if f.closed {
		return nil, &fs.PathError{Op: "stat", Path: f.name, Err: util.Errorf("%w", fs.ErrClosed)}
	}
	return f.info, nil
}

func (f *pCloudFile) Read(data []byte) (int, error) {
	if f.closed {
		return 0, &fs.PathError{Op: "read", Path: f.name, Err: util.Errorf("%w", fs.ErrClosed)}
	}
	if f.body == nil {
		return 0, &fs.PathError{Op: "read", Path: f.name, Err: util.Errorf("%w", syscall.EISDIR)}
	}
	return f.body.Read(data)
}

func (f *pCloudFile) Close() error {
	if f.closed {
		return fs.ErrClosed
	}
	f.closed = true
	if f.body != nil {
		if err := f.body.Close(); err != nil {
			return util.Errorf("%w", err)
		}
	}
	return nil
}

// PCloudError is a non-zero result returned by the pCloud API.
type PCloudError struct {
	Result  int    `json:"result"`
	Message string `json:"error"`
}

func (e *PCloudError) Error() string {
	if e.Message == "" {
		return fmt.Sprintf("pcloud API error %d", e.Result)
	}
	return fmt.Sprintf("pcloud API error %d: %s", e.Result, e.Message)
}

func (e *PCloudError) Unwrap() error {
	switch e.Result {
	case 1000, 2000, 2003, 2028:
		return fs.ErrPermission
	case 2002, 2005, 2009:
		return fs.ErrNotExist
	case 2004:
		return fs.ErrExist
	case 2006:
		return syscall.ENOTEMPTY
	case 2008:
		return syscall.ENOSPC
	case 1001, 1002, 1004, 1017, 1037, 2001, 2007, 2010, 2042, 2043:
		return fs.ErrInvalid
	default:
		return nil
	}
}

func (p *PCloudFS) call(method string, values url.Values, output any) error {
	values = cloneValues(values)
	values.Set("access_token", p.accessToken)
	values.Set("timeformat", "timestamp")

	requestURL := *p.apiURL
	requestURL.Path = strings.TrimRight(requestURL.Path, "/") + "/" + method
	req, err := http.NewRequest(http.MethodPost, requestURL.String(), strings.NewReader(values.Encode()))
	if err != nil {
		return util.Errorf("pcloud %s: build request: %w", method, err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "plainshelf-pcloudfs")

	resp, err := p.client.Do(req)
	if err != nil {
		return util.Errorf("pcloud %s: request failed: %w", method, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return util.Errorf("pcloud: unexpected HTTP response: %s", resp.Status)
	}
	return decodePCloudResponse(resp.Body, output)
}

func decodePCloudResponse(reader io.Reader, output any) error {
	data, err := io.ReadAll(io.LimitReader(reader, maxPCloudAPIResponse+1))
	if err != nil {
		return util.Errorf("pcloud: read API response: %w", err)
	}
	if len(data) > maxPCloudAPIResponse {
		return util.Errorf("pcloud: API response exceeds %d bytes", maxPCloudAPIResponse)
	}
	var apiErr PCloudError
	if err := json.Unmarshal(data, &apiErr); err != nil {
		return util.Errorf("pcloud: decode API response: %w", err)
	}
	if apiErr.Result != 0 {
		return &apiErr
	}
	if output != nil {
		if err := json.Unmarshal(data, output); err != nil {
			return util.Errorf("pcloud: decode API response: %w", err)
		}
	}
	return nil
}

func cloneValues(values url.Values) url.Values {
	cloned := make(url.Values, len(values)+2)
	for key, entries := range values {
		cloned[key] = append([]string(nil), entries...)
	}
	return cloned
}

func (p *PCloudFS) openFile(fileID int64) (io.ReadCloser, error) {
	var link struct {
		Hosts []string `json:"hosts"`
		Path  string   `json:"path"`
	}
	if err := p.call("getfilelink", url.Values{
		"fileid":        {strconv.FormatInt(fileID, 10)},
		"forcedownload": {"1"},
	}, &link); err != nil {
		return nil, err
	}
	if len(link.Hosts) == 0 || link.Hosts[0] == "" {
		return nil, util.NewError("pcloud: getfilelink returned no download host")
	}
	if !strings.HasPrefix(link.Path, "/") {
		return nil, util.NewError("pcloud: getfilelink returned an invalid download path")
	}
	requestPath, err := url.ParseRequestURI(link.Path)
	if err != nil || requestPath.Host != "" {
		return nil, util.NewError("pcloud: getfilelink returned an invalid download path")
	}
	downloadURL := &url.URL{
		Scheme:   "https",
		Host:     link.Hosts[0],
		Path:     requestPath.Path,
		RawPath:  requestPath.RawPath,
		RawQuery: requestPath.RawQuery,
	}
	// Local and private test endpoints commonly do not have TLS. Production
	// pCloud API endpoints always use HTTPS.
	if p.apiURL.Scheme == "http" {
		downloadURL.Scheme = "http"
	}

	req, err := http.NewRequest(http.MethodGet, downloadURL.String(), nil)
	if err != nil {
		return nil, util.Errorf("pcloud download: build request: %w", err)
	}
	req.Header.Set("User-Agent", "plainshelf-pcloudfs")
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, util.Errorf("pcloud download: request failed: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		resp.Body.Close()
		return nil, util.Errorf("pcloud: unexpected HTTP response: %s", resp.Status)
	}
	return resp.Body, nil
}
