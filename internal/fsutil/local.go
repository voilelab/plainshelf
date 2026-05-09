package fsutil

import (
	"io"
	"io/fs"
	"os"
	"path"

	"github.com/voilelab/plainshelf/internal/util"
)

var _ FS = (*LocalFS)(nil)

type LocalFS struct {
	root string
}

func NewLocalFS(root string) *LocalFS {
	return &LocalFS{root: root}
}

func (l *LocalFS) Open(name string) (fs.File, error) {
	fp, err := os.Open(path.Join(l.root, name))
	if err != nil {
		return nil, util.Errorf("%w", err)
	}
	return fp, nil
}

func (l *LocalFS) ReadDir(name string) ([]fs.DirEntry, error) {
	entries, err := os.ReadDir(path.Join(l.root, name))
	if err != nil {
		return nil, util.Errorf("%w", err)
	}
	return entries, nil
}

func (l *LocalFS) Stat(name string) (fs.FileInfo, error) {
	info, err := os.Stat(path.Join(l.root, name))
	if err != nil {
		return nil, util.Errorf("%w", err)
	}
	return info, nil
}

func (l *LocalFS) OpenWriter(name string) (io.WriteCloser, error) {
	fp, err := os.OpenFile(path.Join(l.root, name), os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}
	return fp, nil
}

func (l *LocalFS) Mkdir(name string) error {
	if err := os.Mkdir(path.Join(l.root, name), 0755); err != nil {
		return util.Errorf("%w", err)
	}
	return nil
}

func (l *LocalFS) MkdirAll(pth string) error {
	if err := os.MkdirAll(path.Join(l.root, pth), 0755); err != nil {
		return util.Errorf("%w", err)
	}
	return nil
}

func (l *LocalFS) Rename(oldPath, newPath string) error {
	if err := os.Rename(path.Join(l.root, oldPath), path.Join(l.root, newPath)); err != nil {
		return util.Errorf("%w", err)
	}
	return nil
}

func (l *LocalFS) Remove(name string) error {
	if err := os.Remove(path.Join(l.root, name)); err != nil {
		return util.Errorf("%w", err)
	}
	return nil
}

func (l *LocalFS) RemoveAll(name string) error {
	if err := os.RemoveAll(path.Join(l.root, name)); err != nil {
		return util.Errorf("%w", err)
	}
	return nil
}
