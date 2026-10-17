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

// OpenNoFollow opens name, a clean slash-separated path, like Open, but fails
// with fs.ErrNotExist when any component is a symlink, one swapped in while
// the path is walked included. os.Root follows a link that stays inside the
// root; this does not. Each component is checked against a directory handle
// already held, and what is opened must be the entry that was checked. A hard
// link is an ordinary entry and is not detected.
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
	for i, part := range parts {
		checked, err := dir.Lstat(part)
		if err != nil {
			return nil, util.Errorf("%w", err)
		}
		if checked.Mode()&fs.ModeSymlink != 0 {
			return nil, notExist
		}

		if i == len(parts)-1 {
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
	return nil, notExist // unreachable: parts is never empty
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
			fp.Close() //nolint:errcheck // best-effort cleanup; write error is returned
			return util.Errorf("%w", err)
		}
		if n == 0 {
			fp.Close() //nolint:errcheck // best-effort cleanup; write error is returned
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
