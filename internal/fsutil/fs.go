package fsutil

import (
	"errors"
	"io"
	"io/fs"

	"github.com/voilelab/plainshelf/internal/util"
)

var ErrReadOnly = util.NewError("filesystem is read-only")

var ErrSourceUnavailable = util.NewError("shelf source is unavailable")

var ErrSourceUnauthorized = util.NewError("shelf source refused the credential")

type ReadFS interface {
	Open(name string) (fs.File, error)

	ReadDir(name string) ([]fs.DirEntry, error)

	Stat(name string) (fs.FileInfo, error)
}

type ZipOpener interface {
	OpenZip(names ...string) (io.ReadCloser, error)
}

type FS interface {
	ReadFS

	OpenWriter(name string) (io.WriteCloser, error)

	WriteFile(name string, data []byte) error

	Mkdir(name string) error

	MkdirAll(path string) error

	Rename(oldPath, newPath string) error

	Remove(name string) error

	RemoveAll(name string) error
}

func Writable(rfs ReadFS) (FS, error) {
	fsys, ok := rfs.(FS)
	if !ok {
		return nil, util.Errorf("%w", ErrReadOnly)
	}
	return fsys, nil
}

func ReadOnly(rfs ReadFS) ReadFS {
	if z, ok := rfs.(ZipOpener); ok {
		return readOnlyZipFS{readOnlyFS{rfs}, z}
	}
	return readOnlyFS{rfs}
}

type readOnlyFS struct {
	ReadFS
}

type readOnlyZipFS struct {
	readOnlyFS
	ZipOpener
}

type NoFollowFS interface {
	OpenNoFollow(name string) (fs.File, error)
}

func (r readOnlyFS) OpenNoFollow(name string) (fs.File, error) {
	n, ok := r.ReadFS.(NoFollowFS)
	if !ok {
		return nil, util.Errorf("open %q: %w", name, errors.ErrUnsupported)
	}
	return n.OpenNoFollow(name)
}
