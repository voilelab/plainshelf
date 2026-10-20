package readingprogress

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/gofrs/flock"
	"github.com/voilelab/plainshelf/internal/fsutil"
	"github.com/voilelab/plainshelf/internal/util"
)

const maxDocumentBytes = 1 << 20

const (
	lockTimeout    = 10 * time.Second
	lockRetryDelay = 50 * time.Millisecond
)

type Store struct {
	path     string
	lockPath string
}

// NewStore returns a Store for the reading-progress document at path. The lock
// is taken on a sibling ".lock" file rather than the document itself: the
// document is replaced by rename on every write, which would drop a lock held on
// the old inode.
func NewStore(path string) *Store {
	return &Store{path: path, lockPath: path + ".lock"}
}

func (s *Store) Read() (Document, string, error) {
	raw, err := readFile(s.path)
	if err != nil {
		return New(), "", err
	}
	return Parse(raw), raw, nil
}

func (s *Store) Mutate(fn func(Document) Document) (Document, error) {
	if s.path == "" {
		return New(), util.NewError("reading progress storage is not ready")
	}

	unlock, err := s.lock()
	if err != nil {
		return New(), err
	}
	defer unlock()

	raw, err := readFile(s.path)
	if err != nil {
		return New(), err
	}
	current := Parse(raw)

	next := fn(current)

	currentText, err := Serialize(current)
	if err != nil {
		return New(), err
	}
	nextText, err := Serialize(next)
	if err != nil {
		return New(), err
	}
	if nextText == currentText {
		return next, nil
	}
	if len(nextText) > maxDocumentBytes {
		return New(), util.Errorf("reading progress document is too large: %d bytes", len(nextText))
	}

	if err := writeAtomic(s.path, nextText); err != nil {
		return New(), err
	}
	return next, nil
}

func (s *Store) lock() (func(), error) {
	if err := os.MkdirAll(filepath.Dir(s.lockPath), 0o755); err != nil {
		return nil, util.Errorf("%w", err)
	}

	lk := flock.New(s.lockPath)
	ctx, cancel := context.WithTimeout(context.Background(), lockTimeout)
	defer cancel()

	locked, err := lk.TryLockContext(ctx, lockRetryDelay)
	if err != nil {
		return nil, util.Errorf("locking reading progress: %w", err)
	}
	if !locked {
		return nil, util.NewError("timed out locking reading progress")
	}
	return func() {
		if unlockErr := lk.Unlock(); unlockErr != nil {
			_ = unlockErr
		}
	}, nil
}

func readFile(path string) (string, error) {
	if path == "" {
		return "", util.NewError("reading progress storage is not ready")
	}
	return fsutil.ReadTextFile(path)
}

func writeAtomic(path, text string) error {
	return fsutil.WriteTextFileAtomic(path, ".reading_progress-*.json", text)
}
