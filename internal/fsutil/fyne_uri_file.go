package fsutil

import (
	"errors"
	"io"
	"io/fs"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/storage"
	"github.com/voilelab/plainshelf/internal/util"
)

var _ fs.File = (*FyneURIFile)(nil)
var _ fs.FileInfo = (*FyneURIFileInfo)(nil)
var _ fs.DirEntry = (*FyneURIDirEntry)(nil)

type FyneURIFile struct {
	uri fyne.URIReadCloser
}

type FyneURIFileInfo struct {
	name  string
	isDir bool
}

type FyneURIDirEntry struct {
	uri fyne.URI
}

func (f *FyneURIFile) Stat() (fs.FileInfo, error) {
	uri := f.uri.URI()

	isDir, err := storage.CanList(uri)
	if err != nil {
		isDir = false
	}

	info := FyneURIFileInfo{
		name:  uri.Name(),
		isDir: isDir,
	}
	return &info, nil
}

func (f *FyneURIFile) Read(p []byte) (int, error) {
	n, err := f.uri.Read(p)
	if err != nil {
		if errors.Is(err, io.EOF) {
			return n, io.EOF
		}
		return n, util.Errorf("%w", err)
	}
	return n, nil
}

func (f *FyneURIFile) Close() error {
	err := f.uri.Close()
	if err != nil {
		return util.Errorf("%w", err)
	}
	return nil
}

func (fi *FyneURIFileInfo) Name() string {
	return fi.name
}

func (fi *FyneURIFileInfo) Size() int64 {
	// fyne.URI does not provide size information, so we return 0.
	return 0
}

func (fi *FyneURIFileInfo) Mode() fs.FileMode {
	if fi.isDir {
		return fs.ModeDir | 0755
	}
	return 0644
}

func (fi *FyneURIFileInfo) ModTime() time.Time {
	// fyne.URI does not provide modification time information, so we return the zero time.
	return time.Time{}
}

func (fi *FyneURIFileInfo) IsDir() bool {
	return fi.isDir
}

func (fi *FyneURIFileInfo) Sys() any {
	// fyne.URI does not provide system-specific information, so we return nil.
	return nil
}

func (de *FyneURIDirEntry) Name() string {
	return de.uri.Name()
}

func (de *FyneURIDirEntry) IsDir() bool {
	isDir, err := storage.CanList(de.uri)
	if err != nil {
		return false
	}
	return isDir
}

func (de *FyneURIDirEntry) Type() fs.FileMode {
	if de.IsDir() {
		return fs.ModeDir
	}
	return 0
}

func (de *FyneURIDirEntry) Info() (fs.FileInfo, error) {
	return &FyneURIFileInfo{
		name:  de.uri.Name(),
		isDir: de.IsDir(),
	}, nil
}
