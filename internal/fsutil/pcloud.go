package fsutil

import (
	"errors"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"syscall"

	"github.com/voilelab/plainshelf/internal/util"
)

const (
	// PCloudUSAPIURL is the API endpoint used by pCloud's US data region.
	PCloudUSAPIURL = "https://api.pcloud.com"
	// PCloudEUAPIURL is the API endpoint used by pCloud's EU data region.
	PCloudEUAPIURL = "https://eapi.pcloud.com"
)

// PCloudConfig configures a PCloudFS.
//
// AccessToken is the OAuth access token returned by pCloud. APIURL must match
// the data region that issued the token. RootFolderID scopes the filesystem to
// one pCloud folder; zero is the account root.
type PCloudConfig struct {
	AccessToken  string
	APIURL       string
	RootFolderID int64
	HTTPClient   *http.Client
}

// PCloudFS exposes a pCloud folder through the common FS interface.
//
// Paths are resolved one folder at a time using pCloud file/folder IDs. This
// keeps the configured root isolated and avoids relying on pCloud's discouraged
// path parameters. PCloudFS is read-only; every mutation returns
// errors.ErrUnsupported.
type PCloudFS struct {
	accessToken  string
	apiURL       *url.URL
	rootFolderID int64
	client       *http.Client
}

var _ FS = (*PCloudFS)(nil)

// NewPCloudFS creates a filesystem rooted at the account root in pCloud's US
// data region. Use NewPCloudFSWithConfig for EU accounts, a subfolder root, or
// a custom HTTP client.
func NewPCloudFS(accessToken string) *PCloudFS {
	fsys, _ := NewPCloudFSWithConfig(PCloudConfig{AccessToken: accessToken})
	return fsys
}

// NewPCloudFSWithConfig creates a configured pCloud filesystem.
func NewPCloudFSWithConfig(conf PCloudConfig) (*PCloudFS, error) {
	if conf.RootFolderID < 0 {
		return nil, util.NewError("pcloud: root folder ID must not be negative")
	}
	if conf.APIURL == "" {
		conf.APIURL = PCloudUSAPIURL
	}

	apiURL, err := url.Parse(conf.APIURL)
	if err != nil || (apiURL.Scheme != "http" && apiURL.Scheme != "https") || apiURL.Host == "" {
		return nil, util.Errorf("pcloud: invalid API URL %q", conf.APIURL)
	}
	apiURL.Path = strings.TrimRight(apiURL.Path, "/")
	apiURL.RawQuery = ""
	apiURL.Fragment = ""

	client := conf.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}

	return &PCloudFS{
		accessToken:  conf.AccessToken,
		apiURL:       apiURL,
		rootFolderID: conf.RootFolderID,
		client:       client,
	}, nil
}

// Open opens name for reading. File data is streamed from the temporary
// download URL returned by pCloud.
func (p *PCloudFS) Open(name string) (fs.File, error) {
	if err := validPCloudPath("open", name, true); err != nil {
		return nil, err
	}

	meta, err := p.lookup(name)
	if err != nil {
		return nil, p.pathError("open", name, err)
	}
	if meta.IsFolder {
		return &pCloudFile{name: name, info: pCloudFileInfo{metadata: meta}}, nil
	}

	body, err := p.openFile(meta.FileID)
	if err != nil {
		return nil, p.pathError("open", name, err)
	}
	return &pCloudFile{name: name, info: pCloudFileInfo{metadata: meta}, body: body}, nil
}

// ReadDir reads and sorts the direct children of name.
func (p *PCloudFS) ReadDir(name string) ([]fs.DirEntry, error) {
	if err := validPCloudPath("readdir", name, true); err != nil {
		return nil, err
	}

	meta, err := p.lookup(name)
	if err != nil {
		return nil, p.pathError("readdir", name, err)
	}
	if !meta.IsFolder {
		return nil, p.pathError("readdir", name, syscall.ENOTDIR)
	}

	meta, err = p.listFolder(meta.FolderID)
	if err != nil {
		return nil, p.pathError("readdir", name, err)
	}
	entries := make([]fs.DirEntry, 0, len(meta.Contents))
	for i := range meta.Contents {
		entries = append(entries, pCloudDirEntry{metadata: meta.Contents[i]})
	}
	sortDirEntries(entries)
	return entries, nil
}

