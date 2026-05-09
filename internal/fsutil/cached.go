package fsutil

import (
	"io"
	"io/fs"
	"path"

	"github.com/voilelab/plainshelf/internal/util"
)

var _ FS = (*CachedFS)(nil)

// CachedFS is an FS implementation that first checks cacheFS for files,
// and if not found, checks fs and populates cacheFS.
type CachedFS struct {
	fs      FS
	cacheFS FS
}

func NewCachedFS(fs FS, cacheFS FS) *CachedFS {
	return &CachedFS{
		fs:      fs,
		cacheFS: cacheFS,
	}
}

func (c *CachedFS) Open(name string) (fs.File, error) {
	if fp, err := c.cacheFS.Open(name); err == nil {
		return fp, nil
	}

	// Cache miss: try to populate cache in the background path, then fall back.
	c.populateCache(name)

	if fp, err := c.cacheFS.Open(name); err == nil {
		return fp, nil
	}

	fp, err := c.fs.Open(name)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}
	return fp, nil
}

func (c *CachedFS) ReadDir(name string) ([]fs.DirEntry, error) {
	if entries, err := c.cacheFS.ReadDir(name); err == nil {
		return entries, nil
	}
	entries, err := c.fs.ReadDir(name)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}
	return entries, nil
}

func (c *CachedFS) Stat(name string) (fs.FileInfo, error) {
	if info, err := c.cacheFS.Stat(name); err == nil {
		return info, nil
	}
	info, err := c.fs.Stat(name)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}
	return info, nil
}

func (c *CachedFS) OpenWriter(name string) (io.WriteCloser, error) {
	mainWriter, err := c.fs.OpenWriter(name)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	_ = c.cacheFS.MkdirAll(path.Dir(name))
	cacheWriter, err := c.cacheFS.OpenWriter(name)
	if err != nil {
		return mainWriter, nil
	}

	return &cachedWriteCloser{
		main:  mainWriter,
		cache: cacheWriter,
	}, nil
}

func (c *CachedFS) Mkdir(name string) error {
	if err := c.fs.Mkdir(name); err != nil {
		return util.Errorf("%w", err)
	}
	_ = c.cacheFS.Mkdir(name)
	return nil
}

func (c *CachedFS) MkdirAll(pth string) error {
	if err := c.fs.MkdirAll(pth); err != nil {
		return util.Errorf("%w", err)
	}
	_ = c.cacheFS.MkdirAll(pth)
	return nil
}

func (c *CachedFS) Rename(oldPath, newPath string) error {
	if err := c.fs.Rename(oldPath, newPath); err != nil {
		return util.Errorf("%w", err)
	}
	_ = c.cacheFS.Rename(oldPath, newPath)
	return nil
}

func (c *CachedFS) Remove(name string) error {
	if err := c.fs.Remove(name); err != nil {
		return util.Errorf("%w", err)
	}
	_ = c.cacheFS.Remove(name)
	return nil
}

func (c *CachedFS) RemoveAll(name string) error {
	if err := c.fs.RemoveAll(name); err != nil {
		return util.Errorf("%w", err)
	}
	_ = c.cacheFS.RemoveAll(name)
	return nil
}

func (c *CachedFS) populateCache(name string) {
	fp, err := c.fs.Open(name)
	if err != nil {
		return
	}
	defer fp.Close()

	_ = c.cacheFS.MkdirAll(path.Dir(name))

	w, err := c.cacheFS.OpenWriter(name)
	if err != nil {
		return
	}
	defer w.Close()

	_, _ = io.Copy(w, fp)
}

type cachedWriteCloser struct {
	main  io.WriteCloser
	cache io.WriteCloser
}

func (w *cachedWriteCloser) Write(p []byte) (int, error) {
	n, err := w.main.Write(p)
	if w.cache != nil && n > 0 {
		if _, cacheErr := w.cache.Write(p[:n]); cacheErr != nil {
			_ = w.cache.Close()
			w.cache = nil
		}
	}
	return n, err
}

func (w *cachedWriteCloser) Close() error {
	mainErr := w.main.Close()
	if w.cache != nil {
		_ = w.cache.Close()
	}
	return mainErr
}
