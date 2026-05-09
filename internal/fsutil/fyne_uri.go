package fsutil

import (
	"io"
	"io/fs"
	"path"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/storage"
	"github.com/voilelab/plainshelf/internal/util"
)

var _ FS = (*FyneURIFS)(nil)

type FyneURIFS struct {
	root fyne.ListableURI
}

func NewFyneURIFS(root fyne.ListableURI) *FyneURIFS {
	return &FyneURIFS{root: root}
}

func (l *FyneURIFS) resolve(name string) (fyne.URI, error) {
	cleanName := path.Clean(name)
	if cleanName == "." || cleanName == "/" {
		return l.root, nil
	}

	parts := strings.Split(strings.TrimPrefix(cleanName, "/"), "/")
	var curURI fyne.URI = l.root

	for _, part := range parts {
		child, err := storage.Child(curURI, part)
		if err != nil {
			return nil, util.Errorf("%w", err)
		}
		curURI = child
	}

	return curURI, nil
}

func (l *FyneURIFS) Open(name string) (fs.File, error) {
	uri, err := l.resolve(name)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	fp, err := storage.Reader(uri)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	return &FyneURIFile{uri: fp}, nil
}

func (l *FyneURIFS) ReadDir(name string) ([]fs.DirEntry, error) {
	uri, err := l.resolve(name)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	entries, err := storage.List(uri)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	dirEntries := make([]fs.DirEntry, len(entries))
	for i, entry := range entries {
		dirEntries[i] = &FyneURIDirEntry{uri: entry}
	}
	return dirEntries, nil
}

func (l *FyneURIFS) Stat(name string) (fs.FileInfo, error) {
	uri, err := l.resolve(name)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	isDir, err := storage.CanList(uri)
	if err != nil {
		isDir = false
	}

	return &FyneURIFileInfo{
		name:  uri.Name(),
		isDir: isDir,
	}, nil
}

func (l *FyneURIFS) OpenWriter(name string) (io.WriteCloser, error) {
	uri, err := l.resolve(name)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	fp, err := storage.Writer(uri)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	return fp, nil
}

func (l *FyneURIFS) Mkdir(name string) error {
	uri, err := l.resolve(name)
	if err != nil {
		return util.Errorf("%w", err)
	}

	if err := storage.CreateListable(uri); err != nil {
		return util.Errorf("%w", err)
	}
	return nil
}

func (l *FyneURIFS) MkdirAll(pth string) error {
	cleanPath := path.Clean(pth)
	if cleanPath == "." || cleanPath == "/" {
		return nil
	}

	parts := strings.Split(strings.TrimPrefix(cleanPath, "/"), "/")
	var curURI fyne.URI = l.root

	for _, part := range parts {
		child, err := storage.Child(curURI, part)
		if err != nil {
			return util.Errorf("%w", err)
		}

		if _, err := storage.List(child); err != nil {
			if err := storage.CreateListable(child); err != nil {
				// Handle providers that return an error for existing directories.
				if _, listErr := storage.List(child); listErr != nil {
					return util.Errorf("%w", err)
				}
			}
		}

		curURI = child
	}

	return nil
}

func (l *FyneURIFS) Rename(oldPath, newPath string) error {
	oldURI, err := l.resolve(oldPath)
	if err != nil {
		return util.Errorf("%w", err)
	}

	newURI, err := l.resolve(newPath)
	if err != nil {
		return util.Errorf("%w", err)
	}

	if err := storage.Move(oldURI, newURI); err != nil {
		return util.Errorf("%w", err)
	}
	return nil
}

func (l *FyneURIFS) Remove(name string) error {
	uri, err := l.resolve(name)
	if err != nil {
		return util.Errorf("%w", err)
	}

	if err := storage.Delete(uri); err != nil {
		return util.Errorf("%w", err)
	}
	return nil
}

func (l *FyneURIFS) RemoveAll(name string) error {
	uri, err := l.resolve(name)
	if err != nil {
		return util.Errorf("%w", err)
	}

	if err := storage.Delete(uri); err != nil {
		return util.Errorf("%w", err)
	}
	return nil
}