// Stat returns metadata for name.
func (p *PCloudFS) Stat(name string) (fs.FileInfo, error) {
	if err := validPCloudPath("stat", name, true); err != nil {
		return nil, err
	}

	meta, err := p.lookup(name)
	if err != nil {
		return nil, p.pathError("stat", name, err)
	}
	return pCloudFileInfo{metadata: meta}, nil
}

// OpenWriter is not supported because PCloudFS is read-only.
func (p *PCloudFS) OpenWriter(name string) (io.WriteCloser, error) {
	return nil, p.pathError("openwriter", name, errors.ErrUnsupported)
}

// WriteFile is not supported because PCloudFS is read-only.
func (p *PCloudFS) WriteFile(name string, data []byte) error {
	return p.pathError("write", name, errors.ErrUnsupported)
}

// Mkdir is not supported because PCloudFS is read-only.
func (p *PCloudFS) Mkdir(name string) error {
	return p.pathError("mkdir", name, errors.ErrUnsupported)
}

// MkdirAll is not supported because PCloudFS is read-only.
func (p *PCloudFS) MkdirAll(name string) error {
	return p.pathError("mkdir", name, errors.ErrUnsupported)
}

// Rename is not supported because PCloudFS is read-only.
func (p *PCloudFS) Rename(oldPath, newPath string) error {
	return p.pathError("rename", oldPath, errors.ErrUnsupported)
}

// Remove is not supported because PCloudFS is read-only.
func (p *PCloudFS) Remove(name string) error {
	return p.pathError("remove", name, errors.ErrUnsupported)
}

// RemoveAll is not supported because PCloudFS is read-only.
func (p *PCloudFS) RemoveAll(name string) error {
	return p.pathError("removeall", name, errors.ErrUnsupported)
}

func (p *PCloudFS) lookup(name string) (pCloudMetadata, error) {
	if name == "." {
		meta, err := p.listFolder(p.rootFolderID)
		if err == nil {
			meta.Name = "."
		}
		return meta, err
	}

	currentID := p.rootFolderID
	parts := strings.Split(name, "/")
	for i, part := range parts {
		folder, err := p.listFolder(currentID)
		if err != nil {
			return pCloudMetadata{}, err
		}
		child, found := findPCloudChild(folder.Contents, part)
		if !found {
			return pCloudMetadata{}, fs.ErrNotExist
		}
		if i == len(parts)-1 {
			return child, nil
		}
		if !child.IsFolder {
			return pCloudMetadata{}, syscall.ENOTDIR
		}
		currentID = child.FolderID
	}
	return pCloudMetadata{}, fs.ErrNotExist
}

func (p *PCloudFS) listFolder(folderID int64) (pCloudMetadata, error) {
	var response struct {
		Metadata pCloudMetadata `json:"metadata"`
	}
	err := p.call("listfolder", url.Values{
		"folderid": {strconv.FormatInt(folderID, 10)},
	}, &response)
	return response.Metadata, err
}

func findPCloudChild(contents []pCloudMetadata, name string) (pCloudMetadata, bool) {
	for i := range contents {
		if contents[i].Name == name && !contents[i].IsDeleted {
			return contents[i], true
		}
	}
	return pCloudMetadata{}, false
}

func validPCloudPath(op, name string, allowRoot bool) error {
	if !fs.ValidPath(name) || (!allowRoot && name == ".") {
		return &fs.PathError{Op: op, Path: name, Err: util.Errorf("%w", fs.ErrInvalid)}
	}
	return nil
}

func (p *PCloudFS) pathError(op, name string, err error) error {
	if err == nil {
		err = fs.ErrInvalid
	}
	var pathErr *fs.PathError
	if errors.As(err, &pathErr) {
		return err
	}
	return &fs.PathError{Op: op, Path: name, Err: util.Errorf("%w", err)}
}
