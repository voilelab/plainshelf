package appcache

import (
	"io"
	"path"

	"github.com/voilelab/plainshelf/internal/fsutil"
	"github.com/voilelab/plainshelf/internal/util"
)

type Store interface {
	ReadFile(name string) ([]byte, error)

	EnsureWritable() error

	WriteFileAtomic(name string, data []byte) error
}

type FSStore struct {
	root fsutil.ReadFS
	dir  string
}

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
