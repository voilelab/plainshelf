// Package appcache is the filesystem contract shared by the rebuildable caches
// that live under a shelf's app/ directory. It gives a cache "read a file,
// confirm it can be written, replace it atomically" without the cache having to
// know anything about the shelf that owns the directory - not its read handle,
// not its write handle, not its readiness.
//
// It depends only on internal/fsutil and internal/util, so a cache under app/
// can reuse it without reaching back into a sibling package for file access or
// re-deriving the read-only check.
package appcache

import (
	"io"
	"path"

	"github.com/voilelab/plainshelf/internal/fsutil"
	"github.com/voilelab/plainshelf/internal/util"
)

// Store is a cache's whole contact with a filesystem: it reads and writes the
// cache's file under the shelf's app/ directory and nothing else. Given one of
// these, the cache needs no *Shelf - not its read handle, not its write handle,
// not its readiness.
//
// The cache owns the file's name and passes it on every call; the Store owns the
// directory it lives in and how a write is made atomic and refused when the
// backing filesystem is read-only.
type Store interface {
	// ReadFile returns the named file's bytes. Every error - absent, unreadable,
	// permission - is a cache miss the cache starts empty from, never fatal, so
	// a Store need not distinguish them.
	ReadFile(name string) ([]byte, error)

	// EnsureWritable reports whether a write could happen without performing one,
	// so a save on a read-only shelf is refused before it does any work rather
	// than only when it finally reaches WriteFileAtomic - which a save with
	// nothing new to write never does.
	EnsureWritable() error

	// WriteFileAtomic replaces the named file in one step, so a sync client
	// copying the shelf away never picks up half a cache. It reports
	// fsutil.ErrReadOnly on a read-only backing filesystem.
	WriteFileAtomic(name string, data []byte) error
}

// FSStore is a Store backed by a directory on an fsutil filesystem - the shelf's
// app/ directory in normal use, but equally a directory on any read handle,
// which is what lets a read-only reader (a backup image, or the Android pcloud
// client) open the same cache.
//
// It reads through the read handle and narrows it to a writable one only when
// asked to write, so a read-only filesystem is refused by EnsureWritable and
// WriteFileAtomic exactly where the shelf's own writes are refused.
type FSStore struct {
	root fsutil.ReadFS
	dir  string
}

// NewFSStore returns a Store that keeps its files under dir on root.
func NewFSStore(root fsutil.ReadFS, dir string) *FSStore {
	return &FSStore{root: root, dir: dir}
}

func (s *FSStore) ReadFile(name string) ([]byte, error) {
	file, err := s.root.Open(path.Join(s.dir, name))
	if err != nil {
		return nil, util.Errorf("%w", err)
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}
	return data, nil
}

func (s *FSStore) EnsureWritable() error {
	if _, err := fsutil.Writable(s.root); err != nil {
		return util.Errorf("%w", err)
	}
	return nil
}

func (s *FSStore) WriteFileAtomic(name string, data []byte) error {
	root, err := fsutil.Writable(s.root)
	if err != nil {
		return util.Errorf("%w", err)
	}
	if err := fsutil.WriteFileAtomic(root, path.Join(s.dir, name), data); err != nil {
		return util.Errorf("%w", err)
	}
	return nil
}
