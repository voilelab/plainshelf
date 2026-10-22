package fsutil

import (
	"io"
	"io/fs"
	"os"
	"slices"
	"strings"

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
	f, err := l.root.Open(name)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}
	defer f.Close()

	entries, err := f.ReadDir(-1)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}
	slices.SortFunc(entries, func(a, b fs.DirEntry) int { return strings.Compare(a.Name(), b.Name()) })
	return entries, nil
}

func (l *RootFS) Stat(name string) (fs.FileInfo, error) {
	info, err := l.root.Stat(name)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}
	return info, nil
}

func (l *RootFS) OpenNoFollow(name string) (fs.File, error) {
	if name == "." {
		return l.Open(name)
	}
	notExist := util.Errorf("%q: %w", name, fs.ErrNotExist)

	dir := l.root
	defer func() {
		if dir != l.root {
			dir.Close()
		}
	}()
	parts := strings.Split(name, "/")
	for _, part := range parts[:len(parts)-1] {
		checked, err := dir.Lstat(part)
		if err != nil {
			return nil, util.Errorf("%w", err)
		}
		if checked.Mode()&fs.ModeSymlink != 0 {
			return nil, notExist
		}

		if !checked.IsDir() {
			return nil, notExist
		}

		sub, err := dir.OpenRoot(part)
		if err != nil {
			return nil, util.Errorf("%w", err)
		}
		if dir != l.root {
			dir.Close()
		}
		dir = sub
		opened, err := dir.Stat(".")
		if err != nil || !os.SameFile(checked, opened) {
			return nil, notExist
		}
	}
	part := parts[len(parts)-1]
	checked, err := dir.Lstat(part)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}
	if checked.Mode()&fs.ModeSymlink != 0 {
		return nil, notExist
	}
	fp, err := dir.Open(part)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}
	opened, err := fp.Stat()
	if err != nil || !os.SameFile(checked, opened) {
		fp.Close()
		return nil, notExist
	}
	return fp, nil
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
