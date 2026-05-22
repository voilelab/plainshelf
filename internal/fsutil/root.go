package fsutil

import (
	"io"
	"io/fs"
	"os"

	"github.com/voilelab/plainshelf/internal/util"
)

var _ FS = (*RootFS)(nil)

type RootFS struct {
	root *os.Root
}

func NewRootFS(root *os.Root) *RootFS {
	return &RootFS{root: root}
}

func (l *RootFS) Open(name string) (fs.File, error) {
	fp, err := l.root.Open(name)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}
	return fp, nil
}

func (l *RootFS) ReadDir(name string) ([]fs.DirEntry, error) {
	// list directory entries
	f, err := l.root.Open(name)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}
	defer f.Close()

	entries, err := f.ReadDir(-1)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}
	sortDirEntries(entries)
	return entries, nil
}

func (l *RootFS) Stat(name string) (fs.FileInfo, error) {
	info, err := l.root.Stat(name)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}
	return info, nil
}

func (l *RootFS) OpenWriter(name string) (io.WriteCloser, error) {
	fp, err := l.root.OpenFile(name, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}
	return fp, nil
}

func (l *RootFS) WriteFile(name string, data []byte) error {
	fp, err := l.OpenWriter(name)
	if err != nil {
		return util.Errorf("%w", err)
	}

	written := 0
	for written < len(data) {
		n, err := fp.Write(data[written:])
		if err != nil {
			fp.Close()
			return util.Errorf("%w", err)
		}
		if n == 0 {
			fp.Close()
			return util.Errorf("%w", io.ErrShortWrite)
		}
		written += n
	}

	if err := fp.Close(); err != nil {
		return util.Errorf("%w", err)
	}

	return nil
}

func (l *RootFS) Mkdir(name string) error {
	err := l.root.Mkdir(name, 0755)
	if err != nil {
		return util.Errorf("%w", err)
	}
	return nil
}

func (l *RootFS) MkdirAll(pth string) error {
	err := l.root.MkdirAll(pth, 0755)
	if err != nil {
		return util.Errorf("%w", err)
	}
	return nil
}

func (l *RootFS) Rename(oldPath, newPath string) error {
	err := l.root.Rename(oldPath, newPath)
	if err != nil {
		return util.Errorf("%w", err)
	}
	return nil
}

func (l *RootFS) Remove(name string) error {
	err := l.root.Remove(name)
	if err != nil {
		return util.Errorf("%w", err)
	}
	return nil
}

func (l *RootFS) RemoveAll(name string) error {
	err := l.root.RemoveAll(name)
	if err != nil {
		return util.Errorf("%w", err)
	}
	return nil
}
