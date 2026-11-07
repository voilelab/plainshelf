package readingprogress

import (
	"github.com/voilelab/plainshelf/internal/lockedfile"
)

type Store struct {
	file *lockedfile.File
}

func NewStore(path string) *Store {
	return &Store{file: lockedfile.New(path, "reading progress", ".reading_progress-*.json")}
}

func (s *Store) Read() (Document, string, error) {
	raw, err := s.file.Read()
	if err != nil {
		return New(), "", err
	}
	return Parse(raw), raw, nil
}

func (s *Store) Mutate(fn func(Document) Document) (Document, error) {
	next, err := lockedfile.Mutate(s.file, Parse, Serialize, fn)
	if err != nil {
		return New(), err
	}
	return next, nil
}
